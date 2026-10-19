// Package config holds the plugin settings, their defaults, and the shared
// runtime snapshot that the main plugin publishes for its sibling providers.
package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/models"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/pricing"
	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
)

// parseProxyURL validates a proxy URL. "direct" and "none" mean "no proxy".
func parseProxyURL(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	switch strings.ToLower(trimmed) {
	case "", "direct", "none":
		return "", nil
	}
	parsed, errParse := url.Parse(trimmed)
	if errParse != nil {
		return "", fmt.Errorf("proxy-url %q is not a valid URL: %w", trimmed, errParse)
	}
	switch parsed.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return "", fmt.Errorf("proxy-url %q must use http, https, socks5, or socks5h", trimmed)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("proxy-url %q is missing a host", trimmed)
	}
	return trimmed, nil
}

// Proxy modes recognised per model and as the instance default.
const (
	// ProxyInherit uses the plugin proxy-url, falling back to the host's
	// configured upstream proxy.
	ProxyInherit = "inherit"
	// ProxyDirect forces a connection without a proxy.
	ProxyDirect = "direct"
	// ProxyOn forces a connection through the resolved proxy URL.
	ProxyOn = "proxy"
)

// Defaults for unset settings.
const (
	DefaultSessionMode    = "auto"
	DefaultSession        = "$CPA-SESSION-ID"
	DefaultProxyMode      = ProxyInherit
	DefaultTimeoutSeconds = 900
	DefaultStateDirName   = "cpa-opencodego"
	// LegacyStateDirName is the v1 state directory. The main face copies its
	// counters into the new directory once, at register, so an upgrade keeps the
	// statistics history without the operator doing anything.
	LegacyStateDirName  = "cap-opencode-go"
	SharedStateFileName = "shared.json"
	// DefaultPlan is used by the 统计 tab until the operator picks a plan. The
	// upstream /usage endpoint reports percentages, not the subscription tier,
	// so the tier has to be configured rather than guessed.
	DefaultPlan = string(pricing.PlanGo)
)

// Endpoint layout values. Auto is the default: a base URL that ends in /v1 (or
// /v1beta) keeps the v1.x single-base behaviour, anything else uses the
// per-family layout the OpenCode Go migration introduced.
const (
	LayoutAuto   = "auto"
	LayoutLegacy = "legacy"
	LayoutFamily = "family"
)

// Developer role handling for OpenAI-shaped bodies. OpenCode Go's new router
// rejects the Chat Completions "developer" role on some gateway nodes even
// though it is a legal OpenAI role. "retry-only" (default) passes "developer"
// through verbatim and only rewrites it to "system" upon receiving an upstream
// 400 rejection; "rewrite" proactively rewrites it before every request;
// "keep" leaves the request byte-for-byte as the host translated it without retry.
const (
	DeveloperRoleRetryOnly = "retry-only"
	DeveloperRoleRewrite   = "rewrite"
	DeveloperRoleKeep      = "keep"
)

// Reasoning replay handling for OpenAI-shaped chat bodies. DeepSeek's thinking
// mode requires the reasoning_content of every assistant history message to be
// passed back, and some OpenCode Go gateway nodes enforce that literally; a
// client that drops it for older turns (or for compacted history) then gets
// HTTP 400 "The `reasoning_content` in the thinking mode must be passed back to
// the API". "retry-only" (default) passes the body through verbatim, backfilling
// placeholders and retrying only if the 400 rejection is received, and remembers
// the session sticky for future turns; "auto" proactively fills only thinking-mode
// requests; "always" fills every chat request; "off" leaves the body byte-for-byte
// untouched.
const (
	ReasoningReplayRetryOnly = "retry-only"
	ReasoningReplayAuto      = "auto"
	ReasoningReplayAlways    = "always"
	ReasoningReplayOff       = "off"
)

// DefaultReasoningPlaceholder is written into assistant history messages whose
// reasoning_content the client did not replay. It is deliberately neutral: the
// upstream only needs the field to be present, and replaying stale
// chain-of-thought is known to degrade long agentic sessions.
const DefaultReasoningPlaceholder = "[reasoning unavailable]"

// Key is one upstream credential.
type Key struct {
	Name     string `yaml:"name" json:"name"`
	Key      string `yaml:"key" json:"key"`
	Priority int    `yaml:"priority" json:"priority"`
	Weight   *int   `yaml:"weight,omitempty" json:"weight,omitempty"`
	Enabled  *bool  `yaml:"enabled" json:"enabled,omitempty"`
}

// EnabledOrDefault reports whether the key participates in rotation.
func (k Key) EnabledOrDefault() bool {
	if k.Enabled == nil {
		return true
	}
	return *k.Enabled
}

// Masked renders the key for display without exposing the secret.
func (k Key) Masked() string {
	trimmed := strings.TrimSpace(k.Key)
	switch {
	case trimmed == "":
		return ""
	case len(trimmed) <= 12:
		return strings.Repeat("*", len(trimmed))
	default:
		return trimmed[:8] + "…" + trimmed[len(trimmed)-4:]
	}
}

// ModelSetting is the per-model override.
type ModelSetting struct {
	// Enabled controls whether this provider claims and serves the model.
	Enabled *bool `yaml:"enabled" json:"enabled,omitempty"`
	// Proxy selects inherit, direct, or proxy for this model.
	Proxy string `yaml:"proxy" json:"proxy,omitempty"`
	// Family optionally overrides the protocol family of an unknown model.
	Family string `yaml:"family" json:"family,omitempty"`
	// Aliases lists extra model ids that clients may call for this model.
	// Every alias is registered and routed exactly like the original id.
	Aliases []string `yaml:"aliases" json:"aliases,omitempty"`
}

// Settings is the plugin configuration under plugins.configs.<pluginID>.
type Settings struct {
	BaseURL               string            `yaml:"base-url" json:"base-url"`
	SessionMode           string            `yaml:"session-mode" json:"session-mode"`
	Session               string            `yaml:"session" json:"session"`
	ProxyURL              string            `yaml:"proxy-url" json:"proxy-url"`
	DefaultProxy          string            `yaml:"default-proxy" json:"default-proxy"`
	ExtraHeaders          map[string]string `yaml:"extra-headers" json:"extra-headers,omitempty"`
	RequestTimeoutSeconds int               `yaml:"request-timeout-seconds" json:"request-timeout-seconds"`
	StateDir              string            `yaml:"state-dir" json:"state-dir"`
	// Plan is the OpenCode Go subscription tier used by the statistics tab to
	// turn a model's list-price spend into a percentage of its monthly limit.
	Plan string `yaml:"plan" json:"plan"`
	// EndpointLayout selects auto, legacy, or family URL assembly.
	EndpointLayout string `yaml:"endpoint-layout" json:"endpoint-layout"`
	// BaseURLs optionally pins one API root per protocol family. A pinned value
	// is used verbatim (the family's resource path is appended to it).
	BaseURLs map[string]string `yaml:"base-urls" json:"base-urls,omitempty"`
	// DeveloperRole is rewrite (default) or keep; it only affects OpenAI-shaped
	// chat/responses bodies.
	DeveloperRole string `yaml:"developer-role" json:"developer-role"`
	// ReasoningReplay is auto (default), always, or off; it decides whether the
	// plugin fills reasoning_content on assistant history messages of
	// OpenAI-shaped chat requests.
	ReasoningReplay string `yaml:"reasoning-replay" json:"reasoning-replay"`
	// RequestLog enables the 请求日志 tab's per-request log. Nil means enabled.
	RequestLog    *bool                   `yaml:"request-log" json:"request-log,omitempty"`
	Debug         bool                    `yaml:"debug" json:"debug"`
	Keys          []Key                   `yaml:"keys" json:"keys,omitempty"`
	Models        map[string]ModelSetting `yaml:"models" json:"models,omitempty"`
	ModelsDefault ModelSetting            `yaml:"models-default" json:"models-default"`
	// HostProxyURL is filled from the host config summary and is never read
	// from YAML.
	HostProxyURL string `yaml:"-" json:"host-proxy-url,omitempty"`
	// BuildVersion is stamped by the runtime for diagnostics and is never read
	// from YAML or shared between builds.
	BuildVersion string `yaml:"-" json:"-"`
}

// Default returns the built-in defaults.
func Default() Settings {
	return Settings{
		BaseURL:               roles.DefaultBaseURL,
		SessionMode:           DefaultSessionMode,
		Session:               DefaultSession,
		DefaultProxy:          DefaultProxyMode,
		RequestTimeoutSeconds: DefaultTimeoutSeconds,
		Plan:                  DefaultPlan,
		EndpointLayout:        LayoutAuto,
		DeveloperRole:         DeveloperRoleRetryOnly,
		ReasoningReplay:       ReasoningReplayRetryOnly,
	}
}

// Parse decodes the YAML subtree the host delivers in plugin.register and
// plugin.reconfigure, applies defaults, and validates the result.
func Parse(raw []byte) (Settings, error) {
	settings := Default()
	trimmed := strings.TrimSpace(string(raw))
	if trimmed != "" {
		if errUnmarshal := yaml.Unmarshal([]byte(trimmed), &settings); errUnmarshal != nil {
			return Settings{}, fmt.Errorf("parse plugin config: %w", errUnmarshal)
		}
	}
	settings.normalize()
	if errValidate := settings.Validate(); errValidate != nil {
		return settings, errValidate
	}
	return settings, nil
}

func (s *Settings) normalize() {
	if strings.TrimSpace(s.BaseURL) == "" {
		s.BaseURL = roles.DefaultBaseURL
	}
	s.BaseURL = strings.TrimRight(strings.TrimSpace(s.BaseURL), "/")
	if strings.TrimSpace(s.SessionMode) == "" {
		s.SessionMode = DefaultSessionMode
	}
	if strings.TrimSpace(s.Session) == "" {
		s.Session = DefaultSession
	}
	if strings.TrimSpace(s.DefaultProxy) == "" {
		s.DefaultProxy = DefaultProxyMode
	}
	if s.RequestTimeoutSeconds <= 0 {
		s.RequestTimeoutSeconds = DefaultTimeoutSeconds
	}
	for i := range s.Keys {
		s.Keys[i].Name = strings.TrimSpace(s.Keys[i].Name)
		s.Keys[i].Key = strings.TrimSpace(s.Keys[i].Key)
		if s.Keys[i].Priority == 0 && s.Keys[i].Weight != nil && *s.Keys[i].Weight > 0 {
			s.Keys[i].Priority = *s.Keys[i].Weight
		}
	}
	if s.Models == nil {
		s.Models = map[string]ModelSetting{}
	}
	for id, setting := range s.Models {
		if strings.TrimSpace(setting.Proxy) != "" {
			setting.Proxy = normalizeProxy(setting.Proxy)
		}
		setting.Family = normalizeFamily(setting.Family)
		setting.Aliases = normalizeAliases(setting.Aliases)
		s.Models[id] = setting
	}
	// An unset models-default must not mask the instance-wide default-proxy, so
	// only a value the operator actually wrote is normalised.
	if strings.TrimSpace(s.ModelsDefault.Proxy) != "" {
		s.ModelsDefault.Proxy = normalizeProxy(s.ModelsDefault.Proxy)
	}
	s.Plan = string(pricing.NormalizePlan(s.Plan))
	s.EndpointLayout = normalizeLayout(s.EndpointLayout)
	s.DeveloperRole = normalizeDeveloperRole(s.DeveloperRole)
	s.ReasoningReplay = normalizeReasoningReplay(s.ReasoningReplay)
	s.BaseURLs = normalizeBaseURLs(s.BaseURLs)
	s.ProxyURL = strings.TrimSpace(s.ProxyURL)
	if s.ExtraHeaders == nil {
		s.ExtraHeaders = map[string]string{}
	}
	s.StateDir = strings.TrimSpace(s.StateDir)
}

// normalizeAliases trims, de-duplicates, and orders alias names.
func normalizeAliases(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		// The dashboard edits aliases as one comma-separated field.
		for _, part := range strings.Split(value, ",") {
			alias := strings.TrimSpace(part)
			if alias == "" {
				continue
			}
			if _, duplicate := seen[alias]; duplicate {
				continue
			}
			seen[alias] = struct{}{}
			out = append(out, alias)
		}
	}
	sort.Strings(out)
	return out
}

// normalizeFamily lowercases a per-model protocol family and drops values that
// are not one of the four upstream endpoint families, so a typo falls back to
// the catalogue or the id-prefix inference instead of hiding the model.
func normalizeFamily(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}
	if !roles.Name(trimmed).Valid() {
		return ""
	}
	return trimmed
}

func normalizeProxy(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ProxyDirect, "none", "off", "false", "no":
		return ProxyDirect
	case ProxyOn, "on", "true", "yes", "proxy-url":
		return ProxyOn
	case ProxyInherit, "":
		return ProxyInherit
	default:
		return value
	}
}

// normalizeLayout lowercases the layout and falls back to auto for a typo, so
// an unknown value cannot silently pin the wrong URL shape.
func normalizeLayout(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case LayoutLegacy, LayoutFamily:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return LayoutAuto
	}
}

// normalizeDeveloperRole lowercases the developer-role mode and defaults to
// retry-only.
func normalizeDeveloperRole(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DeveloperRoleKeep:
		return DeveloperRoleKeep
	case DeveloperRoleRewrite:
		return DeveloperRoleRewrite
	case DeveloperRoleRetryOnly:
		return DeveloperRoleRetryOnly
	default:
		return DeveloperRoleRetryOnly
	}
}

// normalizeReasoningReplay lowercases the reasoning-replay mode and defaults to
// retry-only.
func normalizeReasoningReplay(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ReasoningReplayAlways:
		return ReasoningReplayAlways
	case ReasoningReplayAuto:
		return ReasoningReplayAuto
	case ReasoningReplayOff, "none", "false", "no":
		return ReasoningReplayOff
	case ReasoningReplayRetryOnly:
		return ReasoningReplayRetryOnly
	default:
		return ReasoningReplayRetryOnly
	}
}

// ReasoningReplayMode returns the normalised reasoning-replay mode.
func (s Settings) ReasoningReplayMode() string {
	return normalizeReasoningReplay(s.ReasoningReplay)
}

// normalizeBaseURLs keeps only the four known family keys with a non-empty
// value, so a mistyped key cannot look like an override that never applies.
func normalizeBaseURLs(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for rawKey, rawValue := range values {
		key := strings.ToLower(strings.TrimSpace(rawKey))
		if !roles.Name(key).Valid() {
			continue
		}
		value := strings.TrimRight(strings.TrimSpace(rawValue), "/")
		if value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// validateHTTPURL rejects a base URL the plugin could not call.
func validateHTTPURL(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parsed, errParse := url.Parse(trimmed)
	if errParse != nil {
		return fmt.Errorf("%q is not a valid URL: %w", trimmed, errParse)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%q must use http or https", trimmed)
	}
	if parsed.Host == "" {
		return fmt.Errorf("%q is missing a host", trimmed)
	}
	return nil
}

// Layout resolves the endpoint layout: explicit wins, otherwise a base URL that
// ends in /v1 (or /v1beta) keeps the v1.x single-base shape and anything else
// uses the per-family shape the OpenCode Go migration introduced.
func (s Settings) Layout() roles.Layout {
	switch strings.ToLower(strings.TrimSpace(s.EndpointLayout)) {
	case LayoutLegacy:
		return roles.LayoutLegacy
	case LayoutFamily:
		return roles.LayoutFamily
	}
	base := strings.ToLower(strings.TrimRight(strings.TrimSpace(s.BaseURL), "/"))
	if base == "" {
		base = strings.ToLower(roles.DefaultBaseURL)
	}
	if strings.HasSuffix(base, "/v1") || strings.HasSuffix(base, "/v1beta") {
		return roles.LayoutLegacy
	}
	return roles.LayoutFamily
}

// baseRoot is the configured base URL without a trailing slash.
func (s Settings) baseRoot() string {
	base := strings.TrimRight(strings.TrimSpace(s.BaseURL), "/")
	if base == "" {
		return roles.DefaultBaseURL
	}
	return base
}

// EndpointURL assembles the upstream URL for one request. A per-family
// override is an API root, so only the role's resource path is appended to it.
func (s Settings) EndpointURL(role roles.Role, model string, stream bool) string {
	if override := strings.TrimSpace(s.BaseURLs[string(role.Name)]); override != "" {
		return strings.TrimRight(override, "/") + role.BuildPath(model, stream)
	}
	return s.baseRoot() + role.BuildPathFor(s.Layout(), model, stream)
}

// ModelsURL is the model-list endpoint for the resolved layout. It always lives
// on the base URL, never on a per-family override.
func (s Settings) ModelsURL() string {
	if s.Layout() == roles.LayoutFamily {
		return s.baseRoot() + "/v1/models"
	}
	return s.baseRoot() + "/models"
}

// UsageURL is the usage endpoint for the resolved layout.
func (s Settings) UsageURL() string {
	if s.Layout() == roles.LayoutFamily {
		return s.baseRoot() + "/v1/usage"
	}
	return s.baseRoot() + "/usage"
}

// DeveloperRoleMode returns the normalised developer-role mode.
func (s Settings) DeveloperRoleMode() string {
	return normalizeDeveloperRole(s.DeveloperRole)
}

// DeveloperRoleRewrite reports whether developer messages are rewritten to
// system for OpenAI-shaped bodies proactively.
func (s Settings) DeveloperRoleRewrite() bool {
	return normalizeDeveloperRole(s.DeveloperRole) == DeveloperRoleRewrite
}

// RequestLogEnabled reports whether the per-request log is written. An unset
// value means enabled.
func (s Settings) RequestLogEnabled() bool {
	if s.RequestLog == nil {
		return true
	}
	return *s.RequestLog
}

// LegacyStatePath is where v1.x kept the shared snapshot.
func (s Settings) LegacyStatePath(root string) string {
	return filepath.Join(root, "data", LegacyStateDirName, SharedStateFileName)
}

// LegacyStateRoot is the v1.x state directory.
func (s Settings) LegacyStateRoot(root string) string {
	return filepath.Join(root, "data", LegacyStateDirName)
}

// Validate reports configuration problems that would make the plugin useless.
func (s Settings) Validate() error {
	if _, errParse := parseProxyURL(s.ProxyURL); errParse != nil {
		return errParse
	}
	if errBase := validateHTTPURL(s.BaseURL); errBase != nil {
		return fmt.Errorf("base-url: %w", errBase)
	}
	for family, value := range s.BaseURLs {
		if errBase := validateHTTPURL(value); errBase != nil {
			return fmt.Errorf("base-urls.%s: %w", family, errBase)
		}
	}
	seenNames := make(map[string]struct{}, len(s.Keys))
	for _, key := range s.Keys {
		if key.EnabledOrDefault() {
			if strings.TrimSpace(key.Name) == "" {
				return fmt.Errorf("keys: enabled key must have a name")
			}
			if key.Key == "" {
				return fmt.Errorf("keys[%q]: key must not be empty", key.Name)
			}
		}
		if key.Name != "" {
			if _, exists := seenNames[key.Name]; exists {
				return fmt.Errorf("keys: duplicate key name %q", key.Name)
			}
			seenNames[key.Name] = struct{}{}
		}
	}
	return nil
}

// EnabledKeys returns the credentials that may be used for a request.
func (s Settings) EnabledKeys() []Key {
	out := make([]Key, 0, len(s.Keys))
	for _, key := range s.Keys {
		if key.EnabledOrDefault() && strings.TrimSpace(key.Key) != "" {
			out = append(out, key)
		}
	}
	return out
}

// ModelConfig resolves the effective per-model setting.
func (s Settings) ModelConfig(id string) ModelSetting {
	setting := ModelSetting{Proxy: s.DefaultProxy}
	if s.ModelsDefault.Proxy != "" {
		setting.Proxy = s.ModelsDefault.Proxy
	}
	if s.ModelsDefault.Enabled != nil {
		setting.Enabled = s.ModelsDefault.Enabled
	}
	if override, ok := s.Models[id]; ok {
		if override.Enabled != nil {
			setting.Enabled = override.Enabled
		}
		if override.Proxy != "" {
			setting.Proxy = override.Proxy
		}
		if override.Family != "" {
			setting.Family = override.Family
		}
		if len(override.Aliases) > 0 {
			setting.Aliases = append([]string(nil), override.Aliases...)
		}
	}
	if setting.Proxy == "" {
		setting.Proxy = ProxyInherit
	}
	return setting
}

// ModelEnabled reports whether the model participates.
func (s Settings) ModelEnabled(id string) bool {
	enabled := s.ModelConfig(id).Enabled
	return enabled == nil || *enabled
}

// Family returns the protocol family to use for a model.
func (s Settings) Family(id string) roles.Name {
	family, _ := s.FamilyWithSource(id)
	return family
}

// FamilyWithSource returns the protocol family for a model together with the
// reason it was chosen, so the dashboard can mark an inferred guess. The order
// is: explicit per-model family, built-in catalogue, id-prefix inference.
func (s Settings) FamilyWithSource(id string) (roles.Name, models.FamilySource) {
	trimmed := strings.TrimSpace(id)
	if override := normalizeFamily(s.ModelConfig(trimmed).Family); override != "" {
		return roles.Name(override), models.FamilyFromConfig
	}
	if models.IsCatalogued(trimmed) {
		return models.FamilyFor(trimmed), models.FamilyFromCatalogue
	}
	return models.FamilyHint(trimmed), models.FamilyFromHint
}

// ResolvedProxyURL returns the proxy a model should use, and whether a proxy is
// used at all. An empty URL with useProxy true means "inherited but unset".
func (s Settings) ResolvedProxyURL(id string) (proxyURL string, useProxy bool) {
	pluginProxy, pluginProxySet := usableProxy(s.ProxyURL)
	hostProxy, hostProxySet := usableProxy(s.HostProxyURL)
	// An explicit "direct"/"none" at instance level is an instruction to bypass
	// every inherited proxy, matching CLIProxyAPI's own proxy-url semantics.
	pluginForcesDirect := pluginProxySet == false && strings.TrimSpace(s.ProxyURL) != ""

	switch s.ModelConfig(id).Proxy {
	case ProxyDirect:
		return "", false
	case ProxyOn:
		if pluginProxySet {
			return pluginProxy, true
		}
		if pluginForcesDirect {
			return "", false
		}
		return hostProxy, hostProxySet
	default:
		if pluginProxySet {
			return pluginProxy, true
		}
		if pluginForcesDirect {
			return "", false
		}
		return hostProxy, hostProxySet
	}
}

// usableProxy normalises a configured proxy value. "direct" and "none" are the
// documented way to explicitly bypass every inherited proxy.
func usableProxy(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	switch strings.ToLower(trimmed) {
	case "", "direct", "none":
		return "", false
	default:
		return trimmed, true
	}
}

// Timeout is the upstream request timeout.
func (s Settings) Timeout() time.Duration {
	return time.Duration(s.RequestTimeoutSeconds) * time.Second
}

// StatePath resolves the shared state file path.
func (s Settings) StatePath(root string) string {
	if s.StateDir != "" {
		return filepath.Join(s.StateDir, SharedStateFileName)
	}
	return filepath.Join(root, "data", DefaultStateDirName, SharedStateFileName)
}

// StateRoot resolves the directory that holds plugin-owned state.
func (s Settings) StateRoot(root string) string {
	if s.StateDir != "" {
		return s.StateDir
	}
	return filepath.Join(root, "data", DefaultStateDirName)
}

// SharedState is the runtime snapshot the main plugin publishes so that the
// sibling provider plugins share one configuration and one key pool.
type SharedState struct {
	// Version is the plugin version that wrote the snapshot.
	Version string `json:"version"`
	// WrittenAt is the publication time.
	WrittenAt time.Time `json:"written_at"`
	// Settings is the resolved main-plugin configuration.
	Settings Settings `json:"settings"`
}

// WriteSharedState atomically publishes the runtime snapshot.
func WriteSharedState(path string, state SharedState) error {
	if errMkdir := os.MkdirAll(filepath.Dir(path), 0o700); errMkdir != nil {
		return fmt.Errorf("create state directory: %w", errMkdir)
	}
	raw, errMarshal := json.MarshalIndent(state, "", "  ")
	if errMarshal != nil {
		return fmt.Errorf("encode shared state: %w", errMarshal)
	}
	tmp := path + ".tmp"
	if errWrite := os.WriteFile(tmp, raw, 0o600); errWrite != nil {
		return fmt.Errorf("write shared state: %w", errWrite)
	}
	if errRename := os.Rename(tmp, path); errRename != nil {
		return fmt.Errorf("publish shared state: %w", errRename)
	}
	return nil
}

// ReadSharedState loads the runtime snapshot.
func ReadSharedState(path string) (SharedState, error) {
	raw, errRead := os.ReadFile(path)
	if errRead != nil {
		return SharedState{}, errRead
	}
	var state SharedState
	if errUnmarshal := json.Unmarshal(raw, &state); errUnmarshal != nil {
		return SharedState{}, fmt.Errorf("decode shared state: %w", errUnmarshal)
	}
	return state, nil
}
