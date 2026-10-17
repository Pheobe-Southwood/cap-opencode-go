// Command replay-tracker imports OpenCode Go usage from the CAP Token Usage
// Tracker plugin's bbolt database into this plugin's stats files.
//
// It is a one-time migration tool, not part of the plugin .so. Run it while
// CLIProxyAPI is stopped so the running plugin cannot flush over the merge.
// Only token, latency and timing counters are copied; the tracker's API keys,
// prompts and response bodies are never read into the output.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"

	modelinfo "github.com/cpa-opencodego/cap-opencode-go/internal/models"
	"github.com/cpa-opencodego/cap-opencode-go/internal/pricing"
	"github.com/cpa-opencodego/cap-opencode-go/internal/roles"
	"github.com/cpa-opencodego/cap-opencode-go/internal/stats"
)

const (
	requestsBucket  = "requests"
	markerFileName  = "replay-tracker-import.json"
	statsChatRole   = "chat"
	statsChatFile   = "stats-chat.json"
	maxTrackerValue = 8 << 20
)

type options struct {
	trackerPath  string
	statsDir     string
	settingsPath string
	beforeRaw    string
	apply        bool
	force        bool
}

type trackerRequest struct {
	Sequence uint64    `json:"sequence"`
	Time     time.Time `json:"time"`

	Provider    string `json:"provider"`
	Executor    string `json:"executor_type"`
	Model       string `json:"model"`
	Alias       string `json:"alias"`
	Source      string `json:"source"`
	Failed      bool   `json:"failed"`
	FailureCode int    `json:"failure_status"`

	Requests            uint64 `json:"requests"`
	FailedRequests      uint64 `json:"failed_requests"`
	InputTokens         uint64 `json:"input_tokens"`
	OutputTokens        uint64 `json:"output_tokens"`
	ReasoningTokens     uint64 `json:"reasoning_tokens"`
	CachedTokens        uint64 `json:"cached_tokens"`
	CacheReadTokens     uint64 `json:"cache_read_tokens"`
	CacheCreationTokens uint64 `json:"cache_creation_tokens"`
	TotalTokens         uint64 `json:"total_tokens"`

	LatencyNS    uint64 `json:"latency_ns"`
	TTFTNS       uint64 `json:"ttft_ns"`
	GenerationNS uint64 `json:"generation_ns"`
}

type sharedSettings struct {
	Settings struct {
		Models map[string]struct {
			Aliases []string `json:"aliases"`
		} `json:"models"`
	} `json:"settings"`
}

type marker struct {
	Source     string    `json:"source"`
	Tracker    string    `json:"tracker"`
	Cutoff     time.Time `json:"cutoff"`
	Imported   int       `json:"imported"`
	First      time.Time `json:"first"`
	Last       time.Time `json:"last"`
	Tokens     int64     `json:"total_tokens"`
	CostNano   int64     `json:"cost_nano_usd"`
	FinishedAt time.Time `json:"finished_at"`
}

type importRecord struct {
	at     time.Time
	sample stats.UsageSample
	timing *stats.TimingSample
}

func main() {
	var opt options
	flag.StringVar(&opt.trackerPath, "tracker", "/root/cliproxyapi/data/token-usage-tracker.db", "path to token-usage-tracker.db")
	flag.StringVar(&opt.statsDir, "stats-dir", "/root/cliproxyapi/data/cap-opencode-go", "plugin stats directory")
	flag.StringVar(&opt.settingsPath, "settings", "", "shared.json path (default <stats-dir>/shared.json)")
	flag.StringVar(&opt.beforeRaw, "before", "", "import records strictly before this RFC3339 time (default: earliest existing stats sample)")
	flag.BoolVar(&opt.apply, "apply", false, "write the imported counters (default is a dry run)")
	flag.BoolVar(&opt.force, "force", false, "ignore the one-time import marker")
	flag.Parse()

	if err := run(opt); err != nil {
		fmt.Fprintln(os.Stderr, "replay-tracker:", err)
		os.Exit(1)
	}
}

func run(opt options) error {
	if strings.TrimSpace(opt.trackerPath) == "" || strings.TrimSpace(opt.statsDir) == "" {
		return errors.New("tracker and stats-dir are required")
	}
	if opt.settingsPath == "" {
		opt.settingsPath = filepath.Join(opt.statsDir, "shared.json")
	}
	aliases, errAliases := loadAliases(opt.settingsPath)
	if errAliases != nil {
		return errAliases
	}
	families, errFamilies := loadFamilies(opt.statsDir)
	if errFamilies != nil {
		return errFamilies
	}

	cutoff, errCutoff := resolveCutoff(opt.beforeRaw, opt.statsDir)
	if errCutoff != nil {
		return errCutoff
	}
	records, summary, errRead := readTracker(opt.trackerPath, cutoff, aliases, families)
	if errRead != nil {
		return errRead
	}
	fmt.Printf("tracker: %s\n", opt.trackerPath)
	fmt.Printf("cutoff:  %s\n", printableTime(cutoff))
	fmt.Printf("scanned: %d opencode-go records; selected: %d; skipped by cutoff: %d; invalid: %d\n",
		summary.scanned, len(records), summary.cutoffSkipped, summary.invalid)
	if len(records) > 0 {
		fmt.Printf("range:   %s .. %s\n", records[0].at.Format(time.RFC3339), records[len(records)-1].at.Format(time.RFC3339))
		fmt.Printf("tokens:  %d; list price: $%.6f; latency samples: %d; TTFT samples: %d\n",
			summary.tokens, float64(summary.costNano)/1e9, summary.withLatency, summary.withTTFT)
		printModelSummary(summary.models)
	}
	if !opt.apply {
		fmt.Println("dry run: no files written (add -apply after stopping CLIProxyAPI)")
		return nil
	}
	if !opt.force {
		if _, errStat := os.Stat(filepath.Join(opt.statsDir, markerFileName)); errStat == nil {
			return fmt.Errorf("import marker %s already exists; refusing to import twice (use -force only after restoring a backup)", markerFileName)
		}
	}
	if len(records) == 0 {
		fmt.Println("nothing to import")
		return nil
	}

	backupDir, errBackup := backupStats(opt.statsDir)
	if errBackup != nil {
		return errBackup
	}
	fmt.Printf("backup:  %s\n", backupDir)

	store, errOpen := stats.Open(filepath.Join(opt.statsDir, statsChatFile), statsChatRole)
	if errOpen != nil {
		return fmt.Errorf("open stats: %w", errOpen)
	}
	for index, record := range records {
		store.ObserveUsage(record.sample)
		if record.timing != nil {
			store.ObserveTiming(*record.timing)
		}
		if (index+1)%1000 == 0 {
			fmt.Printf("imported %d/%d\n", index+1, len(records))
		}
	}
	store.Shutdown()
	if errStore := store.LastError(); errStore != "" {
		return fmt.Errorf("write stats: %s", errStore)
	}

	first, last := records[0].at, records[0].at
	for _, record := range records[1:] {
		if record.at.Before(first) {
			first = record.at
		}
		if record.at.After(last) {
			last = record.at
		}
	}
	m := marker{
		Source:     "cap-token-usage-tracker",
		Tracker:    opt.trackerPath,
		Cutoff:     cutoff,
		Imported:   len(records),
		First:      first,
		Last:       last,
		Tokens:     summary.tokens,
		CostNano:   summary.costNano,
		FinishedAt: time.Now().UTC(),
	}
	raw, _ := json.MarshalIndent(m, "", "  ")
	if errWrite := os.WriteFile(filepath.Join(opt.statsDir, markerFileName), raw, 0o600); errWrite != nil {
		return fmt.Errorf("write import marker: %w", errWrite)
	}
	fmt.Printf("imported %d records into %s (marker: %s)\n", len(records), statsChatFile, markerFileName)
	return nil
}

type readSummary struct {
	scanned       int
	cutoffSkipped int
	invalid       int
	tokens        int64
	costNano      int64
	models        map[string]modelSummary
	withLatency   int
	withTTFT      int
}

type modelSummary struct {
	requests int
	tokens   int64
	costNano int64
}

func readTracker(path string, cutoff time.Time, aliases map[string]string, families map[string]string) ([]importRecord, readSummary, error) {
	var out []importRecord
	summary := readSummary{models: map[string]modelSummary{}}
	db, errOpen := bolt.Open(path, 0o600, &bolt.Options{ReadOnly: true, Timeout: 5 * time.Second})
	if errOpen != nil {
		return nil, summary, fmt.Errorf("open tracker (stop CLIProxyAPI first): %w", errOpen)
	}
	defer db.Close()

	errView := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(requestsBucket))
		if bucket == nil {
			return errors.New("tracker has no requests bucket")
		}
		return bucket.ForEach(func(_, value []byte) error {
			if len(value) == 0 || len(value) > maxTrackerValue {
				return nil
			}
			var raw trackerRequest
			if errUnmarshal := json.Unmarshal(value, &raw); errUnmarshal != nil {
				summary.invalid++
				return nil
			}
			if !isOpenCodeProvider(raw.Provider) {
				return nil
			}
			summary.scanned++
			if raw.Time.IsZero() {
				summary.invalid++
				return nil
			}
			if !cutoff.IsZero() && !raw.Time.Before(cutoff) {
				summary.cutoffSkipped++
				return nil
			}
			record, ok := trackerRecord(raw, aliases, families)
			if !ok {
				summary.invalid++
				return nil
			}
			summary.tokens += record.sample.Total
			summary.costNano += record.sample.CostNanoUSD
			if record.sample.Latency > 0 {
				summary.withLatency++
			}
			if record.timing != nil {
				summary.withTTFT++
			}
			item := summary.models[record.sample.Model]
			item.requests++
			item.tokens += record.sample.Total
			item.costNano += record.sample.CostNanoUSD
			summary.models[record.sample.Model] = item
			out = append(out, record)
			return nil
		})
	})
	if errView != nil {
		return nil, summary, fmt.Errorf("read tracker: %w", errView)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].at.Equal(out[j].at) {
			return out[i].sample.Model < out[j].sample.Model
		}
		return out[i].at.Before(out[j].at)
	})
	return out, summary, nil
}

func trackerRecord(raw trackerRequest, aliases map[string]string, families map[string]string) (importRecord, bool) {
	family := familyFor(raw.Provider, canonicalModel(raw.Model, raw.Alias, aliases), families)
	model := canonicalModel(raw.Model, raw.Alias, aliases)
	if model == "" {
		return importRecord{}, false
	}
	tokens := pricing.Tokens{
		Input:         uintToInt64(raw.InputTokens),
		Output:        uintToInt64(raw.OutputTokens),
		Reasoning:     uintToInt64(raw.ReasoningTokens),
		Cached:        uintToInt64(raw.CachedTokens),
		CacheRead:     uintToInt64(raw.CacheReadTokens),
		CacheCreation: uintToInt64(raw.CacheCreationTokens),
	}
	total := uintToInt64(raw.TotalTokens)
	if total == 0 {
		total = tokens.Input + tokens.Output + tokens.CacheRead + tokens.CacheCreation
	}
	costNano, priced := pricing.Cost(model, tokens, family != roles.Messages, family == roles.Gemini, raw.Time)
	tpsOutput := tokens.Output
	if family == roles.Gemini && tokens.Reasoning > 0 {
		tpsOutput += tokens.Reasoning
	}
	if tpsOutput < 0 {
		tpsOutput = 0
	}
	unmetered := tokens.Input == 0 && tokens.Output == 0 && tokens.Reasoning == 0 &&
		tokens.Cached == 0 && tokens.CacheRead == 0 && tokens.CacheCreation == 0
	record := importRecord{
		at: raw.Time.UTC(),
		sample: stats.UsageSample{
			Model:         model,
			Family:        string(family),
			At:            raw.Time.UTC(),
			Input:         tokens.Input,
			Output:        tokens.Output,
			Reasoning:     tokens.Reasoning,
			Cached:        tokens.Cached,
			CacheRead:     tokens.CacheRead,
			CacheCreation: tokens.CacheCreation,
			Total:         total,
			CostNanoUSD:   costNano,
			Priced:        priced,
			Stream:        raw.TTFTNS > 0,
			Failed:        raw.Failed,
			Unmetered:     unmetered,
			Latency:       time.Duration(raw.LatencyNS),
			TPSOutput:     tpsOutput,
		},
	}
	if !raw.Failed && raw.TTFTNS > 0 {
		generation := time.Duration(raw.GenerationNS)
		if generation <= 0 && raw.LatencyNS > raw.TTFTNS {
			generation = time.Duration(raw.LatencyNS - raw.TTFTNS)
		}
		record.timing = &stats.TimingSample{
			Model:      model,
			Family:     string(family),
			At:         raw.Time.UTC(),
			TTFT:       time.Duration(raw.TTFTNS),
			Generation: generation,
		}
	}
	return record, true
}

func printModelSummary(models map[string]modelSummary) {
	type row struct {
		model string
		item  modelSummary
	}
	rows := make([]row, 0, len(models))
	for model, item := range models {
		rows = append(rows, row{model: model, item: item})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].item.costNano != rows[j].item.costNano {
			return rows[i].item.costNano > rows[j].item.costNano
		}
		return rows[i].item.tokens > rows[j].item.tokens
	})
	if len(rows) > 12 {
		rows = rows[:12]
	}
	for _, entry := range rows {
		fmt.Printf("  %-36s requests=%-6d tokens=%-12d $%.6f\n",
			entry.model, entry.item.requests, entry.item.tokens, float64(entry.item.costNano)/1e9)
	}
}

func isOpenCodeProvider(provider string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	return provider == "openai-compatible-opencode go" || provider == "cap-opencode-go" ||
		strings.HasPrefix(provider, "cap-opencode-go-")
}

func familyFor(provider, model string, families map[string]string) roles.Name {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch {
	case strings.HasSuffix(provider, "-gemini"):
		return roles.Gemini
	case strings.HasSuffix(provider, "-messages"):
		return roles.Messages
	case strings.HasSuffix(provider, "-responses"):
		return roles.Responses
	}
	if family := strings.TrimSpace(families[strings.ToLower(model)]); family != "" {
		return roles.ByName(family).Name
	}
	return modelinfo.FamilyFor(model)
}

func canonicalModel(model, alias string, aliases map[string]string) string {
	fallback := ""
	for _, candidate := range []string{model, alias} {
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		if candidate == "" {
			continue
		}
		if base := aliases[candidate]; base != "" {
			return base
		}
		if fallback == "" {
			fallback = candidate
		}
	}
	return fallback
}

func loadAliases(path string) (map[string]string, error) {
	aliases := map[string]string{}
	raw, errRead := os.ReadFile(path)
	if errRead != nil {
		if errors.Is(errRead, os.ErrNotExist) {
			return aliases, nil
		}
		return nil, fmt.Errorf("read settings: %w", errRead)
	}
	var shared sharedSettings
	if errUnmarshal := json.Unmarshal(raw, &shared); errUnmarshal != nil {
		return nil, fmt.Errorf("decode settings: %w", errUnmarshal)
	}
	for base, setting := range shared.Settings.Models {
		base = strings.ToLower(strings.TrimSpace(base))
		if base == "" {
			continue
		}
		aliases[base] = base
		for _, alias := range setting.Aliases {
			alias = strings.ToLower(strings.TrimSpace(alias))
			if alias != "" {
				aliases[alias] = base
			}
		}
	}
	return aliases, nil
}

func loadFamilies(statsDir string) (map[string]string, error) {
	families := map[string]string{}
	paths, errGlob := filepath.Glob(filepath.Join(statsDir, "stats-*.json"))
	if errGlob != nil {
		return nil, errGlob
	}
	for _, path := range paths {
		raw, errRead := os.ReadFile(path)
		if errRead != nil {
			continue
		}
		var file struct {
			Models map[string]struct {
				Family string `json:"family"`
			} `json:"models"`
		}
		if errUnmarshal := json.Unmarshal(raw, &file); errUnmarshal != nil {
			continue
		}
		for model, value := range file.Models {
			if value.Family != "" {
				families[strings.ToLower(model)] = value.Family
			}
		}
	}
	return families, nil
}

func resolveCutoff(raw, statsDir string) (time.Time, error) {
	if strings.TrimSpace(raw) != "" {
		parsed, errParse := time.Parse(time.RFC3339, strings.TrimSpace(raw))
		if errParse != nil {
			return time.Time{}, fmt.Errorf("parse -before: %w", errParse)
		}
		return parsed.UTC(), nil
	}
	earliest, errEarliest := earliestStatsTime(statsDir)
	if errEarliest != nil {
		return time.Time{}, errEarliest
	}
	return earliest, nil
}

func earliestStatsTime(statsDir string) (time.Time, error) {
	var earliest time.Time
	paths, errGlob := filepath.Glob(filepath.Join(statsDir, "stats-*.json"))
	if errGlob != nil {
		return time.Time{}, errGlob
	}
	for _, path := range paths {
		raw, errRead := os.ReadFile(path)
		if errRead != nil {
			continue
		}
		var file struct {
			Models map[string]struct {
				FirstUsed time.Time `json:"first_used"`
			} `json:"models"`
		}
		if errUnmarshal := json.Unmarshal(raw, &file); errUnmarshal != nil {
			continue
		}
		for _, model := range file.Models {
			if !model.FirstUsed.IsZero() && (earliest.IsZero() || model.FirstUsed.Before(earliest)) {
				earliest = model.FirstUsed.UTC()
			}
		}
	}
	return earliest, nil
}

func backupStats(statsDir string) (string, error) {
	backupDir := filepath.Join(os.TempDir(), "cpa-opencode-go-stats-backup-"+time.Now().UTC().Format("20060102T150405"))
	if errMkdir := os.MkdirAll(backupDir, 0o700); errMkdir != nil {
		return "", fmt.Errorf("create backup dir: %w", errMkdir)
	}
	paths, errGlob := filepath.Glob(filepath.Join(statsDir, "stats-*.json"))
	if errGlob != nil {
		return "", errGlob
	}
	for _, path := range paths {
		if errCopy := copyFile(path, filepath.Join(backupDir, filepath.Base(path))); errCopy != nil {
			return "", fmt.Errorf("backup %s: %w", filepath.Base(path), errCopy)
		}
	}
	return backupDir, nil
}

func copyFile(src, dst string) error {
	in, errOpen := os.Open(src)
	if errOpen != nil {
		return errOpen
	}
	defer in.Close()
	out, errCreate := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if errCreate != nil {
		return errCreate
	}
	if _, errCopy := io.Copy(out, in); errCopy != nil {
		_ = out.Close()
		return errCopy
	}
	return out.Close()
}

func printableTime(value time.Time) string {
	if value.IsZero() {
		return "(no existing stats; importing all)"
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func uintToInt64(value uint64) int64 {
	const maxInt64 = uint64(1<<63 - 1)
	if value > maxInt64 {
		return int64(maxInt64)
	}
	return int64(value)
}
