package plugin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/abi"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/config"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
)

func TestMainBuildDeclaresProbeRoute(t *testing.T) {
	raw := New(rolesChat()).handleManagementRegister(nil)
	var decoded struct {
		Routes []struct {
			Method string `json:"Method"`
			Path   string `json:"Path"`
			Menu   string `json:"Menu"`
		} `json:"routes"`
	}
	if errUnmarshal := json.Unmarshal(envelopeResult(t, raw), &decoded); errUnmarshal != nil {
		t.Fatalf("management registration is invalid: %v", errUnmarshal)
	}
	if len(decoded.Routes) != 1 {
		t.Fatalf("expected exactly the probe route, got %d", len(decoded.Routes))
	}
	route := decoded.Routes[0]
	if route.Method != http.MethodGet || route.Path != managementProbePath {
		t.Errorf("unexpected probe route: %+v", route)
	}
	// A GET route with a Menu would be turned into a sidebar entry by the host.
	if route.Menu != "" {
		t.Error("the probe route must not declare a menu")
	}
}

// The probe route is host-authenticated: the plugin only sees calls that passed
// the management middleware, and it answers with one entry per protocol family.
func TestProbeRouteDispatch(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/responses" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"resp_1"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"model not found"}}`))
	}))
	defer server.Close()

	runtime := New(rolesChat())
	settings := config.Default()
	settings.BaseURL = server.URL
	// The mock answers the legacy paths; the route under test is layout-agnostic.
	settings.EndpointLayout = config.LayoutLegacy
	settings.Keys = []config.Key{{Name: "k1", Key: "secret"}}
	runtime.applySettings(settings)

	request, _ := json.Marshal(map[string]any{
		"Method": http.MethodGet,
		"Path":   "/v0/management" + managementProbePath,
		"Query":  url.Values{"model": []string{"mystery-model"}},
	})

	var envelope abi.Envelope
	if errUnmarshal := json.Unmarshal(runtime.handleManagementHandle(request), &envelope); errUnmarshal != nil {
		t.Fatalf("probe response is not an envelope: %v", errUnmarshal)
	}
	if !envelope.OK {
		t.Fatalf("the probe must answer even when no family works: %s", envelope.Error)
	}

	// The plugin answers with a ManagementResponse whose Body carries the JSON.
	var response struct {
		StatusCode int    `json:"StatusCode"`
		Body       []byte `json:"Body"`
	}
	if errUnmarshal := json.Unmarshal(envelope.Result, &response); errUnmarshal != nil {
		t.Fatalf("probe response is invalid: %v", errUnmarshal)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", response.StatusCode, response.Body)
	}

	var payload probePayload
	if errUnmarshal := json.Unmarshal(response.Body, &payload); errUnmarshal != nil {
		t.Fatalf("probe payload is invalid: %v (%s)", errUnmarshal, response.Body)
	}
	if payload.Model != "mystery-model" {
		t.Errorf("model = %q", payload.Model)
	}
	if len(payload.Results) != len(roles.All) {
		t.Fatalf("expected one result per family, got %d", len(payload.Results))
	}
	if payload.Best != string(roles.Responses) {
		t.Errorf("best = %q, want responses", payload.Best)
	}
	if len(paths) != len(roles.All) {
		t.Errorf("expected %d upstream calls, got %d (%v)", len(roles.All), len(paths), paths)
	}
}

func TestProbeRouteRejectsBadInput(t *testing.T) {
	runtime := New(rolesChat())
	runtime.applySettings(config.Default())

	cases := map[string]string{
		"missing model": `{"Method":"GET","Path":"/v0/management` + managementProbePath + `"}`,
		"blank model":   `{"Method":"GET","Path":"/v0/management` + managementProbePath + `","Query":{"model":["   "]}}`,
		"long model":    `{"Method":"GET","Path":"/v0/management` + managementProbePath + `","Query":{"model":["` + strings.Repeat("a", 300) + `"]}}`,
		"spaced model":  `{"Method":"GET","Path":"/v0/management` + managementProbePath + `","Query":{"model":["a b"]}}`,
	}
	for name, body := range cases {
		var envelope abi.Envelope
		if errUnmarshal := json.Unmarshal(runtime.handleManagementHandle([]byte(body)), &envelope); errUnmarshal != nil {
			t.Fatalf("%s: response is not an envelope: %v", name, errUnmarshal)
		}
		if envelope.OK {
			t.Errorf("%s: expected a failure envelope", name)
			continue
		}
		if envelope.Error.HTTPStatus != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, envelope.Error.HTTPStatus)
		}
	}
}

// A model whose family was only guessed is reported as such, so the dashboard
// can ask the operator to confirm it.
func TestModelRowsReportFamilySource(t *testing.T) {
	runtime := New(rolesChat())
	settings := config.Default()
	settings.Models = map[string]config.ModelSetting{
		"gpt-7-orbit":       {},
		"grok-4.7":          {Family: "chat"},
		"deepseek-v4-pro":   {Enabled: boolPtr(false)},
		"claude-next-thing": {},
	}
	runtime.applySettings(settings)

	rows := runtime.modelRows([]string{"gpt-7-orbit", "deepseek-v4-pro", "claude-next-thing", "brand-new-model"})
	byID := map[string]ModelRow{}
	for _, row := range rows {
		byID[row.ID] = row
	}

	if row := byID["gpt-7-orbit"]; row.Family != "responses" || row.FamilySource != "inferred" {
		t.Errorf("gpt-7-orbit = %s/%s, want responses/inferred", row.Family, row.FamilySource)
	}
	if row := byID["grok-4.7"]; row.Family != "chat" || row.FamilySource != "config" {
		t.Errorf("grok-4.7 = %s/%s, want chat/config", row.Family, row.FamilySource)
	}
	if row := byID["deepseek-v4-pro"]; row.Family != "chat" || row.FamilySource != "catalogue" {
		t.Errorf("deepseek-v4-pro = %s/%s, want chat/catalogue", row.Family, row.FamilySource)
	}
	if row := byID["claude-next-thing"]; row.Family != "messages" || row.FamilySource != "inferred" {
		t.Errorf("claude-next-thing = %s/%s, want messages/inferred", row.Family, row.FamilySource)
	}
	// An id only the upstream publishes is listed as discovered.
	discovered := byID["brand-new-model"]
	if !discovered.Discovered || discovered.Enabled || discovered.Family != "chat" {
		t.Errorf("unexpected discovered row: %+v", discovered)
	}
}

func boolPtr(value bool) *bool { return &value }
