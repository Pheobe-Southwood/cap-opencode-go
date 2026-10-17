package quota

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/cpa-opencodego/cap-opencode-go/internal/config"
)

func boolPointer(value bool) *bool { return &value }

// The accounts tab shows the remaining quota of every configured key, including
// the ones that are switched off, and never asks the upstream about a key that
// has no secret at all.
func TestFetchCoversDisabledKeysAndSkipsEmptySecrets(t *testing.T) {
	var (
		askedMu sync.Mutex
		asked   []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/usage" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		// The keys are queried concurrently, so the handler must guard its state.
		askedMu.Lock()
		asked = append(asked, request.Header.Get("Authorization"))
		askedMu.Unlock()
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"usage": map[string]any{
				"rolling": map[string]any{"status": "ok", "percent": 12.5, "resetsAt": "2026-01-02T03:04:05Z"},
				"weekly":  map[string]any{"status": "ok", "percent": 40},
				"monthly": map[string]any{"status": "ok", "percent": 99},
			},
		})
	}))
	defer server.Close()

	settings := config.Default()
	settings.BaseURL = server.URL
	settings.Keys = []config.Key{
		{Name: "off", Key: "oc_sk_off_000000000000", Enabled: boolPointer(false)},
		{Name: "on", Key: "oc_sk_on_111111111111", Enabled: boolPointer(true)},
		{Name: "empty", Key: ""},
	}

	snapshots := Fetch(context.Background(), settings)
	if len(snapshots) != 3 {
		t.Fatalf("snapshots = %d, want 3", len(snapshots))
	}
	if snapshots[0].KeyName != "empty" || snapshots[1].KeyName != "off" || snapshots[2].KeyName != "on" {
		t.Fatalf("snapshots must be sorted by key name: %+v", snapshots)
	}
	if snapshots[0].Error == "" || len(snapshots[0].Windows) != 0 {
		t.Errorf("a key without a secret must be reported, not queried: %+v", snapshots[0])
	}
	if snapshots[1].Enabled {
		t.Error("the disabled key must be flagged as disabled")
	}
	if !snapshots[2].Enabled || len(snapshots[2].Windows) != 3 {
		t.Fatalf("enabled key = %+v", snapshots[2])
	}
	askedMu.Lock()
	defer askedMu.Unlock()
	if len(asked) != 2 {
		t.Errorf("upstream calls = %d, want 2 (the empty key must be skipped): %v", len(asked), asked)
	}

	rolling := snapshots[2].Windows[0]
	if rolling.Key != "rolling" || rolling.Label != "5 小时" || rolling.Percent != 12.5 || rolling.ResetsAt.IsZero() {
		t.Errorf("rolling window = %+v", rolling)
	}
	// Unknown windows are still reported so a new upstream limit shows up.
	if snapshots[2].Windows[2].Label != "本月" {
		t.Errorf("window order = %+v", snapshots[2].Windows)
	}
}
