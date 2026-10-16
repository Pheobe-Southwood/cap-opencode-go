// Package plugin implements the CLIProxyAPI plugin RPC surface: registration,
// model catalogue publication, model routing, request execution, and the
// management page.
package plugin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"

	"github.com/cpa-opencodego/cap-opencode-go/internal/abi"
	"github.com/cpa-opencodego/cap-opencode-go/internal/catalog"
	"github.com/cpa-opencodego/cap-opencode-go/internal/config"
	"github.com/cpa-opencodego/cap-opencode-go/internal/paths"
	"github.com/cpa-opencodego/cap-opencode-go/internal/roles"
	"github.com/cpa-opencodego/cap-opencode-go/internal/upstream"
)

// Metadata constants.
const (
	pluginName       = "OpenCode Go Provider"
	pluginAuthor     = "cpa-opencodego"
	pluginRepository = "https://github.com/cpa-opencodego/cap-opencode-go"
	// menuLabel is the sidebar entry shown under the 插件 group.
	menuLabel = "OpenCode Go"
)

// Runtime is the process-wide plugin state.
type Runtime struct {
	role      roles.Role
	root      string
	client    *upstream.Client
	settings  atomic.Pointer[config.Settings]
	resolver  atomic.Pointer[catalog.Resolver]
	hostProxy atomic.Pointer[string]

	buildVersion string
	debugMode    atomic.Bool
	debugPath    string

	mu          sync.Mutex
	pluginID    string
	configRaw   []byte
	sharedPath  string
	sharedStamp time.Time
	lastError   string
	startedAt   time.Time
}

// New builds the runtime for one role.
func New(role roles.Role) *Runtime {
	root := paths.ResolveRoot()
	runtime := &Runtime{
		role:      role,
		root:      root,
		client:    upstream.New(config.Default()),
		startedAt: time.Now(),
	}
	runtime.sharedPath = config.Default().StatePath(root)
	return runtime
}

// Debug reports whether diagnostics are written to the state directory.
func (r *Runtime) Debug() bool { return r.debugMode.Load() }

// Version reports the build version.
func (r *Runtime) Version() string {
	if r.buildVersion == "" {
		return "dev"
	}
	return r.buildVersion
}

// SetVersion records the build version stamped by the linker.
func (r *Runtime) SetVersion(value string) {
	r.buildVersion = strings.TrimSpace(value)
}

// Role reports the build role.
func (r *Runtime) Role() roles.Role { return r.role }

// Resolver returns the model id resolver for the active configuration.
func (r *Runtime) Resolver() *catalog.Resolver {
	if current := r.resolver.Load(); current != nil {
		return current
	}
	return catalog.New(r.Settings())
}

// Settings returns the active settings. The host publishes its upstream proxy
// through the model registration summary, which arrives after the first
// register, so it is overlaid on every read.
func (r *Runtime) Settings() config.Settings {
	settings := config.Default()
	if current := r.settings.Load(); current != nil {
		settings = *current
	}
	if settings.HostProxyURL == "" {
		settings.HostProxyURL = r.hostProxyURL()
	}
	return settings
}

// Dispatch handles one plugin RPC.
func (r *Runtime) Dispatch(method string, request []byte) []byte {
	switch method {
	case pluginabi.MethodPluginRegister, pluginabi.MethodPluginReconfigure:
		return r.handleRegister(method, request)
	case pluginabi.MethodPluginShutdown:
		return r.handleShutdown()
	case pluginabi.MethodPluginQuiesce:
		return okEmpty()
	case pluginabi.MethodExecutorIdentifier:
		return okJSON(identifierResponse{Identifier: r.role.Provider})
	case pluginabi.MethodExecutorExecute:
		return r.handleExecute(request)
	case pluginabi.MethodExecutorExecuteStream:
		return r.handleExecuteStream(request)
	case pluginabi.MethodExecutorCountTokens:
		return abi.Fail("unsupported", "OpenCode Go 插件不支持 token 计数", http.StatusNotImplemented)
	case pluginabi.MethodExecutorHTTPRequest:
		return abi.Fail("unsupported", "OpenCode Go 插件不支持 HTTP 桥接", http.StatusNotImplemented)
	case pluginabi.MethodModelRegister, pluginabi.MethodModelStatic, pluginabi.MethodModelForAuth:
		return r.handleStaticModels(request)
	case pluginabi.MethodModelRoute:
		return r.handleModelRoute(request)
	case pluginabi.MethodManagementRegister:
		return r.handleManagementRegister(request)
	case pluginabi.MethodManagementHandle:
		return r.handleManagementHandle(request)
	default:
		return abi.Fail("unknown_method", "unknown method: "+method, http.StatusNotFound)
	}
}

type lifecycleRequest struct {
	ConfigYAML    []byte `json:"config_yaml"`
	SchemaVersion uint32 `json:"schema_version"`
}

type registrationCapabilities struct {
	ModelProvider         bool     `json:"model_provider"`
	ModelRouter           bool     `json:"model_router"`
	Executor              bool     `json:"executor"`
	ExecutorModelScope    string   `json:"executor_model_scope"`
	ExecutorInputFormats  []string `json:"executor_input_formats,omitempty"`
	ExecutorOutputFormats []string `json:"executor_output_formats,omitempty"`
	ManagementAPI         bool     `json:"management_api"`
}

type registration struct {
	SchemaVersion uint32                   `json:"schema_version"`
	Metadata      pluginapi.Metadata       `json:"metadata"`
	Capabilities  registrationCapabilities `json:"capabilities"`
}

type identifierResponse struct {
	Identifier string `json:"identifier"`
}

type modelRouteResponse struct {
	Handled     bool   `json:"Handled"`
	TargetKind  string `json:"TargetKind,omitempty"`
	Target      string `json:"Target,omitempty"`
	TargetModel string `json:"TargetModel,omitempty"`
	Reason      string `json:"Reason,omitempty"`
}

func (r *Runtime) handleRegister(method string, request []byte) []byte {
	var lifecycle lifecycleRequest
	if len(request) > 0 {
		if errUnmarshal := json.Unmarshal(request, &lifecycle); errUnmarshal != nil {
			return abi.Fail("invalid_config", fmt.Sprintf("解析插件配置失败: %v", errUnmarshal), http.StatusBadRequest)
		}
	}

	settings, errParse := config.Parse(lifecycle.ConfigYAML)

	r.mu.Lock()
	r.configRaw = append([]byte(nil), lifecycle.ConfigYAML...)
	if r.sharedPath == "" {
		r.sharedPath = config.Default().StatePath(r.root)
	}
	sharedPath := r.sharedPath
	isMain := r.role.Name == roles.Chat
	r.mu.Unlock()

	if isMain {
		// The chat-role build owns the configuration and publishes the shared
		// runtime snapshot every sibling reads.
		if errParse != nil {
			r.setLastError(errParse.Error())
		} else {
			r.setLastError("")
			if errWrite := config.WriteSharedState(sharedPath, config.SharedState{
				Version:   r.Version(),
				WrittenAt: time.Now().UTC(),
				Settings:  settings,
			}); errWrite != nil {
				r.setLastError("写入共享配置失败: " + errWrite.Error())
			}
		}
	} else {
		// Sibling builds prefer the shared snapshot and fall back to their own
		// configuration block when it is not published yet.
		if shared, errLoad := config.ReadSharedState(sharedPath); errLoad == nil {
			settings = shared.Settings
			if errParse != nil {
				settings = shared.Settings
			}
			r.setLastError("")
		} else if errParse != nil {
			// Without a snapshot a parse failure is fatal for this instance.
			return abi.Fail("invalid_config", errParse.Error(), http.StatusBadRequest)
		} else {
			r.setLastError("尚未读取到主插件发布的共享配置（cap-opencode-go）")
		}
		r.mu.Lock()
		r.sharedStamp = stampOf(sharedPath)
		r.mu.Unlock()
	}

	r.applySettings(settings)
	_ = method
	return r.registrationPayload()
}

func (r *Runtime) registrationPayload() []byte {
	metadata := pluginapi.Metadata{
		Name:             pluginName,
		Version:          r.Version(),
		Author:           pluginAuthor,
		GitHubRepository: pluginRepository,
		ConfigFields:     configFields(),
	}
	capabilities := registrationCapabilities{
		ModelProvider:         true,
		ModelRouter:           true,
		Executor:              true,
		ExecutorModelScope:    string(pluginapi.ExecutorModelScopeBoth),
		ExecutorInputFormats:  r.role.Formats,
		ExecutorOutputFormats: r.role.Formats,
	}
	if r.role.Name == roles.Chat {
		capabilities.ManagementAPI = true
	}
	raw, errMarshal := abi.OK(registration{
		SchemaVersion: pluginabi.SchemaVersion,
		Metadata:      metadata,
		Capabilities:  capabilities,
	})
	if errMarshal != nil {
		return abi.Fail("marshal_error", errMarshal.Error(), http.StatusInternalServerError)
	}
	return raw
}

func (r *Runtime) handleShutdown() []byte {
	return okEmpty()
}

func (r *Runtime) applySettings(settings config.Settings) {
	settings.HostProxyURL = r.hostProxyURL()
	copied := settings
	r.settings.Store(&copied)
	r.resolver.Store(catalog.New(copied))
	r.client.Update(copied)
	r.debugMode.Store(settings.Debug)
	r.mu.Lock()
	r.debugPath = filepath.Join(settings.StateRoot(r.root), "debug.log")
	r.mu.Unlock()
}

func (r *Runtime) setLastError(message string) {
	r.mu.Lock()
	r.lastError = message
	r.mu.Unlock()
}

// LastError reports the most recent configuration problem.
func (r *Runtime) LastError() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastError
}

// refreshShared reloads the shared snapshot for sibling builds when the main
// plugin republished it. The check is an os.Stat, so it is cheap enough to run
// per request.
func (r *Runtime) refreshShared() {
	if r.role.Name == roles.Chat {
		return
	}
	r.mu.Lock()
	path := r.sharedPath
	stamp := r.sharedStamp
	r.mu.Unlock()
	if path == "" {
		return
	}
	current := stampOf(path)
	if current.IsZero() || current.Equal(stamp) {
		return
	}
	shared, errLoad := config.ReadSharedState(path)
	if errLoad != nil {
		return
	}
	r.mu.Lock()
	r.sharedStamp = current
	r.mu.Unlock()
	r.applySettings(shared.Settings)
	r.setLastError("")
}

func stampOf(path string) time.Time {
	if path == "" {
		return time.Time{}
	}
	info, errStat := os.Stat(path)
	if errStat != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// hostProxyURL returns the upstream proxy configured on the host, which is used
// when a model inherits proxy behaviour and the plugin has no proxy-url.
func (r *Runtime) hostProxyURL() string {
	if current := r.hostProxy.Load(); current != nil {
		return *current
	}
	return ""
}

func (r *Runtime) rememberHostProxy(summary pluginapi.HostConfigSummary) {
	value := strings.TrimSpace(summary.ProxyURL)
	copied := value
	r.hostProxy.Store(&copied)
}

func (r *Runtime) models() []pluginapi.ModelInfo {
	resolver := r.Resolver()
	entries := resolver.EntriesForFamily(r.role.Name)
	out := make([]pluginapi.ModelInfo, 0, len(entries))
	for _, entry := range entries {
		out = append(out, pluginapi.ModelInfo{
			ID:                         entry.ID,
			Object:                     "model",
			Created:                    r.startedAt.Unix(),
			OwnedBy:                    "opencode",
			DisplayName:                entry.DisplayName,
			Name:                       entry.Upstream,
			Description:                entry.Note,
			SupportedGenerationMethods: []string{"chat"},
		})
	}
	return out
}

func (r *Runtime) handleStaticModels(request []byte) []byte {
	r.refreshShared()
	if len(request) > 0 {
		var staticRequest pluginapi.StaticModelRequest
		if errUnmarshal := json.Unmarshal(request, &staticRequest); errUnmarshal == nil {
			previous := r.hostProxyURL()
			r.rememberHostProxy(staticRequest.Host)
			if r.hostProxyURL() != previous {
				// Push the learned proxy into the upstream client's settings so
				// inherited proxy models stop going out direct.
				r.applySettings(r.Settings())
			}
		}
	}
	raw, errMarshal := abi.OK(pluginapi.ModelResponse{
		Provider: r.role.Provider,
		Models:   r.models(),
	})
	if errMarshal != nil {
		return abi.Fail("marshal_error", errMarshal.Error(), http.StatusInternalServerError)
	}
	return raw
}

func (r *Runtime) handleModelRoute(request []byte) []byte {
	r.refreshShared()
	var routeRequest pluginapi.ModelRouteRequest
	if len(request) > 0 {
		if errUnmarshal := json.Unmarshal(request, &routeRequest); errUnmarshal != nil {
			return abi.Fail("invalid_request", errUnmarshal.Error(), http.StatusBadRequest)
		}
	}
	requested := strings.TrimSpace(routeRequest.RequestedModel)
	if requested == "" || !r.claims(requested) {
		raw, errMarshal := abi.OK(modelRouteResponse{Handled: false})
		if errMarshal != nil {
			return abi.Fail("marshal_error", errMarshal.Error(), http.StatusInternalServerError)
		}
		return raw
	}
	raw, errMarshal := abi.OK(modelRouteResponse{
		Handled:    true,
		TargetKind: "self",
		Reason:     "opencode-go/" + string(r.role.Name),
	})
	if errMarshal != nil {
		return abi.Fail("marshal_error", errMarshal.Error(), http.StatusInternalServerError)
	}
	return raw
}

// claims reports whether this provider owns the requested model id.
func (r *Runtime) claims(model string) bool {
	entry, ok := r.Resolver().Resolve(model)
	if !ok || !entry.Enabled {
		return false
	}
	return entry.Family == r.role.Name
}

func okJSON(v any) []byte {
	raw, errMarshal := abi.OK(v)
	if errMarshal != nil {
		return abi.Fail("marshal_error", errMarshal.Error(), http.StatusInternalServerError)
	}
	return raw
}

func okEmpty() []byte {
	raw, errMarshal := abi.OK(map[string]any{})
	if errMarshal != nil {
		return abi.Fail("marshal_error", errMarshal.Error(), http.StatusInternalServerError)
	}
	return raw
}
