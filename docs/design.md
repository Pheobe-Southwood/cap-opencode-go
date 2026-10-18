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

### 2.1 上游协议是硬绑定的（实测矩阵）

合并成一个 `.so` 的前提是"插件自己会翻译"，否则必须让上游接受跨协议调用。2026-10 直接对着
`https://opencode.ai/inference/go/v1` 探测（每个组合一次极小的真实请求，均带
`x-opencode-session`，`User-Agent: Go-http-client/1.1`）：

| 模型（协议族） | `/chat/completions` | `/responses` | `/messages` | GenAI `:generateContent` |
|---|---|---|---|---|
| `deepseek-v4.1-flash`（chat） | 200 | 200 | 200 | **400 `ModelProtocolUnsupported`** |
| `claude-haiku-5-5`（messages） | **400** | **400** | 200 | **400** |
| `grok-4.7`（responses） | **400** | 200 | **400** | **400** |
| `gemini-4-argon-contributor`（gemini） | **400** | **400** | **400** | 200 |
| `gpt-6-luna` / `muse-spark-1.3`（responses） | — | 200 | **400** | — |

即：chat 族模型接受 chat / messages / responses 三种协议，**专有协议模型只接受自己那一种**，
且 gemini 协议只对 gemini 族开放。若把四个面合并成一个声明全部格式的插件，宿主会把客户端协议
**直通**给插件（第 1 条选择逻辑），于是上面每一个 `400` 组合都会变成客户端可见的失败
——今天这些组合全部可用，靠的正是宿主按"该面声明的格式"做翻译。

### 2.2 为什么不做真正的单文件合并

- 宿主的注册契约是**一个插件 = 一个 record = 一个 provider = 一组声明格式**
  （`internal/pluginhost/adapters_executors.go` 里 `seenProviders` / `claimedProviders` 去重，
  `rpc_schema.go` 的 `Capabilities` 只有一个 executor），没有"一个插件声明多个 provider"的入口；
- 唯一能让单文件覆盖全部组合的做法是在插件内实现协议转换，而宿主的翻译器在
  `internal/translator/{openai,claude,codex,gemini}`（约 3.5 万行），外部模块**无法 import 内部包**
  （`sdk/translator` 本身就依赖 `internal/thinking`），只能整段复制并长期跟随上游演进；
- 流式保真、`codex` 双向翻译的已知怪癖（见第 5.5 节、README「已知取舍」）都要在插件里重新踩一遍。

因此 v1.5 的取舍是：**保留四个面，把呈现与管理负担收敛到一个产品**（见第 7.7 节）。

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

## 6. 模型认领（model_router）与宿主路由

`RegisterExecutors` 会跳过"已被原生 provider 服务的模型 ID"，因此当用户的 CPA 已经通过
codex OAuth 提供 `gpt-6-luna` 时，插件默认抢不到它。解决办法是同时声明 `model_router`：

- 只在模型属于本协议族**且**配置为启用时返回 `{Handled: true, TargetKind: "provider", Target: r.role.Provider, TargetModel: upstreamModel}`
- 宿主在 provider 解析之前询问所有 router，因此插件可以稳定认领模型。
- 通过返回 `TargetKind: "provider"`，插件将认证选择、多 Key 轮询与故障转移完全委托给 CPA 宿主的 `AuthManager`（遵循 CPA 宿主「网络配置」的路由策略，如轮询、填满优先等）。
- 插件仅设置 Key 的 `priority` 优先级，相同优先级由宿主调度。
- 网络代理（`proxy-url`、`default-proxy`、模型 `proxy`）依然由插件自行持有 transport 并在出站请求中应用。
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
  静默读取；读不到就弹一次密码框（可勾「在本标签页内记住」，只写 `sessionStorage`），而不是把页面变成只读。
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

- **读**：`/api/state` + `/api/models` + `/api/quota` + `/api/stats` 四个插件自有接口就是页面的读模型，
  它们只返回解析后的设置与掩码 Key。状态接口因此必须携带页面可写的**每一个**字段
  （超时 / 状态目录 / 套餐 / debug …），否则「无凭证也能编辑」就不成立；
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

## 7.7 四个协议面当一个产品管理（v1.5）

宿主的插件列表按 `.so` 文件逐行渲染（`DiscoverPluginFiles` → 每行一个 entry），四行无法从插件侧
隐藏或合并。能做的是让这四行**读起来是一个产品的四个面**：

- **同一个图标**：`internal/brand` 内嵌 OpenCode 官方 mark（`internal/brand/opencode.svg`，
  来自 <https://opencode.ai/favicon.svg>，去掉外层包装与无用的 `prefers-color-scheme` 样式块），
  注册元数据下发 `data:image/svg+xml;base64,…`。宿主的插件行与侧边栏菜单图标都从
  `Metadata.Logo` 取值，且管理中心的 `VM()` 明确放行 `data:`，所以一处声明同时点亮两处，
  不需要额外静态资源路由，也不产生网络请求。仪表盘页头与 favicon 复用同一份字符串
  （`dashboard.Page()` 注入 `__BRAND_MARK__` / `__BRAND_FAVICON__`），三者不会漂移
- **角色化命名**：`roles.Role.Label` —— 主面是产品名 `OpenCode Go`，从属面追加 `· Responses` /
  `· Messages` / `· Gemini`；`Metadata.Name` 直接用这个值，单测保证四个名称互不相同、
  且只有主面可以只叫产品名
- **零配置从属面**：`configFields(role)` 只对 chat 面返回字段。从属面的配置块只有 `enabled`
  （加载开关），它们读主面发布的 `shared.json`；把字段列在它们的表单里只会让人以为要配两遍。
  插件仍保留「`shared.json` 缺失时回退解析自身配置」的旧行为，因此旧配置不会被打破
- **仪表盘的「协议面」卡片**：`/api/state` 的 `siblings[]` 现在带 `label` / `protocol` / `formats` /
  `endpoint` / `main` / `model_count` / `models`（每面最多 5 个示例 id），数据全部来自
  `roles.All` 与 resolver，不增加任何上游调用。页面据此逐面说明"负责什么协议、哪些模型依赖它、
  是不是配置入口"，并写明四个 `.so` 必须同版本、停用或缺失某面会让该协议族模型无法路由
- **从属面版本无法在插件侧读取**（从属面不注册管理路由），所以「同版本」只能作为约定写在
  卡片与 README 里；升级时用 `scripts/install.sh <版本>` 一次装四个

## 7.8 统计面板：宿主记账 + 插件测时（v1.7–v1.7.8）

统计要回答「用过的模型花了多少、占月额度多少、快不快」。这需要两类数据，而它们的最佳来源不同：

- **token / 缓存 / 失败 / 请求总延迟**：宿主的 usage 事件已经按协议解析并归一化
  （`pluginapi.UsageRecord.Detail`）。插件自己再解析一遍响应既重复又容易和宿主的翻译层漂移。
  因此**只有主面**声明 `usage_plugin: true`，宿主把四个面执行的请求都投递给它；三个从属面不订阅，
  否则同一条记录会被四个运行时各记一次。
- **TPS**：宿主 usage 记录的 `latency_ns` 正常，但 `ttft_ns` 对本插件异步流桥接恒为 0。
  TPS 因此不再使用插件 generation 时间，而是同一条 usage 记录里的
  `effective output / latency`：分子是成功请求的有效 output（gemini 把 reasoning 计入），
  分母是宿主请求总延迟。两者天然按请求配对，包含首字、排队与网络，所以是**整体 TPS**，
  不是纯解码速度；失败请求不计。非流式请求也有总延迟，因此也进入整体 TPS。
  旧 stats 没有 `latency_nano` 时 TPS 为空，不用 generation 时间冒充。
- **生成 TPS（次要估算）**：`stream_output_tokens / generation_nano`，即插件测的首内容到末内容，
  不含 TTFT。宿主 usage 没有可与 timing 配对的 request id，因此按 `generation_samples /
  stream_requests` 的覆盖率缩放分子；样本数随 API 返回（`gen_tps_samples`），页面标注「插件实测」。
  它更接近解码速度，但仍不是严格 benchmark；整体 TPS 才是严格配对的主指标。
- **TTFT 与 generation 诊断**：四个面在 `pumpStream` 里自己打时间戳：发起上游请求前记
  `started`，第一个内容帧（chat 的 `delta.content/reasoning_content/tool_calls`、responses 的
  `*.delta`、messages 的 `content_block_delta`、gemini 的 `parts[].text/functionCall`）记
  `firstContent`，之后每个内容帧更新 `lastContent`。TTFT = `firstContent - started`。
  内容识别失败时**没有 first-frame 回退**；上游 error 帧、空流、客户端断开都不产生样本。
  generation = `lastContent - firstContent`，只保留为诊断，不参与 TPS；framed 协议的
  blank/event/[DONE] 行不会再把 generation 拉长。

四个 `.so` 是四个独立运行时，不能共享内存，于是沿用 `shared.json` 的模式，每个面写
`<state-dir>/stats-<role>.json`（主面写 chat 文件），主面的 `/api/stats` 读取自己的快照 +
三个从属面的 timing 文件，按模型合并。主面只从从属文件合并 timing 计数（`Counter.MergeTiming`），
不合并 token，避免同一条 usage 记录被重复计数。文件是版本化 JSON，临时文件 + rename 原子写，
0600；每 5 秒 debounce flush，`plugin.shutdown` 时再 flush 一次。

数据口径：

- **价格**：`internal/pricing` 内嵌 2026-10 的 Go 价目表（Go 与 Go Plus 的逐模型月限额、
  长上下文档位、DeepSeek 工作峰谷价、免费/未定价模型）。chat/responses/gemini 的 input 包含缓存，
  messages 的 input 不包含；gemini 的 output 费用含 reasoning。按请求开始时间取档位与峰谷，
  以 nano-USD 整数累计，API 再换算成美元。
- **价目核对提醒**：`SnapshotCheckedAt` 记录这张内嵌表最后一次与官方 Go 页面人工核对的日子，
  与构建时间分开——只重新编译而没重新核对，旧价格不会假装变新。`SnapshotFreshness` 以 UTC
  日历天计算 `age_days`，满 `RefreshAfterDays`（30 天）就把 `price_book_update_due` 置为 true；
  `/api/stats` 一并返回 `price_book_checked_at`、`price_book_due_at` 等字段。页面常驻显示核对日期，
  到期后显示「数据该更新了」和官方链接。它只是人工核对提醒，不做网络抓取，也不自动改写价目表或
  历史金额。
- **模型列表变更**：`SnapshotModelIDs` 保存核对时 `/models` 返回的 37 个 id，与内置 catalogue
  分开——catalogue 是“这个构建会服务的模型”，快照是“核对时上游实际发布的模型”。每次
  `/api/models` 成功拉取后，主面用 `CompareModelList` 做集合 diff，把结果保存在 Runtime；
  `/api/stats` 只读该结果，返回 `model_list_checked_at`、`model_list_changed`、
  `model_list_added`、`model_list_removed`，自己不发起上游请求。页面打开统计 Tab 时会顺带刷新
  模型列表（复用 5 分钟缓存）；新增或移除模型即使未满 30 天也会显示同一个「数据该更新了」横幅，
  并列出变更 id。空列表/拉取失败不会当成“全部模型被移除”，只保留上一次成功检查。
- **月额度**：模型在「本月窗口」内的刊例价 ÷ 当前 `plan` 下该模型的月限额。`/usage` 只返回百分比，
  无法可靠反推套餐，所以套餐必须配置（默认 `go`）。窗口起点由页面取所有 Key 的 monthly
  `resets_at` 最晚值往前推一个月；拿不到时服务端回落到 UTC 自然月，并在 `range.since_source` 标记。
- **保留期**：逐小时桶保留 40 天（覆盖一个月的窗口），全量 totals 永久保留。v1.7 起可用
  `cmd/replay-tracker` 从旧 token tracker 的 BoltDB **只读**回放 opencode-go 历史；默认只导入
  现有 stats 最早记录之前的部分，导入前备份、导入后写 marker，拒绝重复导入。
- **历史回放**：只复制逐请求 token、费用、总延迟和可用的 TTFT；旧记录的 `ttft_ns=0` 时不伪造
  TTFT，只回放 token/费用/总延迟。模型别名按 `shared.json` 映射，费用按当前价目表重算。
- **API**：`GET /api/stats?range=month|all[&since=RFC3339][&key=<keyName>]`，返回 `usage_records{received,matched}`
  便于诊断 provider 标签不匹配；响应包含模型 id、计数、费用、时间戳以及 `by_key` 逐 Key 额度与剩余，支持按 Key 筛选全部模型或特定 Key。
- **页面**：概览常驻；工具栏支持按 Key 筛选；模型明细在 ≤640px 默认折叠；统计口径始终折叠。桌面端是表格，手机端
  用 `table.responsive` 的 `data-label` 变成逐模型卡片。各模型在「本月额度」列展示各 Key 的剩余额度及百分比。费用、百分比、TPS/TTFT 都在前端格式化，
  没有新增运行时依赖。价目/模型核对状态只占一行，任何提醒横幅都在概览上方，不改变模型明细的
  折叠行为。

## 7.9 已移除的宿主配额集成（v1.8–v1.11）

v1.8 起，插件曾把每个 Key 投影成宿主的认证文件（v1.11 起每 Key 1 个 chat 面文件），并在主面
注册 `quota_provider` 能力、实现 `quota.identifier/describe/fetch/reset`，让 CPA 自带的
「配额管理 → Plugin」把 Key 当成账号列出余量。这条通道的代价是：宿主的「认证文件」页与配额页
共用同一份账号列表（v8.0.27 没有“只对配额页可见”的隐藏标记），认证文件必然出现在「认证文件」页。

v1.12 按操作者要求整体移除：不再写认证文件、不再声明 `quota_provider`、不再应答 `quota.*`。
余量改由插件自己的仪表盘「账号管理」Tab 提供（直接查询上游 `/usage`，见 §7.8），与宿主无关。
路由一直由插件自己的 `model_router` 负责（§7.5），所以这次移除不影响任何协议面的转发；
`scripts/uninstall.sh` 保留对历史 `opencode-go-*.json` 的标记清理。

## 8. 版本兼容

插件 ABI 是 `dlopen` + C ABI（`ABIVersion = 1`，注册 `schema_version = 6`），
与宿主的 Go 工具链、依赖版本**无关**（现网的 `cap-token-usage-tracker` 用 go1.26.8 + v7 SDK
构建，在 v8.0.27 宿主上正常工作即为例证）。本项目只 import public 的 `sdk/pluginapi` 与
`sdk/pluginabi`，不依赖 `internal/*`，因此 CPA 升级只需要重新编译。
