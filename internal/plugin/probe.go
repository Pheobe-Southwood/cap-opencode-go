package plugin

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/abi"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/upstream"
)

// managementProbePath is the host-authenticated route the dashboard calls to
// find out which upstream endpoint serves a model. The host prefixes it with
// /v0/management, so the browser reaches it with the management credential the
// CPA web UI already holds.
const managementProbePath = "/opencode-go/probe"

// maxProbeModelLength bounds the model id accepted by the probe route.
const maxProbeModelLength = 200

// probePayload is the probe response.
type probePayload struct {
	// Model is the id that was probed.
	Model string `json:"model"`
	// Best is the first family that served the model, if any.
	Best string `json:"best,omitempty"`
	// Results lists every family that was tried, in tried order.
	Results []upstream.FamilyProbe `json:"results"`
}

var probeInFlight sync.Map

// probeModel tries every upstream protocol family for one model id. Only one
// probe per model runs at a time so a double click cannot double the load.
func (r *Runtime) probeModel(ctx context.Context, model string) (probePayload, error) {
	id := strings.TrimSpace(model)
	if id == "" {
		return probePayload{}, abi.StatusErrorf(http.StatusBadRequest, "missing_model", "缺少 model 参数")
	}
	if len(id) > maxProbeModelLength {
		return probePayload{}, abi.StatusErrorf(http.StatusBadRequest, "invalid_model", "model 参数过长")
	}
	for _, bad := range []string{" ", "\t", "\n", "\r"} {
		if strings.Contains(id, bad) {
			return probePayload{}, abi.StatusErrorf(http.StatusBadRequest, "invalid_model", "model 参数不能包含空白字符")
		}
	}
	if _, busy := probeInFlight.LoadOrStore(id, struct{}{}); busy {
		return probePayload{}, abi.StatusErrorf(http.StatusTooManyRequests, "probe_in_progress",
			"该模型正在探测中，请稍候")
	}
	defer probeInFlight.Delete(id)

	r.refreshShared()
	preferred, _ := r.Settings().FamilyWithSource(id)
	results := r.client.ProbeFamilies(ctx, id, preferred)
	payload := probePayload{Model: id, Results: results}
	for _, item := range results {
		if item.OK {
			payload.Best = item.Family
			break
		}
	}
	return payload, nil
}

// probeRoute is the management route the dashboard calls.
func probeRoute() pluginapi.ManagementRoute {
	return pluginapi.ManagementRoute{
		Method:      http.MethodGet,
		Path:        managementProbePath,
		Description: "探测某个模型由哪个上游协议族提供（每族一次最小请求）",
	}
}
