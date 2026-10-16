# 设计说明

## 1. 为什么插件本身不做协议转换

CPA 的插件 executor 采用声明式协议格式：

- `executor_input_formats`：插件能**接收**的请求格式（CPA 会把客户端请求翻译成其中之一）
- `executor_output_formats`：插件能**产出**的响应格式（CPA 会把它翻译回客户端协议）

选择逻辑（`internal/pluginhost/adapters_executors.go`）：

1. 客户端格式若在 `input_formats` 中 → 原样传入，不翻译
2. 否则取第一个存在 `HasRequestTransformer(客户端 → 声明格式)` 的声明格式进行翻译
3. 输出同理：客户端格式在 `output_formats` 中就直接返回；否则用插件格式产出后由 CPA 翻译回去

内置翻译矩阵覆盖 openai / openai-response / codex / claude / gemini / antigravity / interactions
的所有方向，因此"插件只声明上游协议、转换全部交给 CPA"是可行的，且**零转换代码**。

## 2. 为什么是四个实例而不是一个

格式声明是 **per-provider** 的，模型所需协议是 **per-model** 的。
OpenCode Go 的 37 个模型分布在 4 个上游端点上，若用一个 provider 承载：

- 声明 `["openai"]` → responses / messages / gemini 三个家族的请求会被送错端点
- 声明全部四种 → 宿主优先选择"客户端格式"直通，模型需要的上游协议依旧无法表达

唯一能让宿主替我们做转换的办法就是**一个协议族一个 provider**，于是同一份代码编译四次，
仅 `-ldflags -X main.role=…` 不同。代价是插件列表里多三行、配置里多三个 `enabled: true`，
换来的收益是插件里没有一行协议转换逻辑。

## 3. 共享配置

宿主只把 `plugins.configs.<pluginID>` 交给对应实例，实例之间无法互相读取配置。
因此：

- `cap-opencode-go`（chat 角色）是唯一配置源
- 它在每次 `plugin.register` / `plugin.reconfigure` 时把解析后的配置写入
  `<root>/data/cap-opencode-go/shared.json`（原子写：临时文件 + rename）
- 其余三个实例在每次 RPC 前 `stat` 该文件，mtime 变化才重新加载

选择文件而不是复制三份配置，是为了让 Key 只存一处、代理策略只有一份真相。

## 4. 逐模型代理

`HostHTTPClient`（CPA 提供的宿主 HTTP 桥）只接受 `WireProfile`，没有 per-request 代理字段，
宿主全局 `requests.proxy-url` 对所有调用一致。要实现"每个模型单独决定是否走代理"，
插件必须自己持有 transport：

- `proxy` → `Transport.Proxy = ProxyURL(解析后的代理)`
- `direct` → `Transport.Proxy = nil`（**显式 nil**，不受 `http_proxy` 等环境变量影响）
- `inherit` → 插件 `proxy-url` → CPA `requests.proxy-url` → 无

transport 按代理 URL 缓存，连接池可复用。代价是这些出站请求不进 CPA 的出站请求日志；
token 用量统计不受影响，因为宿主仍会解析插件返回的 payload 并投递给用量插件。

宿主代理 URL 来自 `model.static` 请求里的 `HostConfigSummary.ProxyURL`。该回调发生在
`plugin.register` **之后**，因此运行时读取时必须把这份值覆盖到当前配置上，
否则重启后第一批请求会误走直连（实测表现为 responses 家族 403）。

## 5. 流式

C ABI 下 `ExecutorStreamResponse.chunks` 是全缓冲的 JSON 数组，用它做流式会牺牲首包延迟。
正确做法：`executor.execute_stream` 立刻返回响应头、不带 chunks，然后用请求携带的
`stream_id` 通过 `host.stream.emit` 逐块推送，最后 `host.stream.close`。

分块形状按协议族区分（对照内置 executor 的约定）：

| 角色 | 形状 | 原因 |
|---|---|---|
| chat / gemini | 裸 JSON（去掉 `data: ` 前缀） | 处理器自己写 `data: ` 前缀；内置 gemini executor 也用 `helps.JSONPayload` 去前缀 |
| responses / messages | 完整 SSE 帧 | 内置 codex executor 会把事件重新拼成 `data: {...}` 再交给翻译器；Claude 处理器逐帧原样转发 |

上游的 `[DONE]` 会被丢弃，由宿主补自己的终止标记。

## 5.5 非流式的 codex 终止事件

responses 协议族声明了 `["codex","openai-response"]`：

- 客户端本身用 `openai-response`（`/v1/responses`）时，宿主走直通分支，插件返回上游原始 body 即可
- 其它客户端（chat / claude / gemini）时，宿主选定的输出格式是 `codex`，随后用
  `ConvertCodexResponseToOpenAI{,NonStream}` / `ConvertCodexResponseToClaudeNonStream` 翻译

这些**非流式**翻译器的第一行就是：

```go
if typeStr != "response.completed" && typeStr != "response.incomplete" { return []byte{} }
```

也就是说它们期望收到的是**终止 SSE 事件**而不是完整响应对象。插件因此在
`Format == "codex"` 的非流式返回里把上游 body 包成
`{"type":"response.completed","response":<body>}`（`status: incomplete` 映射为
`response.incomplete`）。流式路径不需要处理：上游 SSE 帧本身就带 `type`。

## 5.6 模型列表、协议归属与别名

- 内置目录（37 项）给出「模型 → 协议族」的权威映射，依据官方文档 + 实测
- 运行时通过 `GET {base}/models` 拉取上游真实清单（缓存 5 分钟），与本地配置合并后渲染。
  上游接口只返回 `id/object/created/owned_by`，**没有任何协议信息**
- 协议族解析顺序：`models.<id>.family`（配置显式指定）→ 内置目录 → **ID 前缀推断**
  （`claude-*` → messages，`gemini-*` → gemini，`gpt-*` / `grok-*` / `muse-*` → responses，其余 chat）。
  `models.FamilyHint` 与前缀表放在同一处，`internal/models/catalog_test.go` 断言它对全部 37 个
  内置条目与目录完全一致，避免两张表悄悄分叉
- `Entry.FamilySource`（`config` / `catalogue` / `inferred`）随 `/api/models` 下发，
  仪表盘对 `inferred` 标注「协议推测」；`family` 写错（不是四个合法值之一）时在配置归一化阶段丢弃，
  回落到目录/推断，而不是让模型无人认领
- 拿不准的新模型可以用「探测协议」（`GET /v0/management/opencode-go/probe?model=<id>`）实测：
  插件按「当前生效的协议族优先」的顺序，对四个端点各发一次最小请求（chat/responses/messages/gemini
  各自的 body 形状、路径与鉴权头），返回 `{model, best, results[]}`。该路由是**宿主管理鉴权**的，
  因此它既不会成为新的免鉴权写入面，也不会被跨站页面触发
- `models` 里出现的、目录里没有的 id 会被**自动纳入可服务集合**（协议族取配置值或推断值），
  因此上游上新模型时无需重新编译
- 别名是**重命名**（对齐 CPA 自己的模型别名语义）：`Resolver` 用一个别名条目**替换**原名条目，
  `Entry.BaseID` 记住被替换的真实 id，`Upstream()` 把别名映射回它，执行器据此改写请求体里的
  `model` 字段（gemini 改 URL 路径）。原名不再注册、不再认领，因此模型列表与路由都只认别名
- 多个别名 = 同一模型的多个对外名（CPA「同一 name 重复配不同 alias」的等价物）；
  真实模型 id 永远优先于别名：冲突的别名被拒绝并记录在 `Resolver.Shadowed()`（值为声明它的模型 id），
  某个模型的别名若全部被拒绝，它保留自己的原名继续提供。仪表盘按 `BaseID` 归组，一个真实模型一行

## 6. 模型认领（model_router）

`RegisterExecutors` 会跳过"已被原生 provider 服务的模型 ID"，因此当用户的 CPA 已经通过
codex OAuth 提供 `gpt-6-luna` 时，插件默认抢不到它。解决办法是同时声明 `model_router`：

- 只在模型属于本协议族**且**配置为启用时返回 `{Handled: true, TargetKind: "self"}`
- 宿主在 provider 解析之前询问所有 router，因此插件可以稳定压过内置通道
- 逐模型的 `enabled` 开关同时也是"把这个模型让回内置通道"的开关

## 7. 安全边界

- 资源路由（iframe 页面与只读 API）**不需要管理密钥**，因此它们**永不返回明文 Key**，
  只返回掩码（`oc_sk_ab…wxyz`）与健康计数。宿主给资源响应加了 `Access-Control-Allow-Origin: *`，
  所以资源路由**只读**这一点是硬约束：任何"带副作用的 GET"都会变成跨站可利用的写入面，
  插件因此不提供免鉴权写入通道。
- 所有写操作走 CPA 自身的 `/v8/management/config/plugins/configs/<id>`，需要 Bearer 管理密钥，
  由宿主校验、持久化并热重载；插件不碰 config.yaml。
- 插件页**不实现认证、不显示密码框**：写入用的密钥从管理中心自己的
  `localStorage['cli-proxy-auth']`（zustand persist 名 + `enc::v1::` 混淆，material 含 host 与 UA）
  静默读取；读不到就退化为只读页面并提示去管理中心勾一次「记住密码」。
  密钥只存在于浏览器与那次 fetch 的请求头里，插件进程与磁盘上都不留。
- 探测路由挂在宿主管理鉴权下（`/v0/management/opencode-go/probe`，声明为没有 `Menu` 的
  `ManagementRoute`，避免被宿主的 legacy menu 逻辑转成资源路由），跨站请求会在管理中间件被 401。
- 页面自带 CSP（`default-src 'none'; script-src 'unsafe-inline'; connect-src 'self'; frame-ancestors 'self'`）、
  `Referrer-Policy: no-referrer`、`X-Content-Type-Options: nosniff`，不加载任何第三方脚本。

## 7.5 页面布局：为什么不放底部固定条

管理中心手机端的壳是 `.app-shell{min-height:100vh}` + 插件内容区 `height:100dvh`，
而 `.main-header` 是 `position:fixed` 且插件内容区 `padding:0`。在手机上 `100vh` 与真实可见高度
可以差出一两百像素，那部分会变成 iframe 下方一条**属于宿主页面**的空白。iframe 内的元素
（包括 `position:fixed`）都画不到那条空白里，任何贴底 UI 都会浮在它上方，看起来就是"底部有空隙"。

因此 v1.2.0 起：

- 页面是 flex 列：顶部操作栏 + 单块 `overflow:auto` 内容区，**没有任何 `position:fixed` 元素**
  （`internal/dashboard/page_test.go` 断言模板里不出现 `position:fixed`）
- 顶部内容按**实测**的宿主 header 高度下移：读 `parent.document.querySelector('.main-header')`
  的 `getBoundingClientRect().height`，兜底读父级 `--header-height`，再兜底 0，
  写进 `--host-header`；父页面不可访问（跨域/独立打开）时为 0，不影响渲染
- 主题与 header 高度都只在**读**父文档，不修改宿主页面（早期版本曾考虑注入 CSS，已否决）

## 7.6 仪表盘：读模型、改动补丁与凭证

页面挂在免鉴权的资源路由上（`/v0/resource/plugins/<id>/dashboard`），所以**页面永远不接触密钥**，
也**不需要预先登录**：

- **读**：`/api/state` + `/api/models` + `/api/quota` 三个插件自有接口就是页面的读模型，
  它们只返回解析后的设置与掩码 Key。状态接口因此必须携带页面可写的**每一个**字段
  （超时 / 冷却 / 重试 / 状态目录 / debug …），否则「无凭证也能编辑」就不成立；
  这条约束由 `TestStatePayloadExposesEditableSettingsWithoutSecrets` 守住
- **写**：页面把操作者的改动记成补丁（`edits = {top, models, keys:{add,del,set}}`），
  点保存时才 `GET /v8/management/config/plugins/configs/<id>` 读回宿主当前配置块，
  把补丁叠上去再 `PUT`。好处有三：页面从不持有 Key 明文就能增删 Key；
  页面没读到的字段（`extra-headers`、`models-default`、别人手改的配置）不会被覆盖；
  删除 Key 只影响卡片上列出的那些名字，别的 Key 原样保留
- **凭证**：写入需要宿主管理密钥。优先级是「本标签页会话里记住的」→「管理中心
  `localStorage['cli-proxy-auth']` 里存的」→「弹一次密码框」。401/403 时只重试一次，
  并把已缓存的凭证丢掉、在弹窗里显示原因；宿主对同一 IP 连续 5 次失败会封禁 30 分钟，
  所以页面**不做任何自动重试循环**，只在弹窗里提示
- **状态呈现**：`dirty = countEdits() > 0` 同时驱动三处 —— 保存按钮的高亮与角标、
  标题旁的状态 chip、以及对应 Tab 上的小圆点，保证「有没有没保存的东西」在任何 Tab 都看得见

## 8. 版本兼容

插件 ABI 是 `dlopen` + C ABI（`ABIVersion = 1`，注册 `schema_version = 6`），
与宿主的 Go 工具链、依赖版本**无关**（现网的 `cap-token-usage-tracker` 用 go1.26.8 + v7 SDK
构建，在 v8.0.27 宿主上正常工作即为例证）。本项目只 import public 的 `sdk/pluginapi` 与
`sdk/pluginabi`，不依赖 `internal/*`，因此 CPA 升级只需要重新编译。
