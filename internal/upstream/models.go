package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/abi"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/config"
)

// modelsCacheTTL bounds how often the dashboard may query the upstream list.
const modelsCacheTTL = 5 * time.Minute

type modelsCache struct {
	mu        sync.Mutex
	ids       []string
	fetchedAt time.Time
	err       string
}

// ListModels returns the model ids OpenCode Go currently publishes. Results are
// cached so that opening the dashboard does not hammer the upstream; pass force
// to refresh.
func (c *Client) ListModels(ctx context.Context, force bool) ([]string, time.Time, string) {
	settings := c.Settings()

	c.modelsCache.mu.Lock()
	cached := c.modelsCache.ids
	fetchedAt := c.modelsCache.fetchedAt
	cachedErr := c.modelsCache.err
	c.modelsCache.mu.Unlock()

	if !force && len(cached) > 0 && time.Since(fetchedAt) < modelsCacheTTL {
		return cached, fetchedAt, cachedErr
	}

	ids, errFetch := c.fetchModels(ctx, settings)
	if errFetch != nil {
		if len(cached) > 0 {
			// Serve the stale list rather than an empty table.
			return cached, fetchedAt, errFetch.Error()
		}
		return nil, time.Time{}, errFetch.Error()
	}

	c.modelsCache.mu.Lock()
	c.modelsCache.ids = ids
	c.modelsCache.fetchedAt = time.Now()
	c.modelsCache.err = ""
	c.modelsCache.mu.Unlock()
	return ids, time.Now(), ""
}

func (c *Client) fetchModels(ctx context.Context, settings config.Settings) ([]string, error) {
	candidates := settings.EnabledKeys()
	if len(candidates) == 0 {
		return nil, abi.StatusErrorf(http.StatusUnauthorized, "no_api_key",
			"没有可用的 API Key，无法从上游拉取模型列表")
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Priority > candidates[j].Priority
	})

	client, errClient := c.httpClient(settings, Call{Model: ""})
	if errClient != nil {
		return nil, errClient
	}

	var lastErr error
	for _, credential := range candidates {
		endpoint := settings.ModelsURL()
		request, errRequest := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if errRequest != nil {
			return nil, errRequest
		}
		request.Header.Set("Authorization", "Bearer "+credential.Key)
		request.Header.Set("Accept", "application/json")
		if settings.Session != "" {
			request.Header.Set("x-opencode-session", settings.Session)
		}

		response, errDo := client.Do(request)
		if errDo != nil {
			lastErr = abi.StatusErrorf(http.StatusBadGateway, "upstream_unreachable",
				"拉取模型列表失败: %v", errDo)
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		_ = response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			lastErr = upstreamError(response.StatusCode, body, credential.Name)
			if !retryableStatus(response.StatusCode) {
				return nil, lastErr
			}
			continue
		}

		var payload struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if errUnmarshal := json.Unmarshal(body, &payload); errUnmarshal != nil {
			return nil, abi.StatusErrorf(http.StatusBadGateway, "invalid_response",
				"解析上游模型列表失败: %v", errUnmarshal)
		}
		ids := make([]string, 0, len(payload.Data))
		seen := make(map[string]struct{}, len(payload.Data))
		for _, item := range payload.Data {
			id := strings.TrimSpace(item.ID)
			if id == "" {
				continue
			}
			if _, duplicate := seen[id]; duplicate {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
		sort.Strings(ids)
		if len(ids) == 0 {
			return nil, fmt.Errorf("上游模型列表为空")
		}
		return ids, nil
	}
	if lastErr == nil {
		lastErr = abi.StatusErrorf(http.StatusBadGateway, "upstream_failed", "拉取模型列表失败")
	}
	return nil, lastErr
}
