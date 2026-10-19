# 验收记录（CLIProxyAPI v8.0.27 / 插件 v1.7.1 基线，v1.7.2–v2.0.0 热重载补充）

> v1.7.1（含 1.7.0 的统计口径修正与历史回放；1.7.1 只修两处页脚注/空状态文案）
> 在生产实例 127.0.0.1:8317 重启到 1.7.1：四个面均加载 `-v1.7.1.so`，`plan=go`，PID `3651300`；
> 1.7.0 阶段回放 5,628 条历史记录（366,045,455 tokens，$27.835431 刊例价，4,531 条 TTFT）；
> 1.7.1 公开验收 **32 passed, 0 failed**（管理密钥不可用，6 项管理检查跳过；messages 无启用模型，1 项跳过）。
> `go test ./...`、`go vet ./...`、`go test -race ./internal/...` 通过。
> 本轮没有重跑隔离实例的 40 项，因此不以旧隔离结果代替 1.7.1；真实上游请求的 chat /
> responses / gemini 路径由生产公开验收覆盖。
> 2026-10-17 03:16 UTC 起又以 `config.yaml` 原地增注释热重载到 1.7.2 / 1.7.3，**PID `3651300` 不变、没有重启**；
> 1.7.2 新增生成 TPS 指标，1.7.3 修正覆盖率；API `avg_gen_tps/gen_tps_samples`、页面「生成 TPS」列
> 和一次真实 Beam 流式请求已验证。下面的 32 项完整验收仍是 1.7.1 的结果，未把 1.7.3 的热重载检查
> 冒充成完整验收。
> 2026-10-17 03:34 UTC 再热重载到 1.7.6，**PID `3651300` 仍不变、没有重启**；1.7.6 在统计页新增
> 价目核对日期与「数据该更新了」周期提醒，四个面均映射到 `-v1.7.6.so`。API、页面常驻状态和
> 到期横幅（Chromium 拦截 `/api/stats` 模拟第 30 天）已验证；统计文件 schema 没有变化。
> 2026-10-17 03:46 UTC 再热重载到 1.7.8，**PID `3651300` 仍不变、没有重启**；1.7.8 在同一个提醒里
> 加入上游模型列表变更：四个面均映射到 `-v1.7.8.so`，API 返回 `model_list_*`，模型列表新增/移除
> 由 Chromium 拦截模拟验证。完整公开验收仍以 1.7.1 的 32 项为准。
> **2026-10-17 06:03 UTC 热重载到 1.8.0**（配额管理 → Plugin + 账号投影），**PID `3651300`、启动时间
> `03:07:04 UTC` 均未变**，inode 不变；四个面均映射到 `-v1.8.0.so`，生产公开验收 **36 passed, 0 failed**
> （新增 2 项账号检查；管理密钥仍不可用，管理 API 检查在隔离实例上以同一份二进制验证）。
> **06:41 UTC 热重载到 1.8.1** 修复 1.8.0 的仪表盘脚本语法错误（页面能加载但 Tab/数据全失效），
> **PID `3651300` 仍不变**；线上页面脚本 `node --check` 通过，生产公开验收 **37 passed, 0 failed**
> （新增 `dashboard script parses`）。
> **2026-10-17 07:23 / 07:26 UTC 热重载到 1.9.0 → 1.9.1**（仪表盘 Key 改名），**PID `3651300`、
> 启动时间 `03:07:04 UTC`、config inode 均未变**；四个面 `active_version=1.9.1 retired_version=1.9.0`。
> 生产公开验收 **39 passed, 0 failed**（messages 无启用模型跳过 1 项；新增 2 项页面改名标记）。
> 宿主 `static/management.html` 哈希未变，未关闭面板自动更新，未改宿主配置。
> 下面是 v1.7.8 / v1.7.6 的热重载记录，之后是 v1.7.1 记录与更早历史（v1.6 统计面板、v1.5.0 品牌整合、v1.4.1 仪表盘改造等）。
> **2026-10-17 14:03 UTC 热重载到 1.11.0**（账号投影瘦身：每 Key 4 个认证文件 → 1 个），
> `MainPID=4128447`、`ActiveEnterTimestamp=10:02:25 UTC`、config inode `10621868` 均不变；
> 四个面 `active_version=1.11.0`，auth 目录收敛为 1 个 chat 面文件，生产公开验收
> **39 passed, 0 failed**（messages 无启用模型跳过 1 项）；隔离实例 **50 passed, 0 failed** 覆盖四族。
> 详见下面第一节。
> **2026-10-17 15:06 / 15:14 UTC 热重载到 1.12.0**（移除账号投影与宿主配额集成），
> `MainPID=4128447`、`ActiveEnterTimestamp=10:02:25 UTC`、config inode `10621868` 均不变；
> 四个面 `active_version=1.12.0`（gemini 首次安装被 `head -6` 截断，15:14 补装后重载），
> 生产公开验收 **37 passed, 0 failed**（messages 无启用模型跳过 1 项；删除了 5 项账号/配额检查），
> 隔离实例（messages 启用、auth 目录为空）**45 passed, 0 failed**。详见下面第一节。

---

## v2.0.0 更名 cpa-opencodego + 请求日志 + 新协议对齐（生产热重载，未重启）

> **2026-10-19 03:55 UTC 生产热重载到 2.0.0**：`MainPID=179655`、config inode
> `10621868` 均不变，**没有重启进程**。旧 `cap-opencode-go*` 四个面被 unload，
> 新 `cpa-opencodego*` 四个面以 `active_version=2.0.0` 注册；旧插件资源路由返回 404。
> 生产公开验收 **48 passed, 0 failed**（messages 没有启用模型，1 项 SKIP）。

### 1. 交付与变更
- 插件 id / `.so` / GitHub 仓库 / module 统一改为 `cpa-opencodego`；provider key 保持
  `opencode-go*`，模型 id 与别名不变，DSH 侧无需改动。
- 状态目录从 `data/cap-opencode-go` 迁移到 `data/cpa-opencodego`，旧目录保留；
  生产统计历史连续（迁移后 `/api/stats` 仍为 12 个模型 / 12,222 次请求）。
- 新增「请求日志」Tab 与 `/api/requests`；四族各自写 `requests-<role>.jsonl`。
- 协议对齐：支持 `{go}/openai/v1`、`{go}/anthropic/v1`、`{go}/google/v1beta`
  新端点；旧 `/inference/go/v1` 与 `/zen/go/v1` 继续兼容（生产当前即 legacy 布局）。
- `developer` 角色在 OpenAI 形状请求发送前归一化为 `system`；UA 从入站请求透传，
  插件不合成 UA，只自行发送 `x-opencode-session`。

### 2. 构建与静态检查
- `go test ./...` 全绿；`go vet ./...` 无问题；`gofmt -l` 为空。
- `make build VERSION=2.0.0` 产出四个 `dist/cpa-opencodego*-v2.0.0.so`（各 8.3 MB）。
- `bash -n scripts/{install,uninstall,acceptance,migrate-v1}.sh` 通过。

### 3. 隔离实例（8399，独立 config/auth/data）
- 以生产配置副本 + 迁移脚本演练升级：四个 `cpa-opencodego*` 注册成功，旧 id 不加载。
- **legacy 布局**（base-url 保持 `.../inference/go/v1`）：chat / responses / messages /
  gemini 四族各一次真实请求全部 **200**。
- 原地把 base-url 改为 `https://opencode.ai/inference/go` 触发热重载：`/api/state` 返回
  `endpoint_layout=family`，`endpoints` 为四族新 URL；四族再次全部 **200**。
- `/api/requests` 合并四族文件：非流式记录 token、总耗时与端到端 TPS；流式 chat 记录
  **TTFT 1539.8 ms、生成 TPS 215.1、端到端 TPS 19.33**，并带上游
  `x-opencode-log-id`；`?family=messages` 过滤正确；统计页不受影响。

### 4. 生产热重载与真实请求
- `scripts/install.sh 2.0.0` 删除旧 `.so` 并安装四个新库；`scripts/migrate-v1.sh --apply`
  改名四个配置块、备份 `config.yaml`、复制状态文件；原地写配置触发 fsnotify 热重载。
- `main.log`：`plugin unloaded ... cap-opencode-go*` → `plugin loaded ... cpa-opencodego* version=2.0.0`。
- 真实请求（含 `role=developer`）非流式 **200**，`/api/requests` 记录 41/16 tokens、
  端到端 TPS 8.62、上游日志 ID；流式 **200**，TTFT 1355.8 ms、生成 TPS 198.05、
  端到端 TPS 36.09。
- 页面标记全部存在：「请求日志」Tab、请求表、生成/端到端 TPS 列、端点布局、
  developer 角色、请求日志开关；`node --check` 通过。
- 公开验收 **48 passed, 0 failed**；请求日志资源 13 行；统计资源
  `matched=6594 requests=12222 tokens=1251934606 ttft=10775 tps=11652`。

### 5. 已知边界
- messages 族在生产配置中没有启用模型，因此公开验收的 messages 检查 SKIP；
  隔离实例已单独覆盖 messages 200。
- 请求日志是插件自测口径，和「统计」的宿主 usage 口径可能略有差异；
  两者都展示各自样本数，避免误读。
- 旧 `cap-opencode-go*` 配置块与状态目录仍保留在磁盘，回滚见
  [docs/UPGRADE-v2.md](UPGRADE-v2.md) 与 [docs/rollback.md](rollback.md)。

---

## v1.12.0 移除账号投影与宿主配额集成（生产热重载 + 隔离实例 + 回滚演练）

### 1. 交付

按操作者要求（“不用注册认证文件和配合管理了”）整体移除 v1.8 引入的宿主侧集成：

- 删除 `internal/accounts` 整包、`internal/plugin/accounts.go`、`internal/plugin/quota_provider.go`
  及其测试；`Runtime` 不再保存 host auth 目录，`plugin.register` / `model.static` / `/api/state`
  不再触发账号同步，`plugin.quiesce` 与 `Shutdown` 不再有清理语义（`Shutdown` 只 flush 统计）。
- 主面不再声明 `quota_provider`，不再应答 `quota.identifier|describe|fetch|reset`；
  `internal/quota.FetchKey` 收回为包内 `fetchKey`（只剩仪表盘余量在用）。
- 删除配置项 `register-accounts` 与 `AccountsEnabled()`；旧配置里的该字段被宽松 YAML 忽略，
  新增回归用例 `TestRemovedRegisterAccountsKeyIsIgnored` 固定这一行为。
- `/api/state` 不再有 `accounts` 字段，`/api/quota` 不再有 `accounts` 键；仪表盘删除「CPA 账号」
  统计行、Key 卡片小字与「关于」里的投影说明；`scripts/acceptance.sh` 删除 5 项账号/配额检查；
  `scripts/uninstall.sh` 保留 marker 扫描作为 v1.8–v1.11 遗留文件的一次性兜底。
- `go test ./...`、`go vet ./...`、`gofmt -l internal cmd` 全绿；页面脚本 `node --check` 通过
  （验收里的 `dashboard script parses`）。

### 2. 先验证“零 auth 也能工作”（1.11.0，隔离实例）

删除前先用 1.11.0 + `register-accounts: false` 在隔离实例上把 auth 目录清空：

- 宿主日志 `5 clients (0 auth entries + … + 5 OpenAI-compat)`，`/v1/models` 仍返回 13 个模型；
- 四族流式/非流式 12/12 协议检查 PASS（chat=Beam、responses=gpt-6-luna、gemini=gemini-4-argon、
  messages=claude-haiku-5-5）；只有账号投影与宿主配额那几项 FAIL/SKIP，而它们正是本次要删除的检查。
- 结论：路由是插件自己的 `model_router`（`TargetKind=self`），宿主 auth 只服务于配额页展示。

### 3. 隔离实例（1.12.0，messages 启用，auth 目录为空）

- 先放一个 v1.11 格式的遗留文件 `auth/opencode-go-legacy-key.json`：插件**不创建也不删除**它
  （与 1.11 的 quiesce/shutdown 清理语义不同），宿主把它列为 `provider=opencode-go` 凭据；
- `/api/state` 无 `accounts` 字段；`/api/quota` 只有 `keys`，每 Key 仍有 3 个窗口（真实上游）；
  `/v8/management/plugins` 四个面 `supports_quota=false`、`quota_provider` 为空，主面 13 个配置字段；
- 删掉遗留文件后 `/v8/management/credentials` 在约 6 秒内从 1 条变成 0 条（宿主 fsnotify），
  插件继续服务 13 个模型；
- 完整验收 **45 passed, 0 failed**（原 50 项减去 5 项账号/配额检查）。

### 4. 生产热重载（PID / 启动时间 / inode 均不变）

- `VERSION=1.12.0 make install` 后原地给 `config.yaml` 追加注释触发重载：
  `MainPID=4128447`、`ActiveEnterTimestamp=10:02:25 UTC`、config inode `10621868` 全部不变；
- `main.log`：`15:06:48` 热重载 chat / responses / messages 到 `active_version=1.12.0`；
  gemini 的 `.so` 因安装命令被 `head -6` 截断而没装上，15:14 补装后再次热重载
  （`15:14:37 … cpa-opencodego-gemini active_version=1.12.0 retired_version=1.11.0`）；
  最终 `/proc/4128447/maps` 里四个 `-v1.12.0.so` 全部加载；
- 热重载后删除遗留的 `~/.cli-proxy-api/opencode-go-user-example-com.json`（先确认
  `managed_by: cpa-opencodego`），auth 目录不再有 opencode-go 文件；
- 生产公开验收 **37 passed, 0 failed**（messages 无启用模型跳过 1 项），四族照常工作；
  由于环境里的管理密钥是哈希值，管理 API 检查仍跳过，能力变化由隔离实例的同一份二进制验证。

### 5. 回滚演练（隔离实例）

- 把隔离实例的 `register-accounts` 改回 `true` 并装回 1.11.0：启动后 auth 目录重新出现
  `opencode-go-user-example-com.json`，`/api/state` 又带 `accounts`（旧版本行为，预期）；
- 覆盖安装 1.12.0 并热重载：文件**仍然存在**（1.12 不清理，需要手动删一次），
  `/api/state` 恢复无 `accounts`；此时该凭据 `supports_quota` 为空，不会出现在「配额管理 → Plugin」；
- 手动删除后凭据列表清空，插件继续服务 13 个模型。

### 6. 未做与预期结果

- 没有修改宿主、没有关闭管理面板自动更新、没有改 `config.yaml` 的结构（只追加注释）；
- 「配额管理 → Plugin」不再列出 OpenCode Go、「认证文件」页不再有 `opencode-go-*` 行，这是本次
  移除的**预期结果**；余量仍在插件仪表盘「账号管理」Tab（直查上游 `/usage`）。

---

## v1.11.0 账号投影瘦身：每 Key 4 个认证文件 → 1 个（生产热重载 + 隔离实例）

### 1. 交付

- **背景**：「配额管理 → Plugin」需要宿主 auth 记录，而「认证文件」页与配额页共用同一份
  `/v8/management/credentials` 列表，宿主 v8.0.27 没有“只对配额页可见”的隐藏标记；完全隐藏与
  保留配额页不可兼得。v1.10.2 起路由已改为插件自执行（`TargetKind=self`），
  `--responses/--messages/--gemini` 三个投影文件只剩「认证文件」页的显示噪音。
- `internal/accounts.DefaultRoles` 改为只投影 chat 面（`opencode-go-<key 名>.json`）；旧后缀文件
  由现有“不在期望集合的 marker 文件即删除”逻辑在第一次同步删除，没有新增版本检测或迁移代码。
- 投影文件的 `note` 改为「…；仅用于「配额管理 → Plugin」显示」；`register-accounts` 语义不变
  （关闭后不自动删除已生成文件）。
- README §5/§6/§8、`docs/design.md` §7.9.3、`docs/rollback.md` 与仪表盘「关于」同步说明
  为什么认证文件页仍会看到 1 行/Key。

### 2. 测试

```
$ gofmt -l internal cmd        # 无输出
$ go vet ./...                 # 通过
$ go test ./...                # 全绿
```

- `internal/accounts`：`TestPlanBuildsStableMirrorFiles` 期望 2 项（2 Key × 1 面）；
  新增 `TestSyncRemovesStaleRoleSuffixFiles`（4 个 marker 文件 → 只剩 chat 文件，`removed=3`）。
- `internal/plugin`：账号同步用例从 8/4 个文件改为 2/1 个。
- `scripts/acceptance.sh`：`account files` 只接受 `type=opencode-go`，旧后缀文件残留会直接 FAIL。

### 3. 隔离实例（v8.0.27 同版本二进制，端口 8399，独立 auth-dir）

启动前在 auth 目录预置旧版 4 个 marker 文件（chat + `--responses/--messages/--gemini`）：

```
启动后 auth 目录: opencode-go-user-example-com.json 一个（0600，managed_by，note 新文案）
state: version=1.11.0 accounts.files={"user@example.com":"opencode-go-user-example-com.json"}
credentials: 1 条 provider=opencode-go / supports_quota=true / quota_provider=opencode-go /
             quota_plugin_id=cpa-opencodego / auth_index 有值
quota fetch: five_hour 0.93 / weekly 0.34 / monthly 0.52（真实上游 /usage，Go 套餐）
```

公开验收（真实上游）：

- 首次（messages 未启用）**48 passed, 0 failed, 1 skipped**；
- 打开 `claude-haiku-5-5`（messages）后复跑 **50 passed, 0 failed**，四族流式/非流式全部覆盖
  （chat=Beam、responses=gpt-6-luna、gemini=gemini-4-argon、messages=claude-haiku-5-5）。

回滚演练（同一隔离实例）：

- 换回 v1.10.3 四个 `.so` + 原地改 config：`version=1.10.3`，auth 目录自动恢复 4 个文件，
  credentials 4 条（只有 chat 带 `supports_quota`）——旧版本行为；
- 再换回 v1.11.0：`version=1.11.0`，auth 目录重新收敛到 1 个文件。

### 4. 生产热重载（127.0.0.1:8317）

- 14:02 UTC 安装 `-v1.11.0.so` 四个文件；14:03 UTC 原地追加 config 注释触发 hot reload。
- `MainPID=4128447`、`ActiveEnterTimestamp=10:02:25 UTC`、config inode `10621868` 均不变；
  四个面 `active_version=1.11.0`（`/proc/4128447/maps` 同时保留退役的 1.10.3 映射）。
- auth 目录由 4 个 `opencode-go-*` 文件收敛为 1 个 `opencode-go-user-example-com.json`
  （`--responses/--messages/--gemini` 已删除），文件带新 `note`。
- 生产公开验收 **39 passed, 0 failed, 1 skipped**（messages 无启用模型）：`account projection`
  1 个账号、`account files` 1 个文件（0600 + marker）、四族请求除 messages 外全部 PASS。
- 管理密钥在 `config.yaml` 里是 bcrypt 哈希、明文不可得，5 项管理检查未在生产跑；隔离实例上
  用同一份 1.11.0 二进制全部 PASS（plugin list、quota provider、credentials 关联、quota fetch、
  管理协议探测）。
- 未改宿主文件/配置，宿主管理面板自动更新保持开启。

---

## v1.9.1 仪表盘 Key 改名（热重载；含 1.9.0 池快照修复）

### 1. 交付

- 已有 Key（含 `main`）的名称改为可编辑输入框：回车/失焦生效，Esc 放弃；空名、与已有 Key
  或待新增 Key 重名会被拒绝。改名以 `edits.keys.rename` 独立记录，`buildPayload` 按
  “删除 → 改权重/启用 → 改名 → 新增”合并，保存按钮只改名字也会点亮。
- 改名保存后 `internal/accounts` 删除旧 `opencode-go-<旧名>.json`、创建新投影；
  「配额管理 → Plugin」的账号名与余量随之更新。
- 1.9.1 修复 `keys.Pool.Replace`：secret 不变时保留健康状态，但旧实现连 `state.Name` 一起保留，
  导致宿主 `plugin.reconfigure` 后 `/api/models` 仍显示旧名（配置、配额、账号文件已是新名）。
  现在以配置的 `key.Name` 为准，只保留成功/失败/冷却等健康字段。

### 2. 测试

```
$ gofmt -l .            # 无输出
$ go vet ./...          # 通过
$ go test ./...         # 全绿（internal/dashboard、internal/accounts、internal/keys 等）
```

- `TestPageRenamesKeys`：页面存在改名输入、校验函数、payload 改名顺序。
- `TestPageRenameLogic`：Node VM + DOM stub 执行页面真实内联脚本，覆盖空名/同名/重名、
  改名后改权重、改名 + 新增同名 Key、`buildPayload` 顺序；无 node 时跳过。
- `TestSyncRenamesMirrorFileWhenKeyNameChanges`：旧投影删除、新投影创建、marker/名称正确。
- `TestReplacePreservesHealth`：改名后健康计数保留，且快照名称必须变为新名。
- `TestPageScriptParses` 与验收脚本 `dashboard script parses`：线上页面脚本 `node --check` 通过。

### 3. 隔离实例端到端（v8.0.27 同版本二进制，端口 8399）

1.9.0 阶段：`GET/PUT /v8/management/config/plugins/configs/cpa-opencodego` 把 `main` 改成 `primary`，
返回 `{"config-version":8,"status":"ok"}`；auth 目录只剩 `opencode-go-primary.json`（`name=primary`、
`managed_by`、0600），`/v8/management/credentials` 仍是 `provider=opencode-go` / `supports_quota` /
`quota_provider` / `quota_plugin_id=cpa-opencodego`。页面 `data-key-rename` / `id="keyNameHint"` /
`node --check` 均通过。

但同一实例的 `/api/models` 仍返回 `keys=["main"]`（`/api/quota` 已是 `primary`），暴露出池快照
旧名问题。1.9.1 重建隔离实例后复验：

```
models: version=1.9.1 keys=["primary"] accounts={"primary":"opencode-go-primary.json"}
quota:  keys=["primary"] accounts={"primary":"opencode-go-primary.json"}
auth:   only opencode-go-primary.json (name=primary, managed_by=cpa-opencodego, 0600)
cred:   opencode-go-primary.json / quota_provider=opencode-go / quota_plugin_id=cpa-opencodego
```

### 4. 生产热重载与验收（127.0.0.1:8317）

- 07:23 UTC：1.9.0 四文件安装后原地追加 config 注释，四个面 `plugin hot reloaded
  active_version=1.9.0 retired_version=1.8.1`。
- 07:26 UTC：1.9.1 四文件安装后同样热重载，四个面 `active_version=1.9.1 retired_version=1.9.0`；
  `MainPID=3651300`、`ActiveEnterTimestamp=03:07:04 UTC`、config inode `10653636` 均不变。
- 生产公开验收 **39 passed, 0 failed, 1 skipped**（messages 无启用模型）；新增
  `dashboard page: Key 改名提示` / `dashboard page: Key 改名输入`。
  1.9.0 的首次验收曾遇到两次上游 `/responses` EOF（502）；同一请求手工重试 3 次均 200，
  随后整套验收 39/0，故按上游瞬态处理，1.9.1 最终验收一次通过。
- 线上 `/api/state`：`version=1.9.1`，`keys=["main"]`，`accounts.files={"main":"opencode-go-main.json"}`；
  auth 文件 mtime 仍为 06:03:22（热替换未重建）。
- 线上页面 135,396 字节，含 `id="keyNameHint"` 与 `data-key-rename="`，内联脚本 `node --check` 通过。

### 5. 宿主配额页图标

按用户决定不改图标，只记录限制：管理面板的 `AUTH_FILE_ICONS` 按 `quota_provider` 选图，
`opencode-go` 未命中显示 `O`，「Plugin」标签显示 `P`；插件 `metadata.logo` 只用于插件列表、
侧边栏和插件页。本轮没有修改 `static/management.html`（sha256 仍为
`57558cd023475674634a114824b014d3e5ac161f6f3791e3d86b6adab962a700`），没有加入
`disable-auto-update-panel`，也没有改宿主配置。

## v1.8.1 仪表盘脚本语法修复（热重载 + 新增语法回归检查）

### 1. 故障与定位

- 现象：侧边栏点 OpenCode Go 后页面打不开、Tab 与按钮全部无效。
- 服务端一切正常：`GET /v0/resource/plugins/cpa-opencodego/dashboard` 200（`Cache-Control: no-store`），
  `/api/state`、`/api/quota`、`/api/models`、`/api/stats` 均 200 且响应在 2 秒内。
- 日志显示 06:33 三次加载 dashboard 之后**没有任何 `/api/*` 请求**——说明页面内联脚本没有执行。
- 把线上页面的 `<script>` 抽出来跑 `node --check`：

```
/live-script-0.js:918
      aboutRow('CPA 账号', '…下一次同步会按配置复原。'),',
                                                                 ^^
SyntaxError: Invalid or unexpected token
```

根因：`internal/dashboard/script.go` 的「CPA 账号」说明行末尾多了一个逗号（`'),',` 而不是 `'),`），
字符串未闭合 → 整个 81 KB 内联脚本无法解析 → 页面只剩静态骨架。1.8.0 的 `go test`/验收只做 HTML
文案子串断言，因此没拦住。

### 2. 修复与回归检查

- 删除多余的逗号；把线上页面重新抽取后 `node --check` 通过（修复前失败、修复后通过，逐字复现）。
- `internal/dashboard/page_test.go` 新增 `TestPageScriptParses`：抽取页面内联脚本写临时文件，
  用 `node --check` 解析；机器上没有 node 时 `t.Skip`。已实测「把逗号加回去 → 用例 FAIL 并打印
  行号与源码行；改回来 → PASS」。
- `scripts/acceptance.sh` 新增 `dashboard script parses`：对**已部署页面**做同样检查（node 缺失时 SKIP）。

### 3. 部署与验收（生产 127.0.0.1:8317）

- 06:41 UTC 热重载到 **1.8.1**：四个面 `hot reloaded … active_version=1.8.1 retired_version=1.8.0`；
  `MainPID=3651300`、`ActiveEnterTimestamp=2026-10-17 03:07:04 UTC`、config inode `10653636` 均未变。
- 账号投影在热替换后保留（`opencode-go-main.json` 未重写，`created/updated/removed = 0`，
  `accounts.synced=true`，`version=1.8.1`）。
- `/api/quota` 返回真实上游窗口（rolling 0% / weekly 61% / monthly 45%）。
- 线上页面脚本 `node --check` 通过；`grep "'),',"` 为 0，目标行恰好出现 1 次。
- 公开验收：`== 37 passed, 0 failed ==`，其中 `dashboard script parses` PASS、两项账号检查 PASS、
  messages 因无启用模型跳过 1 项（管理密钥不可用，管理 API 检查仍沿用隔离实例结果）。

---

## v1.8.0 CPA「配额管理 → Plugin」+ 账号投影（热重载 / 隔离实例 / 零残留）

### 1. 部署（生产，不重启）

- 06:03:18 UTC 备份 1.7.8 的四个 `.so` 与 `config.yaml` 到
  `/root/cliproxyapi/backup-1.8.0-pre-20261017T060318Z`。
- `CPA_ROOT=/root/cliproxyapi bash scripts/install.sh 1.8.0` 只新增
  `cpa-opencodego{,-responses,-messages,-gemini}-v1.8.0.so`，不覆盖在用文件。
- 原地追加一行注释触发 fsnotify：inode `10653636` 前后不变；
  `MainPID=3651300`、`ActiveEnterTimestamp=2026-10-17 03:07:04 UTC` 未变（没有重启）。
- 日志：四个面 `plugin hot reloaded … active_version=1.8.0 retired_version=1.7.8`，
  随后 `auth file changed (CREATE): opencode-go-main.json`。

### 2. 账号投影与配额

- `/root/.cli-proxy-api/opencode-go-main.json` 生成，`-rw-------`（0600），内容
  `type=opencode-go`、`name=main`、`api_key=oc_sk_cd1a…`、`disabled=false`、
  `managed_by=cpa-opencodego`。
- `/v0/resource/plugins/cpa-opencodego/api/state`：
  `version=1.8.0`、`accounts.enabled=true`、`accounts.auth_dir=/root/.cli-proxy-api`、
  `accounts.files={"main":"opencode-go-main.json"}`、`error` 为空。
  注意生产 `config.yaml` 没有顶层 `auth-dir`（只有 `oauth.auth-dir`），宿主下发的
  `host.auth_dir` 因此为空，插件按宿主同样的规则回落到 `~/.cli-proxy-api` —— 与实施前的
  `util.ResolveAuthDir("")` 一致，实测落点正确。
- `/v1/models` 仍是 16 个模型、无重复：镜像 auth 会为宿主提供同一批模型，
  但公开列表按模型 id 去重，不产生副本。

### 3. 隔离实例（同版本二进制，验证宿主侧契约与「零残留」）

实例：`/tmp/cpa-quota-test`（独立端口 8399、独立 auth-dir、假上游 `/usage`、自设管理密钥），
`/usage` 返回 rolling 32% / weekly 60% / monthly 45%。

- `GET /v8/management/plugins`：`cpa-opencodego` 的 `supports_quota=true`、
  `quota_provider=opencode-go`、`supports_oauth=false`；三个从属面均为 false（只主面声明）。
- `GET /v8/management/credentials`：`provider=opencode-go`、`supports_quota=true`、
  `quota_provider=opencode-go`、`quota_plugin_id=cpa-opencodego`、`auth_index` 有值。
  （`/v0/management/auth-files` 也返回同一份条目，但 v0 不带 `quota_plugin_id`，面板走 v8。）
- `POST /v8/management/plugins/cpa-opencodego/quota {"auth_index":…}` 返回 1 个 group +
  3 个 bucket：`five_hour` 0.68、`weekly` 0.4、`monthly` 0.55（= 1 - percent/100，已去掉浮点噪声），
  `resetTime` 与 `description` 正确，`subscription.plan=Go`。升级到 Go Plus 时显示 `Go Plus`。
- 热替换（1.8.1 → 1.8.2，同 ID 换 `.so` 路径）：旧实例收到 `plugin.quiesce`，
  **账号文件未被删除**，新实例接管；日志 `plugin hot reloaded … retired_version=1.8.1`。
- 删掉主面配置块：`plugin unloaded`（当前实例与退役实例各一次）→
  `auth file changed (REMOVE): opencode-go-main.json` → **auth 目录为空**，
  `/v8/management/credentials` 不再有 opencode-go 条目。
- 停服（SIGTERM）：退出前同样清除账号文件；重新启动后 3 秒内自动重建（自愈）。
- 把配置文件写坏后再修好（中间一次 `failed to reload config`）期间插件保持运行，账号不受影响。
- 教训复核：直接 `cp` 覆盖**正在使用**的 `.so` 会让宿主在读被改写的页时崩溃（本轮在隔离实例上
  实测重现，进程残留但 HTTP 不再响应）；升级必须新增版本号文件，与 README §7 一致。

### 4. 公开验收（生产 127.0.0.1:8317）

```
== OpenCode Go acceptance @ http://127.0.0.1:8317 ==
served models: chat=Beam responses=gpt-6-luna gemini=gemini-4-argon messages=none
  PASS  chat/completions non-stream / stream / via claude client / via responses client
  PASS  responses non-stream / stream / via chat client / via claude client
  PASS  gemini generateContent / streamGenerateContent / via chat client
  SKIP  messages family（无启用模型）
  PASS  /v1/models catalogue（16 个）
  PASS  dashboard state / live model list / quota resource（9 个窗口）
  PASS  account projection（1 accounts, auth-dir /root/.cli-proxy-api）
  PASS  account files（1 files, 0600, managed_by marker）
  PASS  statistics resource / monthly window / data-update reminder / 页面 13 项
== 36 passed, 0 failed ==
```

管理 API 的 5 项（`supports_quota`、`quota_provider`、`quota_plugin_id`、per-plugin quota fetch、
`auth-files` 关联）生产端未跑：管理密钥在 `config.yaml` 里是 bcrypt 哈希，明文不可得；
这 5 项在隔离实例上用**同一份 1.8.0 二进制**全部 PASS，页面侧由 36 项里的 2 项匿名检查兜底
（`accounts.files` 与 auth 文件本体）。这与 1.7.1 记录里"管理密钥不可用、管理检查跳过"的处理一致。

### 5. 单元测试

`go test ./...`、`go vet ./...`、`go test -race ./internal/...` 全部通过；新增
`internal/accounts`（Plan/Sync/Purge/顺序无关命名）、`internal/plugin`（配额映射、凭据来源优先级、
`quota.identifier/describe/fetch/reset`、仅主面声明 `quota_provider`、quiesce vs shutdown 的清理语义、
`register-accounts` 开关、`auth-dir` 解析）与 `internal/config`（新开关默认值）用例。

---

## v1.7.8 模型列表变更提醒（热重载 + API / 页面 / 桌面与手机）

### 1. 部署

- 03:46 UTC 热重载到 **1.7.8**；1.7.7 是加入模型变更提醒的中间版本，1.7.8 修正了横幅里
  「与 2026-10-17 快照 不一致」多出的空格。最终四个面均在 `/proc/3651300/maps` 中：
  `cpa-opencodego{,-responses,-messages,-gemini}-v1.7.8.so`，主面 `/api/state` 返回 `version=1.7.8`。
- `config.yaml` 继续用**原地追加注释**触发 fsnotify：inode `10653636` 前后不变，没有重启 CPA；
  PID `3651300`、服务启动时间 `03:07:04 UTC` 均未变。
- 部署前备份了 1.7.7 四个 `.so` 与 config 到
  `/root/cliproxyapi/backup-1.7.8-pre-20261017T034651Z`。

### 2. 模型快照与 API

- `pricing.SnapshotModelIDs` 保存核对日（2026-10-17）实际从 `/models` 观察到的 37 个 id。
  它**不复用价格表的 37 个条目**：价格表含 `cat-jam-free` 而不含 `omen-alpha`，直接对比会误报；
  模型快照与 catalogue 也分开——catalogue 表示“这个构建会服务的模型”，快照表示“核对时上游
  实际发布了哪些模型”。
- 每次 `/api/models` 成功拉取后，主面用 `CompareModelList` 做集合 diff，结果保存在 Runtime；
  空列表或拉取失败不会覆盖上一次成功检查，避免把一次网络失败误报成“全部模型下线”。
- `/api/stats` 新增 additive 字段：`model_list_checked_at`、`model_list_changed`、
  `model_list_added`、`model_list_removed`。生产实测（03:46 UTC，`/api/models` 之后）：

```
model_list_checked_at: 2026-10-17T03:46:57.131945511Z
model_list_changed:    false
model_list_added:      (absent)
model_list_removed:    (absent)
```

同一时刻价格字段仍为 `checked_at=2026-10-17 / age=0 / update_due=false / due_at=2026-11-16`，
totals 为 6,220 请求、12 个模型；没有新增 stats 文件字段或状态文件。

### 3. 页面验证

- 非提醒态（真实当前数据）：统计页一行显示
  「核对于 2026-10-17（今天）· 下次提醒 2026-11-16 · 模型列表无变化 · 30 天周期」；
  到期/模型变更横幅隐藏。
- 模型变更态：用 Chromium CDP 拦截 `/api/stats`，在真实响应上设置
  `model_list_changed=true`、新增 5 个、移除 2 个；页面显示
  「数据该更新了」+「模型列表有变更」，正文为
  「上游模型列表与 2026-10-17 快照不一致：新增 future-a、future-b、future-c、future-d 等 5 个；
  移除 glm-5.1、grok-4.6。」，并保留官方价目表链接和“当前统计仍按旧表折算”的说明。
  到期的一行状态同时隐藏。
- 模型变更与 30 天到期同时成立时，徽标显示「价目到期 · 模型变更」；两种提醒合并成一条横幅，
  不会出现两条重复提示。
- 桌面 1440×1100 与手机 390×844 均验证；手机模型明细仍按 ≤640px 折叠。统计 Tab 打开时会调用
  `/api/models`（上游结果缓存 5 分钟）；若本次 fetched_at 与 stats 中的检查时间不同，会强制重读
  一次 stats，因此页面首次加载的请求竞态也能收敛。

### 4. 自动化

- `go test ./...`、`go vet ./...`、`go test -race ./internal/...` 通过；JS 通过 `node --check`。
- 新增 `TestSnapshotModelIDsSortedAndUnique`、`TestCompareModelList`：
  覆盖 37 个快照 id、副本返回、重复/空白 id、空列表不误报、新增/移除排序。
- 新增 `TestStatsPayloadExposesModelListChanges`：断言 stats API 的时间戳、变更标记和增删 id，
  并验证空列表不会覆盖上一次成功检查；页面测试断言 `model_list_*` 与提醒文案。
- `scripts/acceptance.sh` 的 data-update 检查现在同时要求 `model_list_checked_at` 和
  `model_list_changed`；本轮没有重跑完整公开验收，仍以 1.7.1 的 **32 passed, 0 failed** 为准。

---

## v1.7.6 价目核对提醒（热重载 + API / 页面 / 桌面与手机）

### 1. 部署

- 03:32 / 03:33 UTC 先以 1.7.4 / 1.7.5 热重载预览；03:34 UTC 最终热重载到 **1.7.6**。
  1.7.4/1.7.5 是中间版本，1.7.6 修掉一个只在到期态出现的 CSS 问题：`.stats-freshness` 的
  `display:flex` 覆盖了 `hidden` 属性，导致手机端同时显示横幅和重复的一行状态。现在到期态只保留
  横幅，非到期态显示一行「核对于 … · 下次提醒 …」。
- 四个 `-v1.7.6.so` 均在 `/proc/3651300/maps` 中；主面 `/api/state` 返回 `version=1.7.6`。
- `config.yaml` 仍是**原地追加注释**触发 fsnotify：inode `10653636` 前后不变，没有重启 CPA；
  PID `3651300`、服务启动时间 `03:07:04 UTC` 均未变。
- 部署前备份了 1.7.5 四个 `.so` 与 config 到
  `/root/cliproxyapi/backup-1.7.6-pre-20261017T033434Z`。

### 2. 口径与 API

`internal/pricing` 新增 `SnapshotCheckedAt = "2026-10-17"`（最后一次人工核对官方 Go 页面的日期）
和 `RefreshAfterDays = 30`；`SnapshotFreshness` 按 UTC 日历天计算年龄，满 30 天后
`price_book_update_due=true`。日期和构建时间刻意分开：只重新编译而没有重新核对，旧价格不会显得新。

`GET /api/stats` 的 additive 字段（生产实测）：

```
price_book_revision:          2026-10
price_book_checked_at:        2026-10-17
price_book_age_days:          0
price_book_refresh_after_days: 30
price_book_update_due:        false
price_book_due_at:            2026-11-16
price_book_url:               https://opencode.ai/docs/go/
```

同一时刻 totals 为 6,202 请求、438,318,884 tokens、12 个模型；没有新增 stats 文件字段或状态文件。
这是**周期提醒**，不是自动抓取；到期只表示应该去官方价目表重新核对，确认变更后仍需更新
`internal/pricing` 并发布新版本，历史金额不重算。

### 3. 页面验证

- 非到期态：统计页概览下方显示一行绿色「价目表 2026-10」+「核对于 2026-10-17（今天）·
  下次提醒 2026-11-16 · 30 天周期」+「官方价目表 ↗」；到期横幅隐藏。
- 到期态：用 Chromium DevTools Protocol 拦截 `/api/stats`，把同一份真实响应改为
  `checked_at=2026-09-17 / age=30 / update_due=true / due_at=2026-10-17`，页面显示
  「数据该更新了」、徽标「30 天未核对」、官方链接和「这只是周期提醒，不代表上游已经调价」说明；
  重复的一行状态同时隐藏。
- 桌面 1440×1100 与手机 390×844 均验证；手机端到期横幅在概览上方，模型明细仍按原有的
  ≤640px 规则折叠，没有把内容重新平铺。
- 页面实际返回的 `/dashboard` 含 `statsPriceNotice`、`statsPriceFreshness`、`statsPriceCheckedAt`
  等标记；JS 通过 `node --check`。

### 4. 自动化

- `go test ./...`、`go vet ./...`、`go test -race ./internal/...` 通过。
- 新增 `TestSnapshotFreshnessUsesCalendarDays`，覆盖同日、29 天、第 30 天、第 75 天和早于核对日；
  `TestStatsPayloadMergesFacesAndComputesAverages` 断言 API 的五个新字段；页面测试断言提醒元素、
  文案和 `.stats-freshness[hidden]` 规则。
- `scripts/acceptance.sh` 新增两项：统计 API 的价目提醒元数据、页面 `statsPriceNotice` 标记。
  本轮没有重跑完整公开验收（仍以 1.7.1 的 **32 passed, 0 failed** 为准），因此不宣称 34 项全过。

---

## v1.7.1 TPS/TTFT 口径修正与历史回放（生产重启 + 公开 32 项）

### 1. 部署与回放

2026-10-17 03:04 UTC，停止 `cliproxyapi.service`，备份 stats、1.6.1 四个 `.so` 与
`config.yaml` 到 `/root/cliproxyapi/backup-1.7.0-20261017T030417Z`；新增四个
`-v1.7.0.so`，用 `dist/replay-tracker-v1.7.0` 先 dry-run 再 `-apply`：

```
scanned: 6049 opencode-go records; selected: 5628; skipped by cutoff: 421; invalid: 0
range:   2026-10-05T05:29:41Z .. 2026-10-17T02:21:02Z
tokens:  366045455; list price: $27.835431; latency samples: 5628; TTFT samples: 4531
backup:  /tmp/cpa-opencode-go-stats-backup-20261017T030438
imported 5628 records into stats-chat.json (marker: replay-tracker-import.json)
```

- 截止时间取现有 stats 的最早一条 `2026-10-17T02:21:08Z`，避免和 v1.6 已统计的 420 条重复。
  回放后 5,628 + 420 = 6,048 条；导入的 366,045,455 + 原有 53,850,587 = 419,896,042 tokens，
  与 `stats-chat.json` 的总量逐项相等，说明没有重复计数。
- 5628 条中 4531 条有真实 TTFT；其余（旧 `cpa-opencodego*` 记录 `ttft_ns=0`）只回放
  token/费用/总延迟，不伪造 TTFT。
- 导入后 `stats-chat.json` 为 0600、137,084 bytes；`grep oc_sk_` / `grep api_key` 均无输出。
- 1.7.0 启动日志显示四个面均为 `version=1.7.0`；因为只修两处页面脚注，随后以
  `-v1.7.1.so` 再重启一次，四个面均为 `version=1.7.1`，新 PID `3651300`；stats 与 marker 不变。

### 2. 公开验收（真实上游）

```
== OpenCode Go acceptance @ http://127.0.0.1:8317 ==
served models: chat=Beam responses=gpt-6-luna gemini=gemini-4-argon messages=none
...
PASS  statistics resource  ready | matched=444 requests=6072 tokens=420099867
      ttft=4924 tps=5534 missing_families=- warnings=
...
== 32 passed, 0 failed ==
```

- chat（Beam）、responses（gpt-6-luna）、gemini（gemini-4-argon）的流式/非流式及跨协议调用均通过；
  messages 因子模型未启用被脚本跳过。
- 统计累计 12 个模型、6,072 请求、420,099,867 tokens、约 $28.36 刊例价；TPS 样本 5,534，
  TTFT 样本 4,924；没有 provider 过滤 warning。
- 单独用 `Beam` 发 `stream=true, max_tokens=8` 后，`received` 从 420 增到 421，
  `beam-free` 的 `requests/stream_requests/ttft_samples/generation_samples/latency_samples`
  各增加 1，`latency_output_tokens` 增加 8，证明实时路径也走新的配对 TPS 与内容帧 TTFT。
- 月窗口当前仍因 `/api/quota` 无 monthly 窗口而回落 UTC 自然月（`since_source=calendar`）；
  Gemini 26.7%、DeepSeek 15.3% 等百分比按当前价目表计算，随历史回放一并补齐。

### 3. 口径变更

- **旧**：`Σ output tokens / Σ generation_nano`，分子来自宿主、分母来自插件，二者没有 request id
  关联；缺失 timing 的流会计入分子不计入分母，单帧/合帧样本会把 TPS 放大。
- **新**：`Σ 成功请求有效 output / Σ 宿主 latency_nano`，同一 usage 记录成对累加；
  gemini reasoning 计入分子，失败请求不计；非流式请求也计入整体 TPS。它是端到端吞吐，
  包含首字/排队/网络，不能当纯解码速度。
- TTFT 只取 `firstContent - started`；没有内容帧就没有样本，不再回退 firstFrame；上游 error 帧、
  空流、客户端断开均排除；generation 只到最后一个内容帧，仅作诊断，不参与 TPS。

### 4. 回滚

保留 `/root/cliproxyapi/backup-1.7.0-20261017T030417Z`（四个 1.6.1 `.so` 与 stats）和
`/root/cliproxyapi/backup-1.7.1-20261017T030704Z`（1.7.0 四个 `.so` 与最新 stats）。
回滚时停服、放回 1.6.1 四个 `.so`、覆盖 stats、启动；v1.7 的 additive `latency_*` 字段会被
1.6.1 忽略。1.7.1 的页面脚注变化不影响 stats schema。详细步骤见 [rollback.md](rollback.md)。

---

## v1.6.0 / 1.6.1 统计面板（生产热重载 + 隔离实例 40 项 + 桌面/手机浏览器）

本轮新增只读观测面，不改变任何协议转换、模型路由、别名或 Key 池行为。设计见
[design.md](design.md) §7.8；数据来源与口径在页面「统计口径与价目说明」里也有一份。

### 1. 生产实例：热重载到 1.6.1

```
$ CPA_ROOT=/root/cliproxyapi bash scripts/install.sh 1.6.1
installed …/cpa-opencodego-v1.6.1.so
installed …/cpa-opencodego-responses-v1.6.1.so
installed …/cpa-opencodego-messages-v1.6.1.so
installed …/cpa-opencodego-gemini-v1.6.1.so

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
| 统计落盘 | ✅ | `data/cpa-opencodego/` 新增 `stats-chat.json`、`stats-gemini.json`、`stats-responses.json`（0600）；`shared.json` 仍在 |
| 统计文件无 Key | ✅ | `grep -l oc_sk_ stats-*.json` 无输出 |
| 公开验收 | ✅ | **32 passed, 0 failed** |

生产环境没有管理密钥明文：`management.secret-key` 是 60 字符 bcrypt 哈希（`$2a$…`），
所以侧边栏、四行品牌、管理探测这 6 项按脚本的 preflight 规则跳过；它们由隔离实例覆盖。

### 2. 生产统计结果（真实上游请求）

用客户端 `DeepSeek-V4.1-Flash` 发一次 `stream=true` 的 `max_tokens=8` 请求，等待一个 5 秒
落盘周期后：

```
$ curl -s http://127.0.0.1:8317/v0/resource/plugins/cpa-opencodego/api/stats?range=all
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

统计只新增 `data/cpa-opencodego/stats-<role>.json`（0600，无 Key）。删除 1.6.1 的四个 `.so`
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
installed /root/cliproxyapi/plugins/linux/amd64/cpa-opencodego-v1.5.0.so
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
| 生产页面 = 仓库源码 | ✅ | 用 `go run` 生成 `dashboard.Page("cpa-opencodego","chat")` 与生产 `GET /v0/resource/plugins/cpa-opencodego/dashboard` **字节一致**（各 96 460 bytes，`diff` 退出码 0） |
| 品牌已在生产页面生效 | ✅ | 生产 HTML 含 `<link rel="icon" type="image/svg+xml" href="data:image/svg+xml;base64,…">`、`class="brand-mark"`（3 处）与「协议面」卡片标记 |
| 配置只被注释动过 | ✅ | sha256 `e4b5c63c…` → `28a8cf3f…`，inode 未变；新增内容就是 2 行 `# (v1.5.0: …)` 注释 |
| 无残留 | ✅ | `data/cpa-opencodego/` 仍然只有 `shared.json`；插件目录 21 个 `.so`（v1.2.0 / v1.3.0 / v1.4.0 / v1.4.1 / v1.5.0 四份 + 用量插件） |
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
  PASS  sidebar menu entry                             OpenCode Go /v0/resource/plugins/cpa-opencodego/dashboard
  PASS  plugin faces                                   cpa-opencodego(-gemini|-messages|-responses)
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
installed /root/cliproxyapi/plugins/linux/amd64/cpa-opencodego-v1.4.1.so
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
| 页面换成 Tab 版 | ✅ | `GET /v0/resource/plugins/cpa-opencodego/dashboard` 含 `data-tab="accounts"`、`id="keyList"`、`id="credModal"`，且**不再含**「只读模式」字样 |
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
| 零残留不变 | ✅ | 未新增任何落盘物：仍只有 `data/cpa-opencodego/shared.json`（+ 可选 `debug.log`），别名只存在宿主拥有的 config.yaml 里 |

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

回滚路径（同样不用重启）：删除插件目录里的 `cpa-opencodego*-v1.3.0.so`，再原地补一行注释 →
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
  PASS  sidebar menu entry                             OpenCode Go /v0/resource/plugins/cpa-opencodego/dashboard
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
| A5 | 侧边栏入口 | ✅ | `GET /v8/management/plugins` 返回 `menus:[{menu:"OpenCode Go", path:"/v0/resource/plugins/cpa-opencodego/dashboard"}]`；`/dashboard` 返回 26 KB HTML（`text/html; charset=utf-8`，带 CSP） |
| A6 | 每模型代理开关真实生效 | ✅ | `gpt-6-luna` 设 `direct` → **403 `unsupported_country_region_territory`**；改 `proxy` → **200**。开关经由管理 API 热重载生效 |
| A7 | 额度显示 | ✅ | `GET /api/quota` 返回 `[("5 小时",0%),("本周",6%),("本月",18%)]` 及各自 `resets_at` |
| A8 | 多 Key | ✅ | Key 池放入一个无效 Key + 一个有效 Key，连续 3 次请求全部 200；坏 Key 记录 401 并进入冷却，好 Key 计数 7 次成功 |
| A9 | 完全可回滚 | ✅ | 删除 4 个 `.so` + 4 个配置块 + 数据目录并重启后：`/v1/models` 为 0、插件列表为空、dashboard 404、`data/cpa-opencodego` 不存在、auth 目录 0 文件 |

## 附加验证

- **插件加载**：4 个实例全部 `plugin loaded` + `plugin registered`，无告警。
- **协议格式声明**：`cpa-opencodego` 声明 `["openai"]`，`-responses` 声明 `["codex","openai-response"]`，
  `-messages` 声明 `["claude"]`，`-gemini` 声明 `["gemini"]`。
- **模型归属**：`/v1/models` 37 项全部 `owned_by = "opencode"`。
- **配置持久化**：仪表盘写入路径（`PUT /v8/management/config/plugins/configs/cpa-opencodego`）返回
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
