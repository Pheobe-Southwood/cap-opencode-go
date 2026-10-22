package plugin

import (
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
)

// configFields describes the plugin configuration for the host management UI.
// The values live under plugins.configs.<pluginID> and are delivered to the
// plugin as YAML on every register and reconfigure.
//
// Only the main build declares them. The three sibling builds read the snapshot
// the main build publishes (data/cpa-opencodego/shared.json) and their own
// config block carries nothing but the enable flag, so advertising the fields
// there would invite the operator to fill in a form that has no effect. The
// fields stay documented in one place: plugins.configs.cpa-opencodego.
func configFields(role roles.Role) []pluginapi.ConfigField {
	if role.Name != roles.Chat {
		return nil
	}
	return []pluginapi.ConfigField{
		{Name: "base-url", Type: pluginapi.ConfigFieldTypeString, Description: "OpenCode Go 接口基址。默认 https://opencode.ai/inference/go（新分族布局）；以 /v1 结尾的旧形式（如 .../inference/go/v1、.../zen/go/v1）继续可用。"},
		{Name: "endpoint-layout", Type: pluginapi.ConfigFieldTypeEnum, EnumValues: []string{"auto", "legacy", "family"}, Description: "端点布局：auto 按 base-url 自动判断，legacy 旧单根 /v1，family 新分族 /openai、/anthropic、/google。"},
		{Name: "base-urls", Type: pluginapi.ConfigFieldTypeObject, Description: "逐协议面覆盖 API 根：{chat, responses, messages, gemini}，设置后该族路径直接追加到这个根。"},
		{Name: "developer-role", Type: pluginapi.ConfigFieldTypeEnum, EnumValues: []string{"retry-only", "rewrite", "keep"}, Description: "OpenAI 形状请求里 developer 角色的处理：retry-only（默认，被上游 400 拒绝时改写并重试）、rewrite（主动改写为 system）或 keep（原样透传不重试）。"},
		{Name: "reasoning-replay", Type: pluginapi.ConfigFieldTypeEnum, EnumValues: []string{"retry-only", "auto", "always", "off"}, Description: "思考模式 reasoning_content 回放：retry-only（默认，被 400 拒绝时补齐占位符并重试，并记住会话 sticky）、auto（仅思考请求提前补齐）、always（所有 chat 提前补齐）或 off（原样透传）。"},
		{Name: "request-log", Type: pluginapi.ConfigFieldTypeBoolean, Description: "是否记录逐条请求日志（时间、token、TTFT、生成/端到端 TPS、状态）；默认开启，每面保留 2000 条 / 7 天。"},
		{Name: "keys", Type: pluginapi.ConfigFieldTypeArray, Description: "Key 列表：[{name, key, priority, enabled}]。插件会把它们投影成 CPA 认证文件（每个协议面一个），由 CPA AuthManager 按 priority 与宿主 routing 策略选择、轮询、冷却并自动故障转移。"},
		{Name: "models", Type: pluginapi.ConfigFieldTypeObject, Description: "逐模型设置：{\"<model-id>\": {enabled, proxy, family, aliases}}，proxy 取 inherit/proxy/direct。aliases 会替代原名对外提供（多个别名 = 同一模型的多个对外名）。"},
		{Name: "models-default", Type: pluginapi.ConfigFieldTypeObject, Description: "未单独列出的模型使用的默认设置。"},
		{Name: "default-proxy", Type: pluginapi.ConfigFieldTypeEnum, EnumValues: []string{"inherit", "proxy", "direct"}, Description: "模型未指定代理时的默认策略。"},
		{Name: "proxy-url", Type: pluginapi.ConfigFieldTypeString, Description: "插件使用的代理地址；留空则继承 CPA 的全局 proxy-url。"},
		{Name: "session-mode", Type: pluginapi.ConfigFieldTypeEnum, EnumValues: []string{"auto", "fixed"}, Description: "x-opencode-session 取值方式：auto 跟随 CPA 会话，fixed 使用固定值。"},
		{Name: "session", Type: pluginapi.ConfigFieldTypeString, Description: "session-mode=fixed 时发送的 x-opencode-session 值。"},
		{Name: "request-timeout-seconds", Type: pluginapi.ConfigFieldTypeInteger, Description: "上游请求超时秒数，默认 900。"},
		{Name: "extra-headers", Type: pluginapi.ConfigFieldTypeObject, Description: "附加到上游请求的额外请求头。"},
		{Name: "debug", Type: pluginapi.ConfigFieldTypeBoolean, Description: "开启后把流式与上游诊断写入 <状态目录>/debug.log。"},
		{Name: "state-dir", Type: pluginapi.ConfigFieldTypeString, Description: "插件自有状态目录，留空为 <CPA 根>/data/cpa-opencodego；升级时会从 v1 目录一次性复制统计。"},
		{Name: "plan", Type: pluginapi.ConfigFieldTypeEnum, EnumValues: []string{"go", "go-plus"}, Description: "OpenCode Go 套餐：go（$10/月）或 go-plus（$40/月），用于统计 Tab 的逐模型月额度百分比。"},
	}
}
