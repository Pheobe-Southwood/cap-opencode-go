package reqlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendReadAndSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "requests-chat.jsonl")
	store, errOpen := Open(path, "chat")
	if errOpen != nil {
		t.Fatalf("Open: %v", errOpen)
	}
	t.Cleanup(store.Shutdown)

	started := time.Now().Add(-time.Minute)
	store.Append(Entry{At: started, Model: "deepseek-v4.1-flash", Role: "chat", Status: 200, Input: 10, Output: 20, TotalTokens: 30})
	store.Append(Entry{At: started.Add(time.Second), Model: "glm-5.3", Status: 500, Error: "boom"})

	snapshot := store.Snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("snapshot rows = %d, want 2", len(snapshot))
	}
	if snapshot[0].ID == "" || snapshot[1].ID == "" {
		t.Error("every row needs an id")
	}
	if snapshot[1].Error != "boom" || !snapshot[1].Failed() {
		t.Errorf("failed row = %+v", snapshot[1])
	}

	// The file is the cross-face read path, so it must already contain both rows.
	rows, errRead := ReadFile(path, "chat")
	if errRead != nil {
		t.Fatalf("ReadFile: %v", errRead)
	}
	if len(rows) != 2 || rows[0].Model != "deepseek-v4.1-flash" {
		t.Fatalf("ReadFile rows = %+v", rows)
	}
}

func TestOpenSkipsCorruptAndTornLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "requests-messages.jsonl")
	content := strings.Join([]string{
		`{"id":"a","role":"messages","model":"claude-haiku-5-5","at":"` + time.Now().UTC().Format(time.RFC3339Nano) + `"}`,
		`{broken json`,
		`{"id":"b","role":"messages","model":"claude-haiku-5-5","at":"` + time.Now().UTC().Format(time.RFC3339Nano) + `"}`,
	}, "\n") + "\n" + `{"id":"torn","role":"messages","model":"claude-haiku-5-5`
	if errWrite := os.WriteFile(path, []byte(content), 0o600); errWrite != nil {
		t.Fatalf("write fixture: %v", errWrite)
	}
	store, _ := Open(path, "messages")
	t.Cleanup(store.Shutdown)
	rows := store.Snapshot()
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want the two valid rows", len(rows))
	}
	// The corrupt line is surfaced as a warning; the torn trailing line is an
	// in-flight append and must not be reported as corruption.
	if got := store.LastError(); !strings.Contains(got, "损坏") {
		t.Errorf("a corrupt line must surface a warning, got %q", got)
	}
}

func TestOpenPrunesExpiredAndExcessRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "requests-chat.jsonl")
	store, errOpen := Open(path, "chat")
	if errOpen != nil {
		t.Fatalf("Open: %v", errOpen)
	}
	old := time.Now().Add(-Retention - time.Hour)
	store.Append(Entry{At: old, Model: "expired", Role: "chat"})
	for i := 0; i < 5; i++ {
		store.Append(Entry{At: time.Now(), Model: "keep", Role: "chat"})
	}
	store.Shutdown()

	reloaded, errReload := Open(path, "chat")
	if errReload != nil {
		t.Fatalf("reload: %v", errReload)
	}
	t.Cleanup(reloaded.Shutdown)
	rows := reloaded.Snapshot()
	if len(rows) != 5 {
		t.Fatalf("rows after retention = %d, want 5 (%+v)", len(rows), rows)
	}
	for _, row := range rows {
		if row.Model == "expired" {
			t.Fatal("an expired row must be dropped on load")
		}
	}
}

func TestAppendTrimsToMaxEntries(t *testing.T) {
	store, errOpen := Open(filepath.Join(t.TempDir(), "requests-chat.jsonl"), "chat")
	if errOpen != nil {
		t.Fatalf("Open: %v", errOpen)
	}
	t.Cleanup(store.Shutdown)
	for i := 0; i < MaxEntries+25; i++ {
		store.Append(Entry{Model: "m", Role: "chat"})
	}
	if got := len(store.Snapshot()); got != MaxEntries {
		t.Fatalf("snapshot rows = %d, want %d", got, MaxEntries)
	}
}

func TestErrorTextIsBoundedToOneLine(t *testing.T) {
	store, errOpen := Open(filepath.Join(t.TempDir(), "requests-chat.jsonl"), "chat")
	if errOpen != nil {
		t.Fatalf("Open: %v", errOpen)
	}
	t.Cleanup(store.Shutdown)
	store.Append(Entry{Model: "m", Error: "line one\nline two\r\n" + strings.Repeat("x", 600)})
	rows := store.Snapshot()
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
	if strings.ContainsAny(rows[0].Error, "\r\n") {
		t.Errorf("error must stay one line: %q", rows[0].Error)
	}
	if len(rows[0].Error) > 320 {
		t.Errorf("error is not truncated: %d bytes", len(rows[0].Error))
	}
}

// A face must not read another face's file even when the path is wrong.
func TestReadFileIgnoresForeignRole(t *testing.T) {
	path := filepath.Join(t.TempDir(), "requests-chat.jsonl")
	store, _ := Open(path, "chat")
	store.Append(Entry{Model: "m", Role: "chat"})
	store.Shutdown()
	rows, errRead := ReadFile(path, "messages")
	if errRead != nil {
		t.Fatalf("ReadFile: %v", errRead)
	}
	if len(rows) != 0 {
		t.Errorf("foreign rows = %+v", rows)
	}
}
