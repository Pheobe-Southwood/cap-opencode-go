package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"

	"github.com/cpa-opencodego/cap-opencode-go/internal/abi"
	"github.com/cpa-opencodego/cap-opencode-go/internal/dashboard"
	"github.com/cpa-opencodego/cap-opencode-go/internal/keys"
	"github.com/cpa-opencodego/cap-opencode-go/internal/quota"
	"github.com/cpa-opencodego/cap-opencode-go/internal/roles"
)

// Resource paths served by the main build. Only /dashboard carries a Menu, so
// the WebUI shows a single flat sidebar entry next to the other plugins.
const (
	resourceDashboard = "/dashboard"
	resourceState     = "/api/state"
	resourceQuota     = "/api/quota"
	resourceModels    = "/api/models"
)

type managementRegistrationResponse struct {
	Routes    []pluginapi.ManagementRoute `json:"routes,omitempty"`
	Resources []pluginapi.ResourceRoute   `json:"resources,omitempty"`
}

func (r *Runtime) handleManagementRegister(request []byte) []byte {
	if r.role.Name != roles.Chat {
		return okJSON(managementRegistrationResponse{})
	}

	var registerRequest pluginapi.ManagementRegistrationRequest
	if len(request) > 0 {
		if errUnmarshal := json.Unmarshal(request, &registerRequest); errUnmarshal == nil {
			base := strings.TrimSpace(registerRequest.ResourceBasePath)
			if index := strings.LastIndex(base, "/"); index >= 0 && index+1 < len(base) {
				r.mu.Lock()
				r.pluginID = base[index+1:]
				r.mu.Unlock()
			}
		}
	}

	response := managementRegistrationResponse{
		Routes: []pluginapi.ManagementRoute{probeRoute()},
		Resources: []pluginapi.ResourceRoute{
			{
				Path:        resourceDashboard,
				Menu:        menuLabel,
				Description: "OpenCode Go 渠道设置、Key 池与额度",
			},
			{Path: resourceState, Description: "OpenCode Go 运行状态（掩码后的配置与模型开关）"},
			{Path: resourceQuota, Description: "OpenCode Go 各 Key 的额度余量"},
			{Path: resourceModels, Description: "OpenCode Go 上游模型列表与本插件模型配置的合并视图"},
		},
	}
	return okJSON(response)
}

// PluginID reports the plugin id the host assigned, learned from the resource
// base path during management.register.
func (r *Runtime) PluginID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pluginID != "" {
		return r.pluginID
	}
	return roles.MainPluginID
}

func (r *Runtime) handleManagementHandle(request []byte) []byte {
	var managementRequest pluginapi.ManagementRequest
	if len(request) > 0 {
		if errUnmarshal := json.Unmarshal(request, &managementRequest); errUnmarshal != nil {
			return abi.Fail("invalid_request", "解析管理请求失败: "+errUnmarshal.Error(), http.StatusBadRequest)
		}
	}
	path := strings.TrimSpace(managementRequest.Path)
	switch {
	case strings.HasSuffix(path, managementProbePath):
		payload, errProbe := r.probeModel(context.Background(), managementRequest.Query.Get("model"))
		if errProbe != nil {
			return abi.FailFor(errProbe, "probe_failed")
		}
		return okJSON(pluginapi.ManagementResponse{
			StatusCode: http.StatusOK,
			Headers:    jsonHeaders(),
			Body:       mustJSON(payload),
		})
	case strings.HasSuffix(path, resourceDashboard):
		return okJSON(pluginapi.ManagementResponse{
			StatusCode: http.StatusOK,
			Headers:    htmlHeaders(),
			Body:       dashboard.Page(r.PluginID(), string(r.role.Name)),
		})
	case strings.HasSuffix(path, resourceState):
		return okJSON(pluginapi.ManagementResponse{
			StatusCode: http.StatusOK,
			Headers:    jsonHeaders(),
			Body:       mustJSON(r.StatePayload()),
		})
	case strings.HasSuffix(path, resourceModels):
		r.refreshShared()
		force := strings.Contains(managementRequest.Query.Get("refresh"), "1")
		ids, fetchedAt, upstreamErr := r.client.ListModels(context.Background(), force)
		summary := UpstreamSummary{Count: len(ids), FetchedAt: fetchedAt, Error: upstreamErr}
		payload := r.stateWithRows(r.modelRows(ids), &summary)
		payload.Upstream = &summary
		return okJSON(pluginapi.ManagementResponse{
			StatusCode: http.StatusOK,
			Headers:    jsonHeaders(),
			Body:       mustJSON(payload),
		})
	case strings.HasSuffix(path, resourceQuota):
		r.refreshShared()
		settings := r.Settings()
		snapshots := quota.Fetch(context.Background(), settings, r.client.Pool())
		return okJSON(pluginapi.ManagementResponse{
			StatusCode: http.StatusOK,
			Headers:    jsonHeaders(),
			Body:       mustJSON(map[string]any{"keys": snapshots}),
		})
	default:
		return okJSON(pluginapi.ManagementResponse{
			StatusCode: http.StatusNotFound,
			Headers:    jsonHeaders(),
			Body:       []byte(`{"error":"not_found"}`),
		})
	}
}

// ModelRow describes one model id for the dashboard.
type ModelRow struct {
	// ID is the id clients call, which may be an alias.
	ID string `json:"id"`
	// Upstream is the id sent to OpenCode Go.
	Upstream string `json:"upstream"`
	// DisplayName is the user-facing label.
	DisplayName string `json:"display_name"`
	// Family is the upstream protocol family.
	Family string `json:"family"`
	// FamilySource is where Family came from: config, catalogue, or inferred.
	FamilySource string `json:"family_source,omitempty"`
	// Enabled reports whether this provider serves the id.
	Enabled bool `json:"enabled"`
	// Proxy is the egress policy for the id.
	Proxy string `json:"proxy"`
	// Aliases lists the aliases configured on the base model.
	Aliases []string `json:"aliases"`
	// Served reports whether this provider build (role) serves the id.
	Served bool `json:"served"`
	// Catalogued reports whether the base model is in the built-in catalogue.
	Catalogued bool `json:"catalogued"`
	// IsAlias reports whether the row is an alias of another model.
	IsAlias bool `json:"is_alias"`
	// BaseID is the model an alias points at.
	BaseID string `json:"base_id,omitempty"`
	// Discovered reports an id the upstream publishes but the plugin does not
	// serve yet; the operator opts in by assigning a family.
	Discovered bool `json:"discovered,omitempty"`
	// UpstreamAvailable reports whether the upstream still publishes the model.
	UpstreamAvailable bool `json:"upstream_available"`
	// AliasShadowed reports an alias that could not be used because the id is
	// already taken.
	AliasShadowed bool   `json:"alias_shadowed,omitempty"`
	Note          string `json:"note,omitempty"`
}

// StatePayload is the dashboard's read model. Secrets are never included.
type StatePayload struct {
	Role          string            `json:"role"`
	Provider      string            `json:"provider"`
	Version       string            `json:"version"`
	PluginID      string            `json:"plugin_id"`
	BaseURL       string            `json:"base_url"`
	SessionMode   string            `json:"session_mode"`
	Session       string            `json:"session"`
	DefaultProxy  string            `json:"default_proxy"`
	ProxyURL      string            `json:"proxy_url"`
	HostProxyURL  string            `json:"host_proxy_url"`
	StatePath     string            `json:"state_path"`
	ModelsDefault string            `json:"models_default_proxy"`
	Keys          []keys.State      `json:"keys"`
	Models        []ModelRow        `json:"models"`
	Siblings      []SiblingRow      `json:"siblings"`
	ConfigError   string            `json:"config_error,omitempty"`
	StartedAt     time.Time         `json:"started_at"`
	Extra         map[string]string `json:"extra_headers,omitempty"`
	Upstream      *UpstreamSummary  `json:"upstream,omitempty"`
}

// SiblingRow reports the enable state of the other provider builds, which the
// dashboard can toggle through the host configuration API.
type SiblingRow struct {
	PluginID string `json:"plugin_id"`
	Role     string `json:"role"`
	Provider string `json:"provider"`
}

// StatePayload builds the dashboard read model without contacting the upstream.
func (r *Runtime) StatePayload() StatePayload {
	return r.stateWithRows(r.modelRows(nil), nil)
}

func (r *Runtime) stateWithRows(rows []ModelRow, upstream *UpstreamSummary) StatePayload {
	settings := r.Settings()
	siblings := make([]SiblingRow, 0, len(roles.All))
	for _, role := range roles.All {
		siblings = append(siblings, SiblingRow{
			PluginID: role.PluginSuffix,
			Role:     string(role.Name),
			Provider: role.Provider,
		})
	}
	_ = upstream
	return StatePayload{
		Role:          string(r.role.Name),
		Provider:      r.role.Provider,
		Version:       r.Version(),
		PluginID:      r.PluginID(),
		BaseURL:       settings.BaseURL,
		SessionMode:   settings.SessionMode,
		Session:       settings.Session,
		DefaultProxy:  settings.DefaultProxy,
		ProxyURL:      settings.ProxyURL,
		HostProxyURL:  settings.HostProxyURL,
		StatePath:     r.sharedPath,
		ModelsDefault: settings.ModelConfig("__default__").Proxy,
		Keys:          r.client.Pool().Snapshot(),
		Models:        rows,
		Siblings:      siblings,
		ConfigError:   r.LastError(),
		StartedAt:     r.startedAt,
		Extra:         settings.ExtraHeaders,
	}
}

// UpstreamSummary reports the state of the live model list.
type UpstreamSummary struct {
	Count     int       `json:"count"`
	FetchedAt time.Time `json:"fetched_at,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// modelRows merges the configured models with the live upstream list. When
// upstreamIDs is nil the upstream is not consulted and availability is reported
// optimistically.
func (r *Runtime) modelRows(upstreamIDs []string) []ModelRow {
	resolver := r.Resolver()
	settings := r.Settings()

	known := make(map[string]struct{}, len(upstreamIDs))
	for _, id := range upstreamIDs {
		known[id] = struct{}{}
	}
	consulted := len(upstreamIDs) > 0
	shadowed := resolver.Shadowed()

	rows := make([]ModelRow, 0, len(resolver.Entries())+len(upstreamIDs))
	seen := make(map[string]struct{}, len(resolver.Entries()))
	for _, entry := range resolver.Entries() {
		seen[entry.ID] = struct{}{}
		baseID := entry.ID
		if entry.Alias {
			baseID = entry.Upstream
		}
		row := ModelRow{
			ID:                entry.ID,
			Upstream:          entry.Upstream,
			DisplayName:       entry.DisplayName,
			Family:            string(entry.Family),
			FamilySource:      string(entry.FamilySource),
			Enabled:           entry.Enabled,
			Proxy:             settings.ModelConfig(baseID).Proxy,
			Aliases:           settings.ModelConfig(baseID).Aliases,
			Served:            entry.Enabled && entry.Family == r.role.Name,
			Catalogued:        entry.Catalogued,
			IsAlias:           entry.Alias,
			UpstreamAvailable: !consulted || containsString(upstreamIDs, entry.Upstream),
			Note:              entry.Note,
			AliasShadowed:     shadowed[entry.ID] != "",
		}
		if entry.Alias {
			row.BaseID = entry.Upstream
		}
		rows = append(rows, row)
	}

	// Ids the upstream publishes that nobody serves yet.
	for _, id := range upstreamIDs {
		if _, exists := seen[id]; exists {
			continue
		}
		family, source := settings.FamilyWithSource(id)
		rows = append(rows, ModelRow{
			ID:                id,
			Upstream:          id,
			DisplayName:       id,
			Family:            string(family),
			FamilySource:      string(source),
			Enabled:           false,
			Proxy:             settings.ModelConfig(id).Proxy,
			Discovered:        true,
			UpstreamAvailable: true,
		})
	}
	return rows
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func htmlHeaders() http.Header {
	return http.Header{
		"Content-Type":            []string{"text/html; charset=utf-8"},
		"Cache-Control":           []string{"no-store"},
		"Referrer-Policy":         []string{"no-referrer"},
		"X-Content-Type-Options":  []string{"nosniff"},
		"Content-Security-Policy": []string{"default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'; img-src data: blob:; base-uri 'none'; form-action 'none'; frame-ancestors 'self'"},
	}
}

func jsonHeaders() http.Header {
	return http.Header{
		"Content-Type":           []string{"application/json; charset=utf-8"},
		"Cache-Control":          []string{"no-store"},
		"X-Content-Type-Options": []string{"nosniff"},
	}
}

func mustJSON(v any) []byte {
	raw, errMarshal := json.Marshal(v)
	if errMarshal != nil {
		return []byte(`{"error":"marshal_error"}`)
	}
	return raw
}
