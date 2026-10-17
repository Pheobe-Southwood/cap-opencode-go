// Package quota reads the (undocumented but live) OpenCode Go usage endpoint
// GET {base}/usage, which reports percent-used and reset time for the rolling,
// weekly, and monthly limit windows of a credential.
package quota

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/cpa-opencodego/cap-opencode-go/internal/config"
)

// Window is one usage limit window.
type Window struct {
	Key      string    `json:"key"`
	Label    string    `json:"label"`
	Status   string    `json:"status,omitempty"`
	Percent  float64   `json:"percent"`
	ResetsAt time.Time `json:"resets_at,omitempty"`
}

// Snapshot is the usage of one credential.
type Snapshot struct {
	KeyName string   `json:"key_name"`
	Masked  string   `json:"masked"`
	Enabled bool     `json:"enabled"`
	Windows []Window `json:"windows,omitempty"`
	Error   string   `json:"error,omitempty"`
}

var windowLabels = map[string]string{
	"rolling": "5 小时",
	"weekly":  "本周",
	"monthly": "本月",
}

var windowOrder = []string{"rolling", "weekly", "monthly"}

type usageEnvelope struct {
	Usage map[string]usageWindow `json:"usage"`
}

type usageWindow struct {
	Status   string  `json:"status"`
	Percent  float64 `json:"percent"`
	ResetsAt string  `json:"resetsAt"`
}

// Fetch queries the usage endpoint for every configured credential, including
// the ones that are currently switched off: the dashboard shows the remaining
// quota of a key before the operator turns it back on. Individual failures are
// reported per credential rather than failing the whole call, and the requests
// run a few at a time so a long key list does not serialise the page load.
func Fetch(ctx context.Context, settings config.Settings) []Snapshot {
	credentials := settings.Keys
	out := make([]Snapshot, len(credentials))
	const parallelism = 4
	semaphore := make(chan struct{}, parallelism)
	done := make(chan int, len(credentials))
	launched := 0
	for index, credential := range credentials {
		out[index] = Snapshot{
			KeyName: credential.Name,
			Masked:  credential.Masked(),
			Enabled: credential.EnabledOrDefault(),
		}
		if strings.TrimSpace(credential.Key) == "" {
			out[index].Error = "未配置 Key 内容"
			continue
		}
		launched++
		go func(index int, credential config.Key) {
			semaphore <- struct{}{}
			defer func() { <-semaphore; done <- index }()
			windows, errFetch := FetchKey(ctx, settings, credential)
			if errFetch != nil {
				out[index].Error = errFetch.Error()
				return
			}
			out[index].Windows = windows
		}(index, credential)
	}
	// Every launched worker is drained before returning: the snapshots it writes
	// must not be handed to the caller while a goroutine still owns an index.
	// A cancelled context makes fetchOne return promptly, so this cannot block.
	for range launched {
		<-done
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].KeyName < out[j].KeyName })
	return out
}

// FetchKey queries the usage endpoint for one credential. The host's quota
// provider RPC serves one credential per call, so the same request/mapping code
// backs both the dashboard and the CPA quota page.
func FetchKey(ctx context.Context, settings config.Settings, credential config.Key) ([]Window, error) {
	endpoint := settings.BaseURL + "/usage"
	request, errRequest := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if errRequest != nil {
		return nil, errRequest
	}
	request.Header.Set("Authorization", "Bearer "+credential.Key)
	if settings.Session != "" {
		request.Header.Set("x-opencode-session", settings.Session)
	}
	request.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second, Transport: quotaTransport(settings, credential)}
	response, errDo := client.Do(request)
	if errDo != nil {
		return nil, fmt.Errorf("连接用量接口失败: %w", errDo)
	}
	defer func() { _ = response.Body.Close() }()

	body, errRead := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if errRead != nil {
		return nil, fmt.Errorf("读取用量接口响应失败: %w", errRead)
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("凭据无效（HTTP %d）", response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("用量接口返回 HTTP %d: %s", response.StatusCode, snippet(body))
	}

	var decoded usageEnvelope
	if errUnmarshal := json.Unmarshal(body, &decoded); errUnmarshal != nil {
		return nil, fmt.Errorf("解析用量接口响应失败: %w", errUnmarshal)
	}
	if len(decoded.Usage) == 0 {
		return nil, fmt.Errorf("用量接口未返回窗口数据")
	}

	windows := make([]Window, 0, len(decoded.Usage))
	for _, name := range windowOrder {
		value, ok := decoded.Usage[name]
		if !ok {
			continue
		}
		window := Window{Key: name, Label: windowLabels[name], Status: value.Status, Percent: value.Percent}
		if value.ResetsAt != "" {
			if parsed, errParse := time.Parse(time.RFC3339, value.ResetsAt); errParse == nil {
				window.ResetsAt = parsed
			}
		}
		windows = append(windows, window)
	}
	for name, value := range decoded.Usage {
		if _, known := windowLabels[name]; known {
			continue
		}
		window := Window{Key: name, Label: name, Status: value.Status, Percent: value.Percent}
		if value.ResetsAt != "" {
			if parsed, errParse := time.Parse(time.RFC3339, value.ResetsAt); errParse == nil {
				window.ResetsAt = parsed
			}
		}
		windows = append(windows, window)
	}
	return windows, nil
}

// quotaTransport mirrors the per-model proxy decision, but /usage is not tied
// to a model, so the instance default applies.
func quotaTransport(settings config.Settings, credential config.Key) http.RoundTripper {
	_ = credential
	proxyURL, useProxy := settings.ResolvedProxyURL("")
	transport := &http.Transport{Proxy: nil, TLSHandshakeTimeout: 15 * time.Second}
	if useProxy && proxyURL != "" {
		if parsed, errParse := parseProxy(proxyURL); errParse == nil {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}
	return transport
}

func snippet(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 200 {
		return text[:200]
	}
	return text
}

func parseProxy(value string) (*url.URL, error) {
	return url.Parse(value)
}
