# CLIProxyAPI OpenCode Go 插件 — 技术路线与实现文档

本文档收录 `cpa-opencodego` 插件的架构原理、协议转换机制、技术构建流程、全量配置参数参考与已知技术细节。日常使用与功能特性请参阅 [README.md](../README.md)。

---

## 1. 架构：为什么是 4 个 .so

CLIProxyAPI (CPA) 的插件执行器（Executor）基于**声明式协议格式**工作：插件在注册时声明自身只接受并处理某一种线上格式（`executor_input_formats` / `executor_output_formats`），由 CPA 宿主负责将客户端传入的各种协议请求翻译为此格式，并将插件回传的数据翻译回客户端所需格式。

OpenCode Go 的 37 个模型分布在 4 个完全不同的上游端点上：

| 上游端点（新分族布局） | 旧单根布局（v1，继续兼容） | 协议格式 |
|---|---|---|
| `POST {go}/openai/v1/chat/completions` | `POST {base}/chat/completions` | `openai` |
| `POST {go}/openai/v1/responses` | `POST {base}/responses` | `codex` + `openai-response` |
| `POST {go}/anthropic/v1/messages` | `POST {base}/messages` | `claude` |
| `POST {go}/google/v1beta/models/{id}:generateContent` | `POST {base}/models/{id}:generateContent` | `gemini` |
| `GET {go}/v1/models`、`GET {go}/v1/usage` | `GET {base}/models`、`GET {base}/usage` | 列表 / 额度 |

`{go}` 默认是 `https://opencode.ai/inference/go`；`{base}` 是旧值如 `https://opencode.ai/inference/go/v1` 或 `https://opencode.ai/zen/go/v1`。布局由 `endpoint-layout`（默认 `auto`，按 base-url 是否以 `/v1` 结尾判断）决定，可用 `base-urls` 逐族覆盖。

由于格式声明是 Provider 级别的，而具体模型所需的协议是模型级别的，单个 Provider 实例无法在不手写全套转换器的情况下承载多种上游协议。

因此，同一套 Go 代码通过不同的链接期编译常量（`-ldflags -X main.role=...`）编译出 4 个动态链接库（`.so`），分别对应 4 个协议面：

| 插件 ID | Provider Key | 声明格式 | 对应上游端点 |
|---|---|---|---|
| `cpa-opencodego` | `opencode-go` | `["openai"]` | `/chat/completions` |
| `cpa-opencodego-responses` | `opencode-go-responses` | `["codex","openai-response"]` | `/responses` |
| `cpa-opencodego-messages` | `opencode-go-messages` | `["claude"]` | `/messages` |
| `cpa-opencodego-gemini` | `opencode-go-gemini` | `["gemini"]` | `models/{id}:generateContent` |

### 统一控制与配置流

- **主实例（`cpa-opencodego`）**：唯一挂载 `management_api` 与侧边栏菜单的面，也是唯一的配置入口。它把解析后的配置写入 `<CPA 根>/data/cpa-opencodego/shared.json`。
- **从属实例**：其余 3 个从属面以只读模式监听 `shared.json` 的 mtime 变化进行惰性加载。从属面在宿主配置中仅需 `enabled: true`，不暴露重复冗余的配置字段。
- **模型路由抢占**：4 个面均注册各自的 `model_router`，按模型名单精准认领。由于设置了专属路由，可以稳定接管同名模型请求。

数据流向：

```
客户端(任意协议) → CPA 宿主(翻译为声明格式) → 插件(路由/选 Key/选网络通道) → 原样 POST 上游
                 → 上游流式返回 → CPA 宿主(翻译回客户端协议) → 客户端
```

插件自身不改写 JSON 协议体业务内容（仅对齐 `stream` 传输模式）。

---

## 2. 统计监控与延迟口径实现

主面 `cpa-opencodego` 声明了 `usage_plugin` 能力，接收宿主在请求完成后投递的 Usage 事件。Token 消耗、缓存命中有无、失败状态与总延迟均由宿主统一解析。

由于宿主对异步流桥接不提供内部 TTFT（`ttft_ns=0`），4 个协议面各自在读取上游 SSE 数据流时自行测量首个内容帧的时间戳，写入 `<状态目录>/stats-<role>.json`。主面的 `/api/stats` 接口自动汇聚 4 个协议面的耗时与用量。

### 指标口径

- **整体 TPS**：成功请求的有效 output tokens ÷ 宿主请求总耗时（包含首字、排队与全链路网络延迟）。
- **生成 TPS**：插件实测有效 output tokens ÷（最后一个内容帧时间 - 首个内容帧时间），不包含 TTFT，贴近纯解码速率。
- **TTFT**：向上游发起请求到接收到首个真实有效内容帧的时间差。空流、错误帧或提前中断不计入样本。
- **刊例价与额度**：内置官方价目快照折算，支持根据 `plan`（`go` / `go-plus`）自动计算各模型当月额度消耗百分比。

### 请求日志（v2.0）

`/api/requests` 合并四个协议面各自的 `requests-<role>.jsonl`，返回逐条记录：时间、模型、协议面、Key、输入/输出/合计 token（含缓存与推理明细）、TTFT、生成 TPS、端到端 TPS、状态、错误摘要、上游 `x-opencode-log-id` 与 8 位会话哈希。

- **数据来源**：executor 在转发时自行埋点并解析上游响应帧的 usage（chat 的 `usage`、messages 的 `message_start`/`message_delta`、responses 的 `response.completed`、gemini 的 `usageMetadata`），不依赖宿主 usage 事件，也不需要请求 id 关联。
- **口径**：生成 TPS = 有效输出 ÷（首内容帧 → 末内容帧）；端到端 TPS = 有效输出 ÷ 总耗时；TTFT 只统计有内容且正常结束的流；失败请求不计算 TPS。
- **保留**：每面最多 2000 条 / 7 天，文件超过 4 MiB 自动压缩；可在设置里关闭。
- **隐私**：不记录提示词、响应正文、Key 密文或原始会话 ID。

### 思考模式 reasoning_content 补齐（v2.0.1）

DeepSeek 思考模式要求把上一轮 assistant 消息携带的 `reasoning_content` 原样回传，包含同时带
`tool_calls` 的消息。DSH 只保留最近一两条（本次 turn 的思维链），更早的历史会丢弃；当请求变长、
且 OpenCode Go 命中严格校验的网关节点时，上游返回：

```
The `reasoning_content` in the thinking mode must be passed back to the API.
```

插件在发送前做两件事，均不改变模型能力与 DSH 的调用方式：

1. **补齐字段**：`reasoning-replay=auto`（默认）在检测到思考请求（`reasoning_effort` /
   `reasoning` / `thinking` / `enable_thinking`，或历史里已有一条非空 `reasoning_content`）时，
   为所有缺少该字段的 assistant 消息写入中性占位文本 `[reasoning unavailable]`。已有内容原样
   保留；空字符串会被替换（proto3 前端无法区分空串与缺省）。`always` 对所有 chat 请求生效，
   `off` 完全透传。
2. **单次重试**：同一份请求在部分节点上会偶发该 400（相同字节重放通常成功），插件对
   “`must be passed back` + `reasoning_content`/`reasoning_text`” 的 400 自动重试一次，其他 400
   立即返回。

补齐用占位文本而不是回放旧思维链：把历史 CoT 原样送回会让长会话模型模仿旧的工具调用格式，
反而造成工具调用丢失（见上游 issue 记录），而校验只要求字段存在。该改写只影响 chat 协议的
请求体，messages/responses/gemini 三族不变；请求日志仍只记录元数据。

---

## 3. 环境要求与构建

### 环境要求

- **运行宿主**：CLIProxyAPI v8.x，且编译时开启 `CGO_ENABLED=1`（具备 `X-CPA-SUPPORT-PLUGIN: 1` 标识）。
- **构建环境**：Go 1.26+（经 go1.27.2 实测通过）。
- **插件 ABI**：采用标准 `dlopen` + C ABI（`abi_version = 1`），不要求与 CPA 宿主使用完全相同的编译器工具链或依赖版本。

### 编译命令

```bash
export PATH=/opt/go/bin:$PATH
export GOPROXY=https://goproxy.cn,direct GOSUMDB=off

# 编译 4 个协议面 .so
make build VERSION=2.0.1

# 执行单元测试与代码检查
make test
make vet
```

编译产出位于 `dist/` 目录：
- `dist/cpa-opencodego-v2.0.1.so`
- `dist/cpa-opencodego-responses-v2.0.1.so`
- `dist/cpa-opencodego-messages-v2.0.1.so`
- `dist/cpa-opencodego-gemini-v2.0.1.so`

---

## 4. 全量配置参数参考

所有核心配置集中在宿主 `config.yaml` 的 `plugins.configs.cpa-opencodego` 节点下：

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    cpa-opencodego:
      enabled: true
      base-url: "https://opencode.ai/inference/go"
      endpoint-layout: "auto"
      developer-role: "rewrite"
      reasoning-replay: "auto"
      request-log: true
      session-mode: "auto"
      session: "$CPA-SESSION-ID"
      default-proxy: "inherit"
      proxy-url: ""
      request-timeout-seconds: 900
      plan: "go"
      debug: false
      keys:
        - name: "primary-key"
          key: "oc_sk_xxx"
          priority: 10
          enabled: true
        - name: "backup-key"
          key: "oc_sk_yyy"
          priority: 5
          enabled: true
      models:
        "deepseek-v4.1-flash":
          enabled: true
          proxy: "proxy"
          aliases: ["DeepSeek-V4.1-Flash"]
        "gemini-4-argon-contributor":
          enabled: true
          proxy: "direct"
          aliases: ["gemini-4-argon"]
    cpa-opencodego-responses: { enabled: true }
    cpa-opencodego-messages:  { enabled: true }
    cpa-opencodego-gemini:    { enabled: true }
```

### 字段说明表

| 字段 | 类型 | 默认值 | 作用与说明 |
|---|---|---|---|
| `base-url` | string | `https://opencode.ai/inference/go` | Go 根（新分族布局）；旧值 `.../inference/go/v1`、`.../zen/go/v1` 仍兼容并按 legacy 布局请求 |
| `endpoint-layout` | enum | `auto` | `auto` 按 base-url 判断、`legacy` 旧单根、`family` 新分族 |
| `base-urls` | map | `{}` | 逐族 API 根覆盖：`{chat, responses, messages, gemini}`，设置的族不再追加前缀 |
| `developer-role` | enum | `rewrite` | OpenAI 形状请求的 `developer` → `system` 兼容改写；`keep` 关闭 |
| `reasoning-replay` | enum | `auto` | 思考模式历史 `reasoning_content` 补齐：`auto` 只在思考请求上补齐，`always` 所有 chat 请求补齐，`off` 不改写 |
| `request-log` | bool | `true` | 是否写逐条请求日志（每面 2000 条 / 7 天） |
| `keys` | array | `[]` | 多账号 Key 列表，包含 `name`、`key`、`priority`、`enabled` |
| `models` | map | `{}` | 逐模型个性化配置（启用状态、代理策略、别名、协议面覆盖） |
| `models-default` | object | `{}` | 未显式声明模型的默认基线行为 |
| `default-proxy` | enum | `inherit` | 全局默认代理模式：`inherit` / `proxy` / `direct` |
| `proxy-url` | string | `""` | 插件专用出站代理地址；留空时自动继承宿主 `requests.proxy-url` |
| `session-mode` | enum | `auto` | 会话传递模式：`auto` 继承 CPA 会话 ID；`fixed` 使用固定值 |
| `session` | string | `"$CPA-SESSION-ID"` | 当 `session-mode=fixed` 时作为 `x-opencode-session` 报头值 |
| `request-timeout-seconds` | int | `900` | 上游单次请求最大超时时长（秒） |
| `plan` | enum | `go` | 订阅套餐类型：`go`（$10/月）或 `go-plus`（$40/月），决定月额度参考线 |
| `state-dir` | string | `<root>/data/cpa-opencodego` | 插件本地数据目录，存放 `shared.json`、各协议面统计文件与 `requests-<role>.jsonl`；首次启动会把 v1 目录的统计复制过来 |
| `debug` | bool | `false` | 是否开启调试模式，开启后输出详细追踪至 `<state-dir>/debug.log` |

### 代理生效链

请求代理按照如下优先级严格判定：
1. **模型级代理 (`models.<id>.proxy`)**：显式设定为 `direct`（直连）或 `proxy`（代理）。
2. **默认代理 (`default-proxy`)**：当模型设置为 `inherit` 时生效。
3. **插件级代理 (`proxy-url`)**：当上述判定需要走代理时优先使用的代理地址。
4. **宿主全局代理 (`requests.proxy-url`)**：当插件未设置专用代理时继承宿主配置。

---

## 5. 卸载与零残留设计

插件严格遵守零入侵、零残留原则：
- 不修改 CPA 源码与二进制；
- 不向宿主 SQLite/bbolt 写入任何脏数据；
- 不污染 CPA 宿主系统配置。

### 卸载步骤

1. 停止 CPA 服务或在运行中卸载：
   ```bash
   bash scripts/uninstall.sh
   ```
2. 从 `config.yaml` 的 `plugins.configs` 列表中移除 `cpa-opencodego` 及其三个从属协议面配置块。
3. 重启 CPA 宿主即可彻底恢复纯净状态。

---

## 6. 历史数据回放工具

针对先前使用 `cap-token-usage-tracker` 的环境，仓库附带 `cmd/replay-tracker` 工具，支持无损读取旧 bbolt 数据库，按最新价目表重算并导入历史调用记录：

```bash
# 停止 CPA 避免写冲突
systemctl stop cliproxyapi

# 编译并执行回放
make VERSION=2.0.1 replay
./dist/replay-tracker-v2.0.1 \
  -tracker /root/cliproxyapi/data/token-usage-tracker.db \
  -stats-dir /root/cliproxyapi/data/cpa-opencodego -apply

# 恢复服务
systemctl start cliproxyapi
```
