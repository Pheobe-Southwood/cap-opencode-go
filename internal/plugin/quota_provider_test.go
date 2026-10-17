package plugin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"

	"github.com/cpa-opencodego/cap-opencode-go/internal/abi"
	"github.com/cpa-opencodego/cap-opencode-go/internal/config"
	"github.com/cpa-opencodego/cap-opencode-go/internal/roles"
)

// usageServerStub serves the upstream /usage payload and records the credential
// it was called with.
func usageServerStub(t *testing.T, status int, body string) (*httptest.Server, *string) {
	t.Helper()
	auth := new(string)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		*auth = request.Header.Get("Authorization")
		if request.URL.Path != "/usage" {
			t.Errorf("unexpected path %q", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, auth
}

const usageBody = `{"usage":{
  "rolling":{"status":"ok","percent":32,"resetsAt":"2026-10-17T06:34:03Z"},
  "weekly":{"status":"ok","percent":60,"resetsAt":"2026-10-19T00:00:00Z"},
  "monthly":{"status":"ok","percent":45,"resetsAt":"2026-11-04T16:44:04Z"}
}}`

// newQuotaRuntime builds a main-face runtime whose base URL points at the stub.
func newQuotaRuntime(t *testing.T, baseURL string) *Runtime {
	t.Helper()
	runtime := New(rolesChat())
	settings := config.Default()
	settings.BaseURL = baseURL
	settings.Plan = "go-plus"
	runtime.applySettings(settings)
	return runtime
}

func quotaRequest(t *testing.T, request pluginapi.QuotaFetchRequest) []byte {
	t.Helper()
	raw, errMarshal := json.Marshal(quotaFetchRequest{QuotaFetchRequest: request})
	if errMarshal != nil {
		t.Fatalf("marshal quota request: %v", errMarshal)
	}
	return raw
}

func authFileJSON(t *testing.T, key string) []byte {
	t.Helper()
	raw, errMarshal := json.Marshal(map[string]any{
		"type":       "opencode-go",
		"name":       "main",
		"api_key":    key,
		"managed_by": "cap-opencode-go",
	})
	if errMarshal != nil {
		t.Fatalf("marshal auth file: %v", errMarshal)
	}
	return raw
}

func TestQuotaIdentifierAndDescribe(t *testing.T) {
	runtime := newQuotaRuntime(t, "http://127.0.0.1:1")

	var identifier identifierResponse
	if errUnmarshal := json.Unmarshal(envelopeResult(t, runtime.Dispatch(pluginabi.MethodQuotaIdentifier, nil)), &identifier); errUnmarshal != nil {
		t.Fatalf("decode identifier: %v", errUnmarshal)
	}
	// The identifier has to equal the provider key the host links accounts by.
	if identifier.Identifier != rolesChat().Provider {
		t.Fatalf("identifier = %q, want %q", identifier.Identifier, rolesChat().Provider)
	}

	var described pluginapi.QuotaDescribeResponse
	if errUnmarshal := json.Unmarshal(envelopeResult(t, runtime.Dispatch(pluginabi.MethodQuotaDescribe, nil)), &described); errUnmarshal != nil {
		t.Fatalf("decode describe: %v", errUnmarshal)
	}
	if len(described.SupportedProviders) != 1 || described.SupportedProviders[0] != rolesChat().Provider {
		t.Fatalf("supported providers = %v", described.SupportedProviders)
	}
	if described.DisplayName == "" {
		t.Fatalf("display name must not be empty")
	}
	if described.SupportsReset {
		t.Fatalf("OpenCode Go has no reset endpoint, SupportsReset must be false")
	}
}

func TestQuotaFetchMapsWindowsForTheManagementPage(t *testing.T) {
	server, auth := usageServerStub(t, http.StatusOK, usageBody)
	runtime := newQuotaRuntime(t, server.URL)

	raw := runtime.Dispatch(pluginabi.MethodQuotaFetch, quotaRequest(t, pluginapi.QuotaFetchRequest{
		AuthIndex:   "index-1",
		AuthID:      "opencode-go-main.json",
		Provider:    rolesChat().Provider,
		StorageJSON: authFileJSON(t, "oc_sk_from_file"),
	}))
	var response pluginapi.QuotaFetchResponse
	if errUnmarshal := json.Unmarshal(envelopeResult(t, raw), &response); errUnmarshal != nil {
		t.Fatalf("decode quota response: %v", errUnmarshal)
	}
	if *auth != "Bearer oc_sk_from_file" {
		t.Fatalf("upstream authorization = %q, want the key from the auth file", *auth)
	}
	if len(response.Groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(response.Groups))
	}
	group := response.Groups[0]
	if group.DisplayName != "OpenCode Go · main" {
		t.Fatalf("group display name = %q", group.DisplayName)
	}
	if len(group.Buckets) != 3 {
		t.Fatalf("buckets = %d, want 3", len(group.Buckets))
	}
	// The webui resolves its localized title from the window value and orders the
	// buckets five-hour → weekly → rest, so these strings are part of the contract.
	wantWindows := []string{"five_hour", "weekly", "monthly"}
	wantRemaining := []float64{0.68, 0.40, 0.55}
	wantReset := []string{"2026-10-17T06:34:03Z", "2026-10-19T00:00:00Z", "2026-11-04T16:44:04Z"}
	for index, bucket := range group.Buckets {
		if bucket.Window != wantWindows[index] {
			t.Errorf("bucket %d window = %q, want %q", index, bucket.Window, wantWindows[index])
		}
		if diff := bucket.RemainingFraction - wantRemaining[index]; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("bucket %d remaining fraction = %v, want %v", index, bucket.RemainingFraction, wantRemaining[index])
		}
		if bucket.ResetTime != wantReset[index] {
			t.Errorf("bucket %d reset time = %q, want %q", index, bucket.ResetTime, wantReset[index])
		}
		if bucket.Description == "" {
			t.Errorf("bucket %d has no description", index)
		}
	}
	if response.Subscription == nil || response.Subscription.Plan != "Go Plus" {
		t.Fatalf("subscription plan = %+v, want the configured Go Plus label", response.Subscription)
	}
}

func TestQuotaFetchAcceptsEveryCredentialSource(t *testing.T) {
	server, auth := usageServerStub(t, http.StatusOK, usageBody)
	runtime := newQuotaRuntime(t, server.URL)

	cases := []struct {
		name    string
		request pluginapi.QuotaFetchRequest
	}{
		{
			name: "metadata",
			request: pluginapi.QuotaFetchRequest{
				AuthIndex: "index-2",
				Metadata:  map[string]any{"api_key": "oc_sk_metadata"},
			},
		},
		{
			name: "attributes",
			request: pluginapi.QuotaFetchRequest{
				AuthIndex:  "index-3",
				Attributes: map[string]string{"api_key": "oc_sk_attributes"},
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			raw := runtime.Dispatch(pluginabi.MethodQuotaFetch, quotaRequest(t, testCase.request))
			var response pluginapi.QuotaFetchResponse
			if errUnmarshal := json.Unmarshal(envelopeResult(t, raw), &response); errUnmarshal != nil {
				t.Fatalf("decode quota response: %v", errUnmarshal)
			}
			if len(response.Groups) != 1 || len(response.Groups[0].Buckets) != 3 {
				t.Fatalf("unexpected shape: %+v", response.Groups)
			}
			if *auth == "" {
				t.Fatalf("upstream was not called")
			}
		})
	}
}

func TestQuotaFetchReportsUpstreamFailures(t *testing.T) {
	server, _ := usageServerStub(t, http.StatusUnauthorized, `{"error":"invalid key"}`)
	runtime := newQuotaRuntime(t, server.URL)

	raw := runtime.Dispatch(pluginabi.MethodQuotaFetch, quotaRequest(t, pluginapi.QuotaFetchRequest{
		AuthIndex:   "index-4",
		StorageJSON: authFileJSON(t, "oc_sk_revoked"),
	}))
	var envelope abi.Envelope
	if errUnmarshal := json.Unmarshal(raw, &envelope); errUnmarshal != nil {
		t.Fatalf("decode envelope: %v", errUnmarshal)
	}
	if envelope.OK || envelope.Error == nil {
		t.Fatalf("a revoked credential must fail, got %s", raw)
	}
	if envelope.Error.Code != "quota_unavailable" {
		t.Fatalf("error code = %q", envelope.Error.Code)
	}
	if !strings.Contains(envelope.Error.Message, "401") || !strings.Contains(envelope.Error.Message, "凭据无效") {
		t.Fatalf("error message = %q, want the HTTP status and the cause", envelope.Error.Message)
	}
}

func TestQuotaFetchRejectsAnAccountWithoutAKey(t *testing.T) {
	runtime := newQuotaRuntime(t, "http://127.0.0.1:1")
	raw := runtime.Dispatch(pluginabi.MethodQuotaFetch, quotaRequest(t, pluginapi.QuotaFetchRequest{AuthIndex: "index-5"}))
	var envelope abi.Envelope
	if errUnmarshal := json.Unmarshal(raw, &envelope); errUnmarshal != nil {
		t.Fatalf("decode envelope: %v", errUnmarshal)
	}
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != "quota_unavailable" {
		t.Fatalf("missing key must fail with quota_unavailable, got %s", raw)
	}
}

func TestQuotaResetIsExplicitlyUnsupported(t *testing.T) {
	runtime := newQuotaRuntime(t, "http://127.0.0.1:1")
	var response pluginapi.QuotaResetResponse
	if errUnmarshal := json.Unmarshal(envelopeResult(t, runtime.Dispatch(pluginabi.MethodQuotaReset, nil)), &response); errUnmarshal != nil {
		t.Fatalf("decode reset response: %v", errUnmarshal)
	}
	if response.Success {
		t.Fatalf("OpenCode Go cannot reset quota")
	}
	if response.Message == "" {
		t.Fatalf("reset must explain why it is unsupported")
	}
}

// The host only registers a quota provider on the face that declares the
// capability, and only the main face may declare it: one provider key maps to
// one quota plugin.
func TestOnlyTheMainFaceDeclaresTheQuotaProvider(t *testing.T) {
	for _, role := range []string{"chat", "responses", "messages", "gemini"} {
		runtime := New(roles.ByName(role))
		runtime.applySettings(config.Default())
		var registration registration
		if errUnmarshal := json.Unmarshal(envelopeResult(t, runtime.registrationPayload()), &registration); errUnmarshal != nil {
			t.Fatalf("%s: decode registration: %v", role, errUnmarshal)
		}
		want := role == "chat"
		if registration.Capabilities.QuotaProvider != want {
			t.Errorf("%s: quota_provider = %v, want %v", role, registration.Capabilities.QuotaProvider, want)
		}
	}
}
