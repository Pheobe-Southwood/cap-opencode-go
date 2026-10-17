# 验收记录（CLIProxyAPI v8.0.27 / 插件 v1.6.1）

> v1.6.1（「统计」Tab：宿主 usage 记账 + 插件自测 TTFT/TPS + 价目表/套餐月额度；
> 1.6.1 修正首屏月窗口竞态、月末倒推与保存后刷新）
> 在生产实例 127.0.0.1:8317 热重载实测：PID 不变、版本 1.6.1、`plan=go`；
> 公开验收 **32 passed, 0 failed**（管理密钥是 bcrypt 哈希，环境里没有明文，故 6 项管理检查跳过）；
> 隔离实例 127.0.0.1:8399 跑完整套 **40 passed, 0 failed**（含 messages 族、四个协议面、管理探测）。
> 无头浏览器验证生产统计页桌面 + 手机（390px）无 JS 异常、无横向溢出、手机默认折叠模型明细。
> 下面是本轮记录，其后是历史记录（v1.5.0 协议面品牌整合、v1.4.1 仪表盘改造、v1.3.0 别名重命名、v1.2.0、v1.1.1）。

---

## v1.6.0 / 1.6.1 统计面板（生产热重载 + 隔离实例 40 项 + 桌面/手机浏览器）

本轮新增只读观测面，不改变任何协议转换、模型路由、别名或 Key 池行为。设计见
[design.md](design.md) §7.8；数据来源与口径在页面「统计口径与价目说明」里也有一份。

### 1. 生产实例：热重载到 1.6.1

```
$ CPA_ROOT=/root/cliproxyapi bash scripts/install.sh 1.6.1
installed …/cap-opencode-go-v1.6.1.so
installed …/cap-opencode-go-responses-v1.6.1.so
installed …/cap-opencode-go-messages-v1.6.1.so
installed …/cap-opencode-go-gemini-v1.6.1.so

$ # 原地追加一行注释触发 fsnotify（保持 inode 10653636）
$ # 新增文件而不是覆盖已 dlopen 的 1.5.0
```

| 断言 | 结果 | 证据 |
|---|---|---|
| 热重载到 1.6.1 | ✅ | `/api/state` → `version=1.6.1`、`role=chat`、`plan=go`、`config_error` 为空 |
| 进程未重启 | ✅ | PID `3194299` 全程不变，ELAPSED 从 20:59:10 继续增长 |
| 四个面同版本 | ✅ | 插件目录同时存在四个 `-v1.6.1.so`；热重载日志按版本选中 |
| 生产页面 = 仓库源码 | ✅ | 生产 `GET /dashboard` 与 `go test` 生成的 `dashboard.Page()` 字节一致（各 120 194 bytes，`cmp` 退出 0） |
| 配置只多注释 | ✅ | `config.yaml` inode 10653636 不变；只追加 `# (v1.6.0 …)` / `# (v1.6.1 …)` 注释 |
| 统计落盘 | ✅ | `data/cap-opencode-go/` 新增 `stats-chat.json`、`stats-gemini.json`、`stats-responses.json`（0600）；`shared.json` 仍在 |
| 统计文件无 Key | ✅ | `grep -l oc_sk_ stats-*.json` 无输出 |
| 公开验收 | ✅ | **32 passed, 0 failed** |

生产环境没有管理密钥明文：`management.secret-key` 是 60 字符 bcrypt 哈希（`$2a$…`），
所以侧边栏、四行品牌、管理探测这 6 项按脚本的 preflight 规则跳过；它们由隔离实例覆盖。

### 2. 生产统计结果（真实上游请求）

用客户端 `DeepSeek-V4.1-Flash` 发一次 `stream=true` 的 `max_tokens=8` 请求，等待一个 5 秒
落盘周期后：

```
$ curl -s http://127.0.0.1:8317/v0/resource/plugins/cap-opencode-go/api/stats?range=all
usage_records  = {"received": 5, "matched": 5}
totals         = requests=5 tokens=827110 cost_usd=0.003360414
                 ttft_samples=4 tps_samples=4 avg_tps=389.87 avg_ttft_ms=3088.3
deepseek-v4.1-flash  requests=5  input=825817 output=1293 cached=825088
                     cost_usd=0.003360414  avg_tps=389.87 avg_ttft_ms=3088.3
```

随后完整公开验收生成了更多请求（chat / responses / gemini 的流式与非流式）：

```
statistics resource  ready | matched=136 requests=136 tokens=13518739 ttft=102 tps=102 missing_families=-
  deepseek-v4.1-flash(chat)       r=92 tok=13516614 $0.1542 ttft=90 tps=90
  gemini-4-argon-contributor(gemini) r=12 tok=542  $0.0047 ttft=4  tps=4
  gpt-6-luna(responses)            r=16 tok=530     $0.0002 ttft=4  tps=4
  beam-free(chat)                  r=16 tok=1053    $0.0000 ttft=4  tps=4
```

- 主面聚合了 chat 的 token 与三个从属面的 timing：`stats-responses.json` /
  `stats-gemini.json` 各自出现后，`gpt-6-luna` 与 `gemini-4-argon-contributor` 才有
  TTFT/TPS 样本，说明跨 `.so` 合并生效。
- 月额度：生产页面从 `/api/quota` 的 monthly `resets_at`（2026-11-04）往前推一个月到
  2026-10-04，账户本月 **44%**；最终验收请求跑完后该模型约 **0.19%**
  （$0.1542 / $80.00 Go 限额；截图拍摄于较早一次运行，数值随请求增长）。
- `usage_records.matched=136` 等于 `received`，provider 过滤没有漏记录；若为 0，页面会显示诊断提示。
- 验收脚本的统计轮询现在要求**每个跑过流式检查的协议族**都出现 TTFT/TPS 样本
  （本次 chat/responses/gemini 均为 `missing_families=-`），避免只等到主面 chat timing 就提前通过。

### 3. 隔离实例：完整 40 项

`/tmp/cpa-161-test`：v8.0.27 + 四个 1.6.1 `.so`，独立客户端 key 与明文管理密钥，
chat（Beam）/ responses（gpt-6-luna）/ gemini（gemini-4-argon）/ messages（claude-haiku-5-5）
全部打开，`requests.proxy-url=http://127.0.0.1:7890`（与生产一致）：

```
== 40 passed, 0 failed ==
  PASS  chat/completions non-stream / stream
  PASS  responses non-stream / stream
  PASS  gemini generateContent / streamGenerateContent
  PASS  messages non-stream / stream
  PASS  sidebar menu entry
  PASS  plugin faces / icon / names / config（main 15 fields, siblings 0）
  PASS  statistics resource  ready | matched=13 requests=13 tokens=605 ttft=3 tps=3
  PASS  stats payload hides secrets
  PASS  statistics monthly window
  PASS  management protocol probe  best=responses
  PASS  dashboard page: 统计 tab / 统计面板 / 统计概览 / 统计表 / 统计口径折叠 / TTFT 列 / 套餐设置
  PASS  state carries the editable settings（含 plan）
```

### 4. 桌面与手机页面（无头 Chromium）

生产页面 `#stats`，CDP 设置 1280×900 与 390×844：

| 断言 | 结果 |
|---|---|
| 页面无未捕获 JS 异常 | ✅（`Runtime.exceptionThrown` 无事件） |
| 手机无横向溢出 | ✅（`innerWidth=390`、`documentElement.scrollWidth=390`） |
| 手机默认折叠「模型明细」 | ✅（`#statsModelsFold.open === false`） |
| 桌面默认展开「模型明细」 | ✅ |
| 「统计口径与价目说明」始终折叠 | ✅ |
| 手机展开后表格变卡片 | ✅（`data-label` 生效；截图见 README） |

截图：`docs/screenshots/desktop-stats.png`、`mobile-stats.png`（折叠）、
`mobile-stats-open.png`（展开明细）。

### 5. 单元/静态检查

```
$ go test ./... && go vet ./...
ok   …/internal/pricing
ok   …/internal/stats
ok   …/internal/plugin
ok   …/internal/dashboard
…

$ go test -race ./...
ok   … (all packages)
```

新增覆盖：价目表档位/峰谷/缓存/免费/未定价/Go Plus 限额；小时桶窗口、裁剪、角色不匹配、
损坏文件恢复、Shutdown、并发；四协议内容帧识别；provider 过滤、别名→base、messages 缓存口径、
gemini reasoning、流式 timing、`/api/stats` 合并与月百分比、管理路由、页面统计标记。

### 6. 回滚对照

统计只新增 `data/cap-opencode-go/stats-<role>.json`（0600，无 Key）。删除 1.6.1 的四个 `.so`
并原地改一次 config.yaml 即回退到 1.5.0；1.5.0 不认识 `plan` 字段，也不会读这些文件，
`scripts/uninstall.sh` 会连同数据目录一起删除。回滚清单详见 [rollback.md](rollback.md)。

---

## v1.5.0 四个协议面品牌整合（生产实例热重载 + 隔离实例 30 项 + 无头浏览器）

本轮不改变任何协议/路由行为：四个 `.so` 依然各声明一种上游协议，全部协议翻译仍由 CPA 完成。
改变的是「插件管理」里那四行如何呈现，以及管理负担如何收敛到一个产品（方案与理由见
[design.md](design.md) §2.1 / §2.2 / §7.7）。

### 1. 生产实例：热重载部署，进程未重启

```
# 装新版本（新增版本号文件，绝不覆盖已 dlopen 的 .so；旧版本留着随时可回滚）
$ CPA_ROOT=/root/cliproxyapi bash scripts/install.sh 1.5.0
installed /root/cliproxyapi/plugins/linux/amd64/cap-opencode-go-v1.5.0.so
installed …-responses-v1.5.0.so   installed …-messages-v1.5.0.so   installed …-gemini-v1.5.0.so

# 原地补两行注释（保持 inode），fsnotify → ApplyConfig → 按版本号选中最新的 .so
$ python3 /tmp/touch_config_150.py
touched
$ ls -i config.yaml
10653636 config.yaml        # inode 前后一致，监听器没有跟丢
```

| 断言 | 结果 | 证据 |
|---|---|---|
| 热重载到 1.5.0 | ✅ | `/api/state` → `version=1.5.0`（改前 `1.4.1`），37 个模型 id 与 Key 池 `[('main', True)]` 不变 |
| 进程未重启 | ✅ | PID `3194299` 全程不变（`systemctl status` 仍显示 05:21:58 启动、ELAPSED 连续增长），管理页面会话未中断 |
| 四个面都真的换了 | ✅ | 热重载后立刻跑验收：chat / responses / gemini 三个协议族的直连与跨协议调用全部 200（messages 族因操作者关闭了全部 messages 模型而 SKIP，与 v1.4.1 相同）。热重载不重打 `pluginhost: plugin registered` 日志，所以这里以「各族请求真的通」为准 |
| 生产页面 = 仓库源码 | ✅ | 用 `go run` 生成 `dashboard.Page("cap-opencode-go","chat")` 与生产 `GET /v0/resource/plugins/cap-opencode-go/dashboard` **字节一致**（各 96 460 bytes，`diff` 退出码 0） |
| 品牌已在生产页面生效 | ✅ | 生产 HTML 含 `<link rel="icon" type="image/svg+xml" href="data:image/svg+xml;base64,…">`、`class="brand-mark"`（3 处）与「协议面」卡片标记 |
| 配置只被注释动过 | ✅ | sha256 `e4b5c63c…` → `28a8cf3f…`，inode 未变；新增内容就是 2 行 `# (v1.5.0: …)` 注释 |
| 无残留 | ✅ | `data/cap-opencode-go/` 仍然只有 `shared.json`；插件目录 21 个 `.so`（v1.2.0 / v1.3.0 / v1.4.0 / v1.4.1 / v1.5.0 四份 + 用量插件） |
| 官方验收脚本 | ✅ | **22 passed, 0 failed** |

```
$ CPA_BASE=http://127.0.0.1:8317 CPA_CLIENT_KEY=sk-… bash scripts/acceptance.sh
served models: chat=CatJam responses=gpt-6-luna gemini=gemini-4-argon messages=none

  PASS  chat/completions non-stream                    200  444 bytes
  PASS  chat/completions stream                        200  1202 bytes
  PASS  chat model via claude client                   200  274 bytes
  PASS  chat model via responses client                200  854 bytes
  PASS  responses non-stream                           200  2854 bytes
  PASS  responses stream                               200  11153 bytes
  PASS  responses model via chat client                200  540 bytes
  PASS  responses model via claude client              200  2076 bytes
  PASS  gemini generateContent                         200  574 bytes
  PASS  gemini streamGenerateContent                   200  1202 bytes
  PASS  gemini model via chat client                   200  374 bytes
  SKIP  messages family                                no messages model enabled
  PASS  /v1/models catalogue                           16 models
  PASS  dashboard state resource … PASS  state payload hides secrets

== 22 passed, 0 failed ==
```

（生产实例没有可用的管理密钥明文，故新增的 4 项品牌检查在生产运行时跳过；它们由隔离实例覆盖。）

### 2. 隔离实例：30 passed, 0 failed

隔离实例 `127.0.0.1:8399`：v8.0.27 + 插件 v1.5.0，配置同生产（同一个 Key、同一批模型设置），
额外把唯一的 messages 模型 `claude-haiku-5-5` 打开、把偶发上游地域限制的 `cat-jam-free` 关掉，
因此四个协议族全部被真实请求覆盖：

```
served models: chat=DeepSeek-V4.1-Flash responses=gpt-6-luna gemini=gemini-4-argon messages=claude-haiku-5-5

  PASS  chat/completions non-stream / stream · chat model via claude client · chat model via responses client
  PASS  responses non-stream / stream · responses model via chat client · responses model via claude client
  PASS  gemini generateContent / streamGenerateContent · gemini model via chat client
  PASS  messages non-stream / stream
  PASS  /v1/models catalogue                           16 models
  PASS  sidebar menu entry                             OpenCode Go /v0/resource/plugins/cap-opencode-go/dashboard
  PASS  plugin faces                                   cap-opencode-go(-gemini|-messages|-responses)
  PASS  plugin icon                                    same data:icon on all four rows
  PASS  plugin names                                   OpenCode Go / · Gemini / · Messages / · Responses
  PASS  plugin config                                  main 14 fields, siblings 0
  PASS  dashboard state resource · live model list resource (37) · quota resource · management protocol probe
  PASS  dashboard page: 账号管理 tab / 模型 tab / Key 余量列表 / 保存密码弹窗 / editable without a credential
  PASS  state carries the editable settings · state payload hides secrets

== 30 passed, 0 failed ==
```

四行品牌检查各自证明什么：

| 检查 | 证明 |
|---|---|
| `plugin faces` | `/v8/management/plugins` 里四个 id 齐全（不是三个也不是五个） |
| `plugin icon` | 四行的 `logo` 都以 `data:image/svg+xml;base64,` 开头 —— 同一个内嵌 OpenCode mark |
| `plugin names` | `metadata.name` 恰好是 `OpenCode Go` / `· Responses` / `· Messages` / `· Gemini` |
| `plugin config` | 主面 14 个配置字段、三个从属面 0 个 —— 「只有主插件要配置」在接口层成立 |

### 3. 无头浏览器实测（真实管理中心 + 插件 iframe）

隔离实例上用 Playwright 打开 `management.html` → 登录 → 「插件管理」→ 侧边栏 `OpenCode Go`：

| 断言 | 结果 | 证据 |
|---|---|---|
| 插件管理四行都显示 OpenCode 图标 | ✅ | 四个 `article` 各有一个 36px 图标盒，`img.src` 是 data URI 且 `img.complete && naturalWidth > 0`（真的解码成图，不是坏图占位） |
| 四行名称一眼可辨 | ✅ | 行标题依次 `OpenCode Go`、`OpenCode Go · Gemini`、`OpenCode Go · Messages`、`OpenCode Go · Responses` |
| 侧边栏入口同源同图 | ✅ | 侧边栏 `OpenCode Go` 链接内 `<img src="data:image/svg+xml;base64,…">`，同样加载成功 |
| 仪表盘品牌 | ✅ | `<title>OpenCode Go</title>`、`.brand-mark svg` 实测 26×26 CSS px、`link[rel=icon]` 是同一 data URI |
| 「协议面」卡片 | ✅ | 四张卡：主面标「配置入口」，从属面标「零配置」；每卡显示协议（`OpenAI Chat Completions` / `OpenAI Responses（Codex）` / `Anthropic Messages` / `Google GenAI`）、声明格式（`openai` / `codex + openai-response` / `claude` / `gemini`）、上游端点、插件 ID、provider、承载模型数与示例 id，以及"缺失即该族模型不可路由"的说明 |
| 手机端 | ✅ | 390×844：协议面单列（`grid-template-columns` 只有 1 列）、4 张卡、无横向溢出；工具栏图标 26px，「配置入口」标签不换行挤压 |
| 页面无脚本错误 | ✅ | `pageerror` 与 `console.error` 均为空（桌面与手机两次登录都没有） |

### 4. 截图

`docs/screenshots/`：`plugin-list.png`（插件管理四行 + 图标）、`desktop-accounts.png`、
`desktop-models.png`、`desktop-status.png`（协议面卡片）、`desktop-settings.png`、
`desktop-save-password.png`、`mobile-accounts.png`、`mobile-models.png`、`mobile-model-settings.png`、
`mobile-save-password.png`、`mobile-status.png`（手机端协议面）。

### 5. 顺带坐实的上游事实（本轮为决策做的探测）

合并成一个 `.so` 的前提是"插件自己会翻译"，否则跨协议请求必须被上游接受。2026-10 直接对
`https://opencode.ai/inference/go/v1` 逐个组合探测（每组合一次极小真实请求，带
`x-opencode-session` 与 `User-Agent: Go-http-client/1.1`，否则 Cloudflare 会先挡）：

| 模型（协议族） | `/chat/completions` | `/responses` | `/messages` | GenAI |
|---|---|---|---|---|
| `deepseek-v4.1-flash`（chat） | 200 | 200 | 200 | **400** |
| `claude-haiku-5-5`（messages） | **400** | **400** | 200 | **400** |
| `grok-4.7`（responses） | **400** | 200 | **400** | **400** |
| `gemini-4-argon-contributor`（gemini） | **400** | **400** | **400** | 200 |

失败一律是 `400 ModelProtocolUnsupported`。即：chat 族通吃 chat/messages/responses，
专有协议模型只吃自己那一种，gemini 协议只对 gemini 族开放 —— 所以"单文件合并 + 宿主直通"会让
上表每个 400 组合从"现在能用"变成"客户端报错"，只剩"在插件里重写协议转换"一条路（约 3.5 万行，
且被 `internal/` 挡住无法复用）。结论与取舍记录在 [design.md](design.md) §2.1–2.2。

---

## v1.4.1 仪表盘验收（生产实例热重载 + 隔离实例 + 无头浏览器）

本轮把仪表盘改成 Tab 结构（默认「账号管理」看各 Key 剩余额度）、保存按钮按未保存改动高亮、
取消「只读模式」（改成保存时按需输入 WebUI 密码），保存从「整块覆盖」改为「补丁合并」。

### 1. 生产实例：热重载部署，进程未重启

```
# 装新版本（旧版本文件留着，随时可回滚）
$ CPA_ROOT=/root/cliproxyapi bash scripts/install.sh 1.4.1
installed /root/cliproxyapi/plugins/linux/amd64/cap-opencode-go-v1.4.1.so
installed …-responses-v1.4.1.so   installed …-messages-v1.4.1.so   installed …-gemini-v1.4.1.so

# 原地补一行注释（保持 inode），fsnotify → ApplyConfig → 按版本号选中最新的 .so
$ python3 /tmp/touch_config.py
config rewritten in place
$ ls -i config.yaml
10653636 config.yaml        # inode 前后一致，监听器没有跟丢
```

| 断言 | 结果 | 证据 |
|---|---|---|
| 热重载到 1.4.1 | ✅ | `/api/state` → `version=1.4.1`（改前 `1.3.0`），37 个模型 id 不变，Key 池不变；生产实际部署顺序是先 1.4.0、随后为带上竞态修复再热重载到 1.4.1，两次都是同一个进程 |
| 进程未重启 | ✅ | PID `3194299` 全程不变（`ELAPSED` 从 2h07m 连续增长），管理页面会话没有中断 |
| 页面换成 Tab 版 | ✅ | `GET /v0/resource/plugins/cap-opencode-go/dashboard` 含 `data-tab="accounts"`、`id="keyList"`、`id="credModal"`，且**不再含**「只读模式」字样 |
| 只读接口带上可编辑字段 | ✅ | `/api/state` 新增 `timeout_seconds` / `key_cooldown_seconds` / `max_retries` / `debug` / `state_dir` |
| 额度接口覆盖全部 Key | ✅ | `/api/quota` → `[('main', True, ['5 小时','本周','本月'])]`，并新增 `enabled` 字段 |
| 官方验收脚本 | ✅ | **22 passed, 0 failed**（messages 协议族因操作者关闭了全部 messages 模型而 SKIP） |

```
$ CPA_BASE=http://127.0.0.1:8317 CPA_CLIENT_KEY=sk-… bash scripts/acceptance.sh
served models: chat=CatJam responses=gpt-6-luna gemini=gemini-4-argon messages=none

  PASS  chat/completions non-stream                    200  456 bytes
  PASS  chat/completions stream                        200  975 bytes
  PASS  chat model via claude client                   200  272 bytes
  PASS  chat model via responses client                200  968 bytes
  PASS  responses non-stream                           200  2844 bytes
  PASS  responses stream                               200  11584 bytes
  PASS  responses model via chat client                200  530 bytes
  PASS  responses model via claude client              200  2042 bytes
  PASS  gemini generateContent                         200  571 bytes
  PASS  gemini streamGenerateContent                   200  1202 bytes
  PASS  gemini model via chat client                   200  373 bytes
  SKIP  messages family                                no messages model enabled
  PASS  /v1/models catalogue                           16 models
  PASS  dashboard state resource
  PASS  live model list resource                       upstream 37 models
  PASS  quota resource
  PASS  dashboard page: 账号管理 tab
  PASS  dashboard page: 模型 tab
  PASS  dashboard page: Key 余量列表
  PASS  dashboard page: 保存密码弹窗
  PASS  dashboard page: editable without a credential
  PASS  state carries the editable settings
  PASS  state payload hides secrets

== 22 passed, 0 failed ==
```

（脚本新增的 7 项就是上面这些 UI 断言；它用 bash 的 `[[ … == *marker* ]]` 匹配而不是
`printf | grep -q`：页面比管道缓冲区大，`grep -q` 提前退出会被 `pipefail` 记成失败。）

### 2. 页面实测（无头 Chromium，驱动真实管理中心 iframe）

隔离实例 `127.0.0.1:8318`（v8.0.27 + 插件 v1.4.x、模型配置与生产一致），
用 Playwright 打开 `management.html` → 登录 → 点侧边栏 `OpenCode Go`：

| 断言 | 结果 | 证据 |
|---|---|---|
| 默认 Tab 是「账号管理」 | ✅ | 首屏 `aria-selected=true` 的 Tab = `accounts`；`#keyList` 直接列出 `main oc_sk_cd…q2N4` |
| 看得到各 Key 剩余额度 | ✅ | `5 小时 剩余 98%（已用 2% · 22 分钟后重置）`、`本周 剩余 93%`、`本月 剩余 82%`；汇总条 `最低剩余 82% main · 本月` |
| 保存按钮只在有改动时亮 | ✅ | 干净时 `class="save-btn"` + `disabled`；改一个别名 → `class="save-btn is-dirty"`、角标 2、chip「2 项未保存」、模型 Tab 出现小圆点 |
| 没有只读模式 | ✅ | 页面无「只读」字样；未登录到管理中心凭证时仍可编辑，点保存才弹框 |
| 没勾「记住密码」→ 弹密码框 | ✅ | 点保存 → `#credModal` 显示；输入错误密码 → 框内提示「WebUI 密码不正确（HTTP 401）。连续输错 5 次会临时封禁当前 IP」（只失败 1 次）；输入正确密码 → 保存成功，toast「已保存 2 项修改」 |
| 勾了「在本标签页内记住」→ 不再问 | ✅ | 同一会话第二次保存（切换模型开关）无弹框直接成功 |
| 勾了管理中心「记住密码」→ 不弹框 | ✅ | 另一浏览器上下文登录时勾选 → `localStorage['cli-proxy-auth']` 携带密钥（未勾时同一位置为空）→ 保存直接成功 |
| 增删 Key 是合并写 | ✅ | 加 `backup`（权重 3）→ config.yaml 出现该 Key 且 `main` 的明文 Key 原样保留；插件热重载后 `/api/state` 的 `keys` = `[backup:3, main:1]`；删除后再保存 → 配置只剩 `main`（明文未变） |
| 顶层字段同样是合并写 | ✅ | 只改「请求超时 → 777」，config.yaml 里 `request-timeout-seconds: 777`，`base-url` / `default-proxy` / `models.*` 全部原样 |
| 页面无脚本错误 | ✅ | 生产页面在无头浏览器里 `pageerror` / `console.error` 均为空，模型 Tab 渲染 37 行 |

### 3. 隔离实例全量验收

```
$ CPA_BASE=http://127.0.0.1:8318 CPA_CLIENT_KEY=sk-test-… CPA_MGMT_KEY=test-… bash scripts/acceptance.sh
  PASS  chat/completions non-stream … PASS  management protocol probe
  PASS  dashboard page: 账号管理 tab … PASS  state payload hides secrets
== 26 passed, 0 failed ==
```

### 4. 截图

`docs/screenshots/`：`desktop-accounts.png`（默认 Tab）、`desktop-models.png`、`desktop-status.png`、
`desktop-settings.png`、`desktop-save-password.png`、`mobile-accounts.png`、`mobile-models.png`、
`mobile-model-settings.png`、`mobile-save-password.png`（390×844，密码框为底部弹层）。

### 5. 本轮踩到的宿主侧陷阱（已写入 README / rollback）

- **不要覆盖正在使用的 `.so`**：隔离实例上用 `cp` 原地覆盖已 `dlopen` 的库，
  宿主随即 SIGSEGV 退出（日志停在崩溃前最后一条请求）。升级必须新增版本号文件。
- 同上，删除正在使用的 `.so` 对运行中的进程无害（inode 仍在），但**下一次重载/重启就会退到旧版本**，
  所以升级时旧文件要留、新文件要装，别在中间态停留。

> v1.3.0 在生产实例 127.0.0.1:8317 实测：**15 passed, 0 failed**（messages 协议族仍因操作者
> 关闭了 `claude-haiku-5-5` 而 SKIP）。该轮把别名从「额外注册一个 id」改成**重命名**，
> 重点验收「模型列表与路由都只认别名」，并顺带验证了**不重启服务的配置热重载**。

---

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
