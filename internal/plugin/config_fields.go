package plugin

import (
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"

	"github.com/cpa-opencodego/cap-opencode-go/internal/roles"
)

// configFields describes the plugin configuration for the host management UI.
// The values live under plugins.configs.<pluginID> and are delivered to the
// plugin as YAML on every register and reconfigure.
//
// Only the main build declares them. The three sibling builds read the snapshot
// the main build publishes (data/cap-opencode-go/shared.json) and their own
// config block carries nothing but the enable flag, so advertising the fields
// there would invite the operator to fill in a form that has no effect. The
// fields stay documented in one place: plugins.configs.cap-opencode-go.
func configFields(role roles.Role) []pluginapi.ConfigField {
	if role.Name != roles.Chat {
		return nil
	}
	return []pluginapi.ConfigField{
		{Name: "base-url", Type: pluginapi.ConfigFieldTypeString, Description: "OpenCode Go 接口基址，默认 https://opencode.ai/inference/go/v1。"},
		{Name: "keys", Type: pluginapi.ConfigFieldTypeArray, Description: "多账号 Key 列表：[{name, key, priority, enabled}]，路由选择与容灾交由 CPA 宿主调度。"},
		{Name: "models", Type: pluginapi.ConfigFieldTypeObject, Description: "逐模型设置：{\"<model-id>\": {enabled, proxy, family, aliases}}，proxy 取 inherit/proxy/direct。aliases 会替代原名对外提供（多个别名 = 同一模型的多个对外名）。"},
		{Name: "models-default", Type: pluginapi.ConfigFieldTypeObject, Description: "未单独列出的模型使用的默认设置。"},
		{Name: "default-proxy", Type: pluginapi.ConfigFieldTypeEnum, EnumValues: []string{"inherit", "proxy", "direct"}, Description: "模型未指定代理时的默认策略。"},
		{Name: "proxy-url", Type: pluginapi.ConfigFieldTypeString, Description: "插件使用的代理地址；留空则继承 CPA 的全局 proxy-url。"},
		{Name: "session-mode", Type: pluginapi.ConfigFieldTypeEnum, EnumValues: []string{"auto", "fixed"}, Description: "x-opencode-session 取值方式：auto 跟随 CPA 会话，fixed 使用固定值。"},
		{Name: "session", Type: pluginapi.ConfigFieldTypeString, Description: "session-mode=fixed 时发送的 x-opencode-session 值。"},
		{Name: "request-timeout-seconds", Type: pluginapi.ConfigFieldTypeInteger, Description: "上游请求超时秒数，默认 900。"},
		{Name: "extra-headers", Type: pluginapi.ConfigFieldTypeObject, Description: "附加到上游请求的额外请求头。"},
		{Name: "debug", Type: pluginapi.ConfigFieldTypeBoolean, Description: "开启后把流式与上游诊断写入 <状态目录>/debug.log。"},
		{Name: "state-dir", Type: pluginapi.ConfigFieldTypeString, Description: "插件自有状态目录，留空为 <CPA 根>/data/cap-opencode-go。"},
		{Name: "plan", Type: pluginapi.ConfigFieldTypeEnum, EnumValues: []string{"go", "go-plus"}, Description: "OpenCode Go 套餐：go（$10/月）或 go-plus（$40/月），用于统计 Tab 的逐模型月额度百分比。"},
		{Name: "register-accounts", Type: pluginapi.ConfigFieldTypeBoolean, Description: "把 keys 投影成 CPA 认证文件，使每个 Key 在 CPA 的「配额管理 → Plugin」里显示为一个账号；默认开启，插件卸载时自动删除。"},
	}
}
