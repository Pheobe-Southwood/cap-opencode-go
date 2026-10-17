package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"

	"github.com/cpa-opencodego/cap-opencode-go/internal/abi"
	"github.com/cpa-opencodego/cap-opencode-go/internal/dashboard"
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
	resourceStats     = "/api/stats"
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
			{Path: resourceStats, Description: "本插件用过的模型的 token、刊例价、月额度、平均 TPS 与 TTFT"},
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
		// Opening the dashboard is also the self-healing hook for the account
		// projection: if a sync failed earlier it is retried here.
		r.syncAccountsAsync()
		return okJSON(pluginapi.ManagementResponse{
			StatusCode: http.StatusOK,
			Headers:    jsonHeaders(),
			Body:       mustJSON(r.StatePayload()),
		})
	case strings.HasSuffix(path, resourceModels):
		r.refreshShared()
		force := strings.Contains(managementRequest.Query.Get("refresh"), "1")
		ids, fetchedAt, upstreamErr := r.client.ListModels(context.Background(), force)
		if upstreamErr == "" {
			r.recordModelListCheck(ids, fetchedAt)
		}
		summary := UpstreamSummary{Count: len(ids), FetchedAt: fetchedAt, Error: upstreamErr}
		payload := r.stateWithRows(r.modelRows(ids), &summary)
		payload.Upstream = &summary
		return okJSON(pluginapi.ManagementResponse{
			StatusCode: http.StatusOK,
			Headers:    jsonHeaders(),
			Body:       mustJSON(payload),
		})
	case strings.HasSuffix(path, resourceStats):
		r.refreshShared()
		return okJSON(pluginapi.ManagementResponse{
			StatusCode: http.StatusOK,
			Headers:    jsonHeaders(),
			Body:       mustJSON(r.statsPayload(managementRequest.Query)),
		})
	case strings.HasSuffix(path, resourceQuota):
		r.refreshShared()
		settings := r.Settings()
		snapshots := quota.Fetch(context.Background(), settings)
		return okJSON(pluginapi.ManagementResponse{
			StatusCode: http.StatusOK,
			Headers:    jsonHeaders(),
			Body: mustJSON(map[string]any{
				"keys": snapshots,
				// The dashboard shows which host auth file represents each key,
				// which is what the operator sees in CPA's 配额管理 page.
				"accounts": r.accountsState(),
			}),
		})
	default:
		return okJSON(pluginapi.ManagementResponse{
			StatusCode: http.StatusNotFound,
			Headers:    jsonHeaders(),
			Body:       []byte(`{"error":"not_found"}`),
		})
	}
}

// ModelRow describes one model for the dashboard. It is keyed by the configured
// model id, because that is what the operator edits; the ids clients may call
// are reported separately.
type ModelRow struct {
	// ID is the configured model id, i.e. the key under plugin config models.
	ID string `json:"id"`
	// ClientIDs lists every id clients can call for this model: the aliases that
	// replaced it, or ID itself when no alias is in effect.
	ClientIDs []string `json:"client_ids"`
	// Aliased reports whether an alias replaced ID as the client-facing name.
	Aliased bool `json:"aliased"`
	// Upstream is the id sent to OpenCode Go. It always equals ID: an alias
	// renames the client-facing id, never the upstream one.
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
	// Aliases lists the aliases configured on the model, including any that were
	// rejected.
	Aliases []string `json:"aliases"`
	// Served reports whether this provider build (role) serves the id.
	Served bool `json:"served"`
	// Catalogued reports whether the model is in the built-in catalogue.
	Catalogued bool `json:"catalogued"`
	// Discovered reports an id the upstream publishes but the plugin does not
	// serve yet; the operator opts in by assigning a family.
	Discovered bool `json:"discovered,omitempty"`
	// UpstreamAvailable reports whether the upstream still publishes the model.
	UpstreamAvailable bool `json:"upstream_available"`
	// AliasShadowed reports that at least one configured alias was rejected
	// because its id belongs to another model.
	AliasShadowed bool `json:"alias_shadowed,omitempty"`
	// ShadowedAliases lists the rejected aliases, so the operator can fix them.
	ShadowedAliases []string `json:"shadowed_aliases,omitempty"`
	Note            string   `json:"note,omitempty"`
}

// KeyState describes one configured key for the dashboard.
type KeyState struct {
	Name     string `json:"name"`
	Masked   string `json:"masked"`
	Priority int    `json:"priority"`
	Enabled  bool   `json:"enabled"`
}

// StatePayload is the dashboard's read model. Secrets are never included: the
// dashboard edits the plugin configuration without ever reading a key back, and
// a save merges the edits into the host configuration it reads at write time.
type StatePayload struct {
	Role           string            `json:"role"`
	Provider       string            `json:"provider"`
	Version        string            `json:"version"`
	PluginID       string            `json:"plugin_id"`
	BaseURL        string            `json:"base_url"`
	SessionMode    string            `json:"session_mode"`
	Session        string            `json:"session"`
	DefaultProxy   string            `json:"default_proxy"`
	ProxyURL       string            `json:"proxy_url"`
	HostProxyURL   string            `json:"host_proxy_url"`
	StatePath      string            `json:"state_path"`
	StateDir       string            `json:"state_dir"`
	ModelsDefault  string            `json:"models_default_proxy"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	Plan           string            `json:"plan"`
	Debug          bool              `json:"debug"`
	Keys           []KeyState        `json:"keys"`
	Models         []ModelRow        `json:"models"`
	Siblings       []SiblingRow      `json:"siblings"`
	ConfigError    string            `json:"config_error,omitempty"`
	StartedAt      time.Time         `json:"started_at"`
	Extra          map[string]string `json:"extra_headers,omitempty"`
	Upstream       *UpstreamSummary  `json:"upstream,omitempty"`
	// Accounts describes the auth files projected into the host auth directory:
	// one per configured key, so CPA's 配额管理 → Plugin lists them.
	Accounts AccountsPayload `json:"accounts"`
}

// SiblingRow describes one protocol face of the provider.
//
// The provider ships as four shared libraries, one per upstream protocol
// family, because the host translates client protocols into the formats a
// library declares. The host's plugin list therefore has four rows for one
// product; the dashboard renders this list as the four faces of that product,
// names what each face serves and states what stops working when it is missing.
type SiblingRow struct {
	PluginID string `json:"plugin_id"`
	Role     string `json:"role"`
	Provider string `json:"provider"`
	// Label is the name the management centre shows for this face: the product
	// name on the main face, the product name plus the protocol on siblings.
	Label string `json:"label"`
	// Protocol names the wire protocol in user-facing terms.
	Protocol string `json:"protocol"`
	// Formats lists the CLIProxyAPI wire formats this face declares. The host
	// passes these through and translates every other client protocol into one
	// of them.
	Formats []string `json:"formats"`
	// Endpoint is the upstream endpoint family this face talks to.
	Endpoint string `json:"endpoint"`
	// Main reports whether this face carries the sidebar page and the plugin
	// configuration. Exactly one face does.
	Main bool `json:"main"`
	// ModelCount is how many enabled model ids this face serves.
	ModelCount int `json:"model_count"`
	// Models samples those ids, so the dashboard can say which models a missing
	// or disabled face would take away.
	Models []string `json:"models,omitempty"`
}

// faceModelSample caps how many model ids are listed per face.
const faceModelSample = 5

// StatePayload builds the dashboard read model without contacting the upstream.
func (r *Runtime) StatePayload() StatePayload {
	return r.stateWithRows(r.modelRows(nil), nil)
}

func (r *Runtime) stateWithRows(rows []ModelRow, upstream *UpstreamSummary) StatePayload {
	settings := r.Settings()
	resolver := r.Resolver()
	siblings := make([]SiblingRow, 0, len(roles.All))
	for _, role := range roles.All {
		served := resolver.EntriesForFamily(role.Name)
		sample := make([]string, 0, faceModelSample)
		for _, entry := range served {
			if len(sample) == faceModelSample {
				break
			}
			sample = append(sample, entry.ID)
		}
		siblings = append(siblings, SiblingRow{
			PluginID: role.PluginSuffix,
			Role:     string(role.Name),
			Provider: role.Provider,
			Label:    role.Label,
			Protocol: role.Protocol,
			Formats:  append([]string(nil), role.Formats...),
			Endpoint: role.Endpoint,
			// The main face is the one that registers the management API, which
			// is the chat build; it owns the page and the configuration.
			Main:       role.Name == roles.Chat,
			ModelCount: len(served),
			Models:     sample,
		})
	}
	_ = upstream
	return StatePayload{
		Role:           string(r.role.Name),
		Provider:       r.role.Provider,
		Version:        r.Version(),
		PluginID:       r.PluginID(),
		BaseURL:        settings.BaseURL,
		SessionMode:    settings.SessionMode,
		Session:        settings.Session,
		DefaultProxy:   settings.DefaultProxy,
		ProxyURL:       settings.ProxyURL,
		HostProxyURL:   settings.HostProxyURL,
		StatePath:      r.sharedPath,
		StateDir:       settings.StateDir,
		ModelsDefault:  settings.ModelConfig("__default__").Proxy,
		TimeoutSeconds: settings.RequestTimeoutSeconds,
		Plan:           settings.Plan,
		Debug:          settings.Debug,
		Keys: func() []KeyState {
			out := make([]KeyState, len(settings.Keys))
			for i, k := range settings.Keys {
				out[i] = KeyState{
					Name:     k.Name,
					Masked:   k.Masked(),
					Priority: k.Priority,
					Enabled:  k.EnabledOrDefault(),
				}
			}
			return out
		}(),
		Models:      rows,
		Siblings:    siblings,
		ConfigError: r.LastError(),
		StartedAt:   r.startedAt,
		Extra:       settings.ExtraHeaders,
		Accounts:    r.accountsState(),
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
//
// The resolver publishes client-facing ids, so a model renamed by its aliases
// yields one entry per alias. Rows are regrouped by the configured model: the
// dashboard shows one row per real model, carrying every id it answers to.
func (r *Runtime) modelRows(upstreamIDs []string) []ModelRow {
	resolver := r.Resolver()
	settings := r.Settings()

	consulted := len(upstreamIDs) > 0
	shadowed := resolver.Shadowed()

	rows := make([]ModelRow, 0, len(resolver.Entries())+len(upstreamIDs))
	index := make(map[string]int, len(resolver.Entries()))
	seen := make(map[string]struct{}, len(resolver.Entries())+len(upstreamIDs))
	for _, entry := range resolver.Entries() {
		// Both ids count as covered: the alias is what clients call, and the
		// model id it replaced is what the live upstream list still publishes,
		// so neither may come back as a "discovered" row.
		seen[entry.ID] = struct{}{}
		seen[entry.BaseID] = struct{}{}
		if position, exists := index[entry.BaseID]; exists {
			rows[position].ClientIDs = append(rows[position].ClientIDs, entry.ID)
			rows[position].Aliased = rows[position].Aliased || entry.Alias
			continue
		}
		setting := settings.ModelConfig(entry.BaseID)
		rejected := shadowedAliases(shadowed, entry.BaseID)
		rows = append(rows, ModelRow{
			ID:                entry.BaseID,
			ClientIDs:         []string{entry.ID},
			Aliased:           entry.Alias,
			Upstream:          entry.BaseID,
			DisplayName:       entry.DisplayName,
			Family:            string(entry.Family),
			FamilySource:      string(entry.FamilySource),
			Enabled:           entry.Enabled,
			Proxy:             setting.Proxy,
			Aliases:           setting.Aliases,
			Served:            entry.Enabled && entry.Family == r.role.Name,
			Catalogued:        entry.Catalogued,
			UpstreamAvailable: !consulted || containsString(upstreamIDs, entry.BaseID),
			AliasShadowed:     len(rejected) > 0,
			ShadowedAliases:   rejected,
			Note:              entry.Note,
		})
		index[entry.BaseID] = len(rows) - 1
	}

	// Ids the upstream publishes that nobody serves yet.
	for _, id := range upstreamIDs {
		if _, exists := seen[id]; exists {
			continue
		}
		family, source := settings.FamilyWithSource(id)
		rows = append(rows, ModelRow{
			ID:                id,
			ClientIDs:         []string{id},
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

// shadowedAliases lists the aliases one model declared that had to be rejected,
// sorted so the dashboard renders them deterministically.
func shadowedAliases(shadowed map[string]string, baseID string) []string {
	var out []string
	for alias, owner := range shadowed {
		if owner == baseID {
			out = append(out, alias)
		}
	}
	sort.Strings(out)
	return out
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
