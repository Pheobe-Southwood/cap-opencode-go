package keys

import (
	"testing"
	"time"

	"github.com/cpa-opencodego/cap-opencode-go/internal/config"
)

func settingsWith(keys ...config.Key) config.Settings {
	settings := config.Default()
	settings.Keys = keys
	settings.KeyCooldownSeconds = 60
	return settings
}

func TestRotationOrderIncludesEveryKeyOnce(t *testing.T) {
	pool := New(settingsWith(
		config.Key{Name: "a", Key: "ka"},
		config.Key{Name: "b", Key: "kb"},
		config.Key{Name: "c", Key: "kc"},
	))
	for round := 0; round < 4; round++ {
		candidates := pool.Candidates()
		if len(candidates) != 3 {
			t.Fatalf("round %d: got %d candidates, want 3", round, len(candidates))
		}
		seen := map[string]bool{}
		for _, candidate := range candidates {
			if seen[candidate.Key] {
				t.Fatalf("round %d: duplicate candidate %q", round, candidate.Key)
			}
			seen[candidate.Key] = true
		}
	}
}

func TestWeightedRotationPrefersHeavierKey(t *testing.T) {
	pool := New(settingsWith(
		config.Key{Name: "light", Key: "kl", Weight: 1},
		config.Key{Name: "heavy", Key: "kh", Weight: 5},
	))
	first := map[string]int{}
	for round := 0; round < 6; round++ {
		first[pool.Candidates()[0].Key]++
	}
	if first["kh"] <= first["kl"] {
		t.Fatalf("heavier key should lead more often: %v", first)
	}
}

func TestCooldownMovesKeyToTheBack(t *testing.T) {
	pool := New(settingsWith(
		config.Key{Name: "a", Key: "ka"},
		config.Key{Name: "b", Key: "kb"},
	))
	pool.RecordFailure("ka", 429, "rate limited")

	candidates := pool.Candidates()
	if len(candidates) != 2 {
		t.Fatalf("cooling keys must stay available as a fallback, got %d", len(candidates))
	}
	if candidates[0].Key != "kb" {
		t.Fatalf("healthy key must lead, got %q", candidates[0].Key)
	}
	if candidates[1].Key != "ka" {
		t.Fatalf("cooling key must trail, got %q", candidates[1].Key)
	}

	// Expiring the cooldown restores normal rotation.
	pool.mu.Lock()
	pool.entries[0].state.CooldownTill = time.Now().Add(-time.Second)
	pool.mu.Unlock()
	pool.RecordSuccess("kb", 200)
	var found bool
	for _, state := range pool.Snapshot() {
		if state.Name == "b" && state.Successes == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("success was not recorded: %+v", pool.Snapshot())
	}
}

func TestDisabledKeyIsExcluded(t *testing.T) {
	disabled := false
	pool := New(settingsWith(
		config.Key{Name: "a", Key: "ka"},
		config.Key{Name: "b", Key: "kb", Enabled: &disabled},
	))
	if pool.Size() != 1 {
		t.Fatalf("Size() = %d, want 1", pool.Size())
	}
	for _, candidate := range pool.Candidates() {
		if candidate.Key == "kb" {
			t.Fatal("disabled key must not be a candidate")
		}
	}
}

func TestReplacePreservesHealth(t *testing.T) {
	pool := New(settingsWith(config.Key{Name: "a", Key: "ka"}))
	pool.RecordSuccess("ka", 200)

	next := settingsWith(config.Key{Name: "renamed", Key: "ka"})
	pool.Replace(next)

	snapshot := pool.Snapshot()
	if len(snapshot) != 1 || snapshot[0].Successes != 1 {
		t.Fatalf("health should survive a rename: %+v", snapshot)
	}
}

func TestAllKeysCoolingStillReturnsCandidates(t *testing.T) {
	pool := New(settingsWith(config.Key{Name: "a", Key: "ka"}))
	pool.RecordFailure("ka", 500, "boom")
	if got := pool.Candidates(); len(got) != 1 {
		t.Fatalf("a single cooling key must still be retryable, got %d", len(got))
	}
}
