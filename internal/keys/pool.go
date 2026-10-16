// Package keys implements the multi-credential rotation pool. It provides
// weighted round-robin ordering plus per-credential failure cooldowns so an
// exhausted or rate-limited key steps aside without failing the request.
package keys

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cpa-opencodego/cap-opencode-go/internal/config"
)

// State is the observable health of one credential.
type State struct {
	Name         string    `json:"name"`
	Masked       string    `json:"masked"`
	Weight       int       `json:"weight"`
	Enabled      bool      `json:"enabled"`
	Successes    int64     `json:"successes"`
	Failures     int64     `json:"failures"`
	LastUsed     time.Time `json:"last_used,omitempty"`
	LastStatus   int       `json:"last_status,omitempty"`
	LastError    string    `json:"last_error,omitempty"`
	CooldownTill time.Time `json:"cooldown_till,omitempty"`
}

type entry struct {
	key   config.Key
	state State
}

// Pool rotates over the configured credentials.
type Pool struct {
	mu       sync.Mutex
	entries  []*entry
	cursor   int
	cooldown time.Duration
}

// New builds a pool from the configured keys.
func New(settings config.Settings) *Pool {
	pool := &Pool{cooldown: settings.KeyCooldown()}
	pool.Replace(settings)
	return pool
}

// Replace swaps the credential set, preserving health state for keys whose
// secret is unchanged.
func (p *Pool) Replace(settings config.Settings) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cooldown = settings.KeyCooldown()
	previous := make(map[string]*entry, len(p.entries))
	for _, item := range p.entries {
		previous[item.key.Key] = item
	}

	next := make([]*entry, 0, len(settings.Keys))
	for index, key := range settings.Keys {
		item := &entry{key: key}
		if existing, ok := previous[key.Key]; ok {
			item.state = existing.state
		}
		if item.state.Name == "" {
			item.state.Name = key.Name
		}
		if item.state.Name == "" {
			item.state.Name = defaultKeyName(index)
		}
		item.state.Masked = key.Masked()
		item.state.Weight = key.EffectiveWeight()
		item.state.Enabled = key.EnabledOrDefault()
		next = append(next, item)
	}
	p.entries = next
	if p.cursor >= len(next) {
		p.cursor = 0
	}
}

func defaultKeyName(index int) string {
	return "key-" + itoa(index)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	position := len(digits)
	for value > 0 {
		position--
		digits[position] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[position:])
}

// Candidates returns usable credentials in rotation order: the next key first,
// then the remaining keys, with cooling keys moved to the end. When every key
// is cooling the caller still receives the full list so a retry is possible.
func (p *Pool) Candidates() []config.Key {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	usable := make([]*entry, 0, len(p.entries))
	cooling := make([]*entry, 0, len(p.entries))
	for _, item := range p.entries {
		if !item.state.Enabled || strings.TrimSpace(item.key.Key) == "" {
			continue
		}
		if item.state.CooldownTill.After(now) {
			cooling = append(cooling, item)
			continue
		}
		usable = append(usable, item)
	}
	ordered := p.weightedOrder(usable)
	ordered = append(ordered, cooling...)
	out := make([]config.Key, 0, len(ordered))
	for _, item := range ordered {
		out = append(out, item.key)
	}
	return out
}

// weightedOrder expands the candidate list by weight and rotates the cursor,
// yielding smooth weighted round-robin behaviour.
func (p *Pool) weightedOrder(items []*entry) []*entry {
	if len(items) == 0 {
		return nil
	}
	ring := make([]*entry, 0, len(items))
	for _, item := range items {
		weight := item.state.Weight
		if weight <= 0 {
			weight = config.DefaultKeyWeight
		}
		for repeat := 0; repeat < weight; repeat++ {
			ring = append(ring, item)
		}
	}
	if len(ring) == 0 {
		return nil
	}
	start := p.cursor % len(ring)
	p.cursor = (p.cursor + 1) % len(ring)

	seen := make(map[string]struct{}, len(items))
	ordered := make([]*entry, 0, len(items))
	for offset := 0; offset < len(ring); offset++ {
		item := ring[(start+offset)%len(ring)]
		if _, ok := seen[item.key.Key]; ok {
			continue
		}
		seen[item.key.Key] = struct{}{}
		ordered = append(ordered, item)
	}
	return ordered
}

// RecordSuccess marks a credential as healthy.
func (p *Pool) RecordSuccess(secret string, status int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if item := p.find(secret); item != nil {
		item.state.Successes++
		item.state.LastUsed = time.Now()
		item.state.LastStatus = status
		item.state.LastError = ""
		item.state.CooldownTill = time.Time{}
	}
}

// RecordFailure marks a credential as unhealthy for the cooldown window.
func (p *Pool) RecordFailure(secret string, status int, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	item := p.find(secret)
	if item == nil {
		return
	}
	item.state.Failures++
	item.state.LastUsed = time.Now()
	item.state.LastStatus = status
	item.state.LastError = truncate(message, 240)
	if p.cooldown > 0 {
		item.state.CooldownTill = time.Now().Add(p.cooldown)
	}
}

func (p *Pool) find(secret string) *entry {
	for _, item := range p.entries {
		if item.key.Key == secret {
			return item
		}
	}
	return nil
}

// Snapshot returns the health of every configured credential.
func (p *Pool) Snapshot() []State {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]State, 0, len(p.entries))
	for _, item := range p.entries {
		state := item.state
		state.Name = item.state.Name
		state.Enabled = item.state.Enabled
		state.Weight = item.key.EffectiveWeight()
		out = append(out, state)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Size reports how many credentials are configured and enabled.
func (p *Pool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	count := 0
	for _, item := range p.entries {
		if item.state.Enabled && strings.TrimSpace(item.key.Key) != "" {
			count++
		}
	}
	return count
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
