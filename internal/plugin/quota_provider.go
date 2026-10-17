package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"

	"github.com/cpa-opencodego/cap-opencode-go/internal/abi"
	"github.com/cpa-opencodego/cap-opencode-go/internal/brand"
	"github.com/cpa-opencodego/cap-opencode-go/internal/config"
	"github.com/cpa-opencodego/cap-opencode-go/internal/pricing"
	"github.com/cpa-opencodego/cap-opencode-go/internal/quota"
)

// quotaWindowNames maps an upstream usage window to the value the host
// management centre understands. The webui resolves the localized title from
// this value ("5 小时限额" / "周限额" / "月限额") and orders the buckets
// five-hour → weekly → the rest, so an unknown window still renders last.
var quotaWindowNames = map[string]string{
	"rolling": "five_hour",
	"weekly":  "weekly",
	"monthly": "monthly",
}

// handleQuotaDescribe answers quota.describe. One provider key is served, so the
// host shows exactly one OpenCode Go entry in the quota plugin list.
func (r *Runtime) handleQuotaDescribe() []byte {
	return okJSON(pluginapi.QuotaDescribeResponse{
		SupportedProviders: []string{accountProvider()},
		DisplayName:        brand.Name,
		SupportsReset:      false,
	})
}

// quotaFetchRequest mirrors the host payload. The embedded struct carries the
// auth context; host_callback_id is accepted but unused because the plugin talks
// to the upstream with its own HTTP client (the host's client is not serialized).
type quotaFetchRequest struct {
	pluginapi.QuotaFetchRequest
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

// handleQuotaFetch answers quota.fetch: it reads the credential of the account
// the host asked about and returns the three usage windows as normalized quota,
// which the management centre renders next to the Antigravity/Codex accounts.
func (r *Runtime) handleQuotaFetch(request []byte) []byte {
	var payload quotaFetchRequest
	if len(request) > 0 {
		if errUnmarshal := json.Unmarshal(request, &payload); errUnmarshal != nil {
			return abi.Fail("invalid_request", fmt.Sprintf("解析配额请求失败: %v", errUnmarshal), http.StatusBadRequest)
		}
	}
	settings := r.Settings()

	key, source, errKey := credentialFromPayload(payload.QuotaFetchRequest)
	if errKey != nil {
		return quotaFail(errKey)
	}
	if masked := key.Masked(); masked != "" {
		r.trace("quota: fetch", map[string]any{
			"auth_index": payload.AuthIndex,
			"key_name":   key.Name,
			"masked":     masked,
			"source":     source,
		})
	}

	// A per-account base URL (for example a self-hosted gateway) wins over the
	// plugin-wide setting.
	if base := strings.TrimSpace(metadataString(payload.Metadata, "base_url")); base != "" {
		settings.BaseURL = strings.TrimRight(base, "/")
	}

	windows, errFetch := quota.FetchKey(context.Background(), settings, key)
	if errFetch != nil {
		return quotaFail(errFetch)
	}
	return okJSON(quotaResponse(settings, key, windows))
}

// quotaResponse converts the upstream usage windows into the host's normalized
// quota shape: one group per account and one bucket per window.
func quotaResponse(settings config.Settings, key config.Key, windows []quota.Window) pluginapi.QuotaFetchResponse {
	name := strings.TrimSpace(key.Name)
	if name == "" {
		name = "OpenCode Go"
	}
	group := pluginapi.QuotaGroup{
		DisplayName: brand.Name + " · " + name,
		Buckets:     make([]pluginapi.QuotaBucket, 0, len(windows)),
	}
	for _, window := range windows {
		remaining := 1 - window.Percent/100
		if remaining < 0 {
			remaining = 0
		}
		if remaining > 1 {
			remaining = 1
		}
		// Round away float noise (0.68, not 0.6799999999999999): the number is
		// rendered as a percentage, and the raw quotient is not useful precision.
		remaining = math.Round(remaining*1e6) / 1e6
		bucket := pluginapi.QuotaBucket{
			Window:            quotaWindowName(window.Key),
			RemainingFraction: remaining,
			Description:       windowDescription(window),
		}
		if !window.ResetsAt.IsZero() {
			bucket.ResetTime = window.ResetsAt.UTC().Format(time.RFC3339)
		}
		group.Buckets = append(group.Buckets, bucket)
	}
	return pluginapi.QuotaFetchResponse{
		Subscription: &pluginapi.QuotaSubscription{
			Plan: pricing.NormalizePlan(settings.Plan).Label(),
		},
		Groups: []pluginapi.QuotaGroup{group},
	}
}

// quotaWindowName maps an upstream window key onto the window value the webui
// recognizes; an unknown window keeps its own name so it is still rendered.
func quotaWindowName(key string) string {
	if mapped, ok := quotaWindowNames[strings.ToLower(strings.TrimSpace(key))]; ok {
		return mapped
	}
	return strings.TrimSpace(key)
}

// windowDescription is the per-bucket detail line. The webui uses the bucket
// description as the bucket label when it does not recognize the window, so the
// text names the window and carries the numbers instead of repeating a title the
// page already localizes.
func windowDescription(window quota.Window) string {
	label := strings.TrimSpace(window.Label)
	if label == "" {
		label = window.Key
	}
	used := clampPercent(window.Percent)
	remaining := 100 - used
	return fmt.Sprintf("%s：已用 %.0f%% · 剩余 %.0f%%", label, used, remaining)
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

// handleQuotaReset answers quota.reset. OpenCode Go has no reset endpoint, and
// the management page hides the reset button because quota.describe reports
// SupportsReset=false; the handler still answers explicitly instead of failing
// with an unknown method.
func (r *Runtime) handleQuotaReset(request []byte) []byte {
	return okJSON(pluginapi.QuotaResetResponse{
		Success: false,
		Message: "OpenCode Go 不支持重置额度",
	})
}

// credentialFromPayload recovers the API key of the account the host asked
// about. The host sends the persisted auth file as StorageJSON and the parsed
// auth metadata alongside it; both are consulted so an operator-created auth file
// works as well as a mirrored one.
func credentialFromPayload(request pluginapi.QuotaFetchRequest) (config.Key, string, error) {
	name := strings.TrimSpace(request.AuthID)
	if name == "" {
		name = strings.TrimSpace(request.AuthIndex)
	}
	if raw := strings.TrimSpace(string(request.StorageJSON)); raw != "" {
		var stored map[string]any
		if errUnmarshal := json.Unmarshal(request.StorageJSON, &stored); errUnmarshal == nil {
			if key := firstString(stored, "api_key", "apiKey", "key", "token"); key != "" {
				if label := firstString(stored, "name", "label", "email"); label != "" {
					name = label
				}
				return config.Key{Name: name, Key: key}, "auth-file", nil
			}
		}
	}
	if key := metadataString(request.Metadata, "api_key", "apiKey", "key"); key != "" {
		if label := metadataString(request.Metadata, "name", "label", "email"); label != "" {
			name = label
		}
		return config.Key{Name: name, Key: key}, "metadata", nil
	}
	if key := strings.TrimSpace(request.Attributes["api_key"]); key != "" {
		return config.Key{Name: name, Key: key}, "attributes", nil
	}
	return config.Key{}, "", fmt.Errorf("该账号没有可用的 API Key（auth_index=%s）", request.AuthIndex)
}

// metadataString returns the first non-empty string value among keys.
func metadataString(metadata map[string]any, keys ...string) string {
	if len(metadata) == 0 {
		return ""
	}
	for _, key := range keys {
		if value, ok := metadata[key].(string); ok {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

// firstString returns the first non-empty string value among keys.
func firstString(values map[string]any, keys ...string) string {
	return metadataString(values, keys...)
}

// quotaFail reports an upstream problem to the host. The management page shows
// the message next to the account, so it names the real cause instead of a
// generic failure.
func quotaFail(err error) []byte {
	return abi.Fail("quota_unavailable", err.Error(), http.StatusBadGateway)
}
