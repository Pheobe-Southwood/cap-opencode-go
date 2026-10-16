# 验收记录（CLIProxyAPI v8.0.27 / 插件 v1.3.0）

> v1.3.0 在生产实例 127.0.0.1:8317 实测：**15 passed, 0 failed**（messages 协议族仍因操作者
> 关闭了 `claude-haiku-5-5` 而 SKIP）。本轮把别名从「额外注册一个 id」改成**重命名**，
> 因此重点验收「模型列表与路由都只认别名」，并顺带验证了**不重启服务的配置热重载**。

## v1.3.0 别名重命名验收（生产实例，热重载，未重启进程）

部署方式：`scripts/install.sh 1.3.0` 把 4 个新 `.so` 放进插件目录（**保留** v1.2.0 的 4 个文件），
然后**原地**（保持 inode）在 config.yaml 的插件块上方补一行版本注释 → fsnotify 检测到内容变化 →
`pluginHost.ApplyConfig` 重新扫描插件目录 → 按版本号选中 1.3.0 并热加载。进程始终是同一个：

```
PID 3194299  STARTED Fri Oct 16 05:21:57 2026  ELAPSED 12:32   # 整个替换过程未重启
```

| 断言 | 结果 | 证据 |
|---|---|---|
| 别名取代原名（HTTP 列表） | ✅ | `/v1/models` 从 16 项变 15 项：有 `gemini-4-argon`，**没有** `gemini-4-argon-contributor` |
| 别名取代原名（Gemini 列表） | ✅ | `/v1beta/models` 只返回一条 `models/gemini-4-argon`；v1.2.0 时是两条同名的 `models/gemini-4-argon-contributor` |
| 别名仍然真实可用 | ✅ | `POST /v1beta/models/gemini-4-argon:generateContent` → **200**，上游回显 `"modelVersion":"gemini-4-argon-contributor"` |
| 原名不再可调用 | ✅ | `POST /v1beta/models/gemini-4-argon-contributor:generateContent` → **400** `model_not_found`（`unknown provider for model …`） |
| 改写在执行层发生 | ✅ | 隔离实例 `debug: true` 的 trace：`requested=gemini-4-argon` / `upstream=gemini-4-argon-contributor` |
| 仪表盘一行一个真实模型 | ✅ | `/api/state`：`version=1.3.0`，37 行；`{id: gemini-4-argon-contributor, client_ids: [gemini-4-argon], aliased: true}`，无独立别名行 |
| 官方验收脚本 | ✅ | `gemini=gemini-4-argon`（脚本改为取 `client_ids[0]`）；**15 passed, 0 failed** |
| 零残留不变 | ✅ | 未新增任何落盘物：仍只有 `data/cap-opencode-go/shared.json`（+ 可选 `debug.log`），别名只存在宿主拥有的 config.yaml 里 |

```
$ CPA_BASE=http://127.0.0.1:8317 CPA_CLIENT_KEY=sk-… bash scripts/acceptance.sh
== OpenCode Go acceptance @ http://127.0.0.1:8317 ==
served models: chat=deepseek-v4-pro responses=gpt-6-luna gemini=gemini-4-argon messages=none

  PASS  chat/completions non-stream                    200  553 bytes
  PASS  chat/completions stream                        200  2829 bytes
  PASS  chat model via claude client                   200  279 bytes
  PASS  chat model via responses client                200  875 bytes
  PASS  responses non-stream                           200  1086 bytes
  PASS  responses stream                               200  11753 bytes
  PASS  responses model via chat client                200  531 bytes
  PASS  responses model via claude client              200  2042 bytes
  PASS  gemini generateContent                         200  571 bytes
  PASS  gemini streamGenerateContent                   200  1206 bytes
  PASS  gemini model via chat client                   200  374 bytes
  SKIP  messages family                                no messages model enabled
  PASS  /v1/models catalogue                           15 models
  PASS  dashboard state resource
  PASS  live model list resource                       upstream 37 models
  PASS  quota resource

== 15 passed, 0 failed ==
```

回滚路径（同样不用重启）：删除插件目录里的 `cap-opencode-go*-v1.3.0.so`，再原地补一行注释 →
热重载回 v1.2.0。v1.2.0 的 `.so` 一直留在原地就是为了这个。

---

# 验收记录（插件 v1.2.0）

> v1.2.0 在生产实例 127.0.0.1:8317 实测：**15 passed, 0 failed**（同上基线，消息协议族仍因
> 操作者关闭了 `claude-haiku-5-5` 而 SKIP）。本轮新增的接口（协议探测）是管理鉴权路由，
> 在无管理密钥的环境下以 `401`（路由存在）复核，并在隔离实例上以真管理密钥跑通
> （`GET /v0/management/opencode-go/probe?model=grok-4.7` → 200，逐族返回可用性）。

## UI 验收（隔离实例 + 无头 Chromium，22/22）

用一次性实例（端口 18317，明文管理密钥）加载真实的 `management.html`，在 412×915 手机视口与
1440×940 桌面视口下由 CDP 驱动断言：

| 断言 | 结果 |
|---|---|
| 页面内**没有任何 `position:fixed` 元素**（底部不再有固定条） | PASS |
| 顶部内移 = 管理中心 header 实测高度（`--host-header` = 74px） | PASS |
| 内容在单块滚动区里滚动 | PASS |
| 首屏可见搜索框、38 个模型行、设置面板默认收起 | PASS |
| 复用管理中心已存凭据 → 页面可写（无只读提示） | PASS |
| 点「保存」→ 配置真的变更（`models.deepseek-v4-pro.proxy = direct`） | PASS |
| 保存后操作者原有字段不丢（base-url / session / session-mode / keys 原样保留） | PASS |
| 「探测协议」按钮返回逐协议族结果 | PASS |
| 未勾「记住密码」登录 → 页面只读、**全程 0 次弹窗**（`Page.javascriptDialogOpening` 未触发） | PASS |
| 只读模式下模型列表照常显示 | PASS |

---

## 早期记录（插件 v1.1.1）

> 最新一轮在生产实例 127.0.0.1:8317 上执行：**15 passed, 0 failed**（messages 协议族因操作者
> 关闭了 `claude-haiku-5-5` 而 SKIP）。验收脚本会先读 `/api/state` 选出每个协议族里**已启用**
> 的模型再测试，所以操作者增删模型不会造成假失败。

```
$ CPA_BASE=http://127.0.0.1:8317 CPA_CLIENT_KEY=sk-… bash scripts/acceptance.sh
== OpenCode Go acceptance @ http://127.0.0.1:8317 ==
served models: chat=deepseek-v4-pro responses=gpt-6-luna gemini=gemini-4-argon-contributor messages=none

  PASS  chat/completions non-stream                    200  552 bytes
  PASS  chat/completions stream                        200  2828 bytes
  PASS  chat model via claude client                   200  279 bytes
  PASS  chat model via responses client                200  841 bytes
  PASS  responses non-stream                           200  2874 bytes
  PASS  responses stream                               200  11024 bytes
  PASS  responses model via chat client                200  534 bytes
  PASS  responses model via claude client              200  1937 bytes
  PASS  gemini generateContent                         200  571 bytes
  PASS  gemini streamGenerateContent                   200  1206 bytes
  PASS  gemini model via chat client                   200  377 bytes
  SKIP  messages family                                no messages model enabled
  PASS  /v1/models catalogue                           15 models
  PASS  dashboard state resource
  PASS  live model list resource                       upstream 37 models
  PASS  quota resource

== 15 passed, 0 failed ==
```

验收脚本自 v1.1.1 起断言**响应体非空**并检查协议标志（`"choices"` / `"object":"response"` /
`"candidates"` / `"content"` / `data:` / `event:`），因为翻译链路出问题时表现为
「HTTP 200 但 body 为空」——这正是 v1.1.1 修掉的那个问题。

---

## 早期记录（临时实例 127.0.0.1:8399）

测试环境：隔离的临时实例，插件目录 `/tmp/cpa-test/plugins`、auth 目录 `/tmp/cpa-test/auth`、
数据目录 `/tmp/cpa-test/data`，**未触碰用户的 8317 生产实例**。

```
$ CPA_BASE=http://127.0.0.1:8399 CPA_CLIENT_KEY=sk-… CPA_MGMT_KEY=… bash scripts/acceptance.sh
== OpenCode Go acceptance @ http://127.0.0.1:8399 ==
  PASS  chat/completions non-stream                    200
  PASS  chat/completions stream                        200
  PASS  responses non-stream                           200
  PASS  responses stream                               200
  PASS  responses model via chat client                200
  PASS  gemini generateContent                         200
  PASS  gemini streamGenerateContent                   200
  PASS  gemini model via chat client                   200
  PASS  messages non-stream                            200
  PASS  messages stream                                200
  PASS  chat model via claude client                   200
  PASS  chat model via responses client                200
  PASS  /v1/models catalogue                           37 models
  PASS  sidebar menu entry                             OpenCode Go /v0/resource/plugins/cap-opencode-go/dashboard
  PASS  dashboard state resource
  PASS  quota resource

== 16 passed, 0 failed ==
```

## 逐条对照计划中的验收标准

| 编号 | 标准 | 结果 | 证据 |
|---|---|---|---|
| A1 | chat 家族可用 | ✅ | `deepseek-v4-pro` / `glm-5.3-flash` 非流式 + 流式均 200 |
| A2 | responses 家族可用 | ✅ | `gpt-6-luna` / `grok-4.7` 经 `/v1/responses` 非流式 + 流式均 200 |
| A3 | Gemini4 可用 | ✅ | `gemini-4-argon-contributor` 非流式返回 `"modelVersion":"gemini-4-argon-contributor"`，流式 SSE 正常 |
| A4 | 其它客户端协议可用 | ✅ | chat 模型经 `/v1/messages`、`/v1/responses`；responses 模型经 `/v1/chat/completions`；argon 经 `/v1/chat/completions` 全部 200 |
| A5 | 侧边栏入口 | ✅ | `GET /v8/management/plugins` 返回 `menus:[{menu:"OpenCode Go", path:"/v0/resource/plugins/cap-opencode-go/dashboard"}]`；`/dashboard` 返回 26 KB HTML（`text/html; charset=utf-8`，带 CSP） |
| A6 | 每模型代理开关真实生效 | ✅ | `gpt-6-luna` 设 `direct` → **403 `unsupported_country_region_territory`**；改 `proxy` → **200**。开关经由管理 API 热重载生效 |
| A7 | 额度显示 | ✅ | `GET /api/quota` 返回 `[("5 小时",0%),("本周",6%),("本月",18%)]` 及各自 `resets_at` |
| A8 | 多 Key | ✅ | Key 池放入一个无效 Key + 一个有效 Key，连续 3 次请求全部 200；坏 Key 记录 401 并进入冷却，好 Key 计数 7 次成功 |
| A9 | 完全可回滚 | ✅ | 删除 4 个 `.so` + 4 个配置块 + 数据目录并重启后：`/v1/models` 为 0、插件列表为空、dashboard 404、`data/cap-opencode-go` 不存在、auth 目录 0 文件 |

## 附加验证

- **插件加载**：4 个实例全部 `plugin loaded` + `plugin registered`，无告警。
- **协议格式声明**：`cap-opencode-go` 声明 `["openai"]`，`-responses` 声明 `["codex","openai-response"]`，
  `-messages` 声明 `["claude"]`，`-gemini` 声明 `["gemini"]`。
- **模型归属**：`/v1/models` 37 项全部 `owned_by = "opencode"`。
- **配置持久化**：仪表盘写入路径（`PUT /v8/management/config/plugins/configs/cap-opencode-go`）返回
  `{"config-version":8,"status":"ok"}`，宿主重写 config.yaml 时完整保留了插件自定义字段。
- **单元测试**：`go test ./...` 全绿；`go vet ./...` 无输出。

## 排查过程中定位的三个宿主侧陷阱（已写入 README）

1. 分离 goroutine 里调用 `host.log` 会让宿主 **SIGSEGV**（实测），诊断改写文件。
2. `codex → openai-response` 的流式翻译若与输入逐字节相同，宿主判定为注册表回退并
   **丢弃全部流式帧**，导致 502 `upstream stream closed before first payload`；
   解法是 responses 协议族同时声明 `openai-response`，让客户端本就使用该协议时走直通分支。
3. `codex → openai` / `codex → claude` 的**非流式**翻译器只接受终止事件载荷
   （`type` = `response.completed` / `response.incomplete`），收到裸 response 对象返回空 body；
   插件在 `Format == "codex"` 时把上游 body 包成 `{"type":"response.completed","response":{…}}`。
