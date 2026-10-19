# v2.0.0 升级指南（cpa-opencodego）

v2.0.0 是一次大版本升级，包含三件事：

1. **重命名**：插件 id / `.so` 前缀 / GitHub 仓库都改为 `cpa-opencodego`；状态目录改为 `data/cpa-opencodego`。
2. **请求日志**：新增「请求日志」Tab，逐条展示时间、输入/输出/合计 token、TTFT、生成 TPS、端到端 TPS、状态与错误。
3. **协议对齐**：支持 OpenCode Go 新的分族端点（`/openai/v1`、`/anthropic/v1`、`/google/v1beta`），旧 `/v1` 单根端点继续兼容；并把 OpenAI 的 `developer` 角色在发送前归一化为 `system`，消除新路由偶发的 400。

模型 id、别名、代理策略、DSH 调用方式都不变；插件仍然只自行发送 `x-opencode-session` 一个请求头，`User-Agent` 跟随 CPA 透传。

> **v2.0.1 热修复（已在生产中热部署）**：修复 DeepSeek 思考模型长任务偶发的
> `The reasoning_content in the thinking mode must be passed back to the API`。默认
> `reasoning-replay: "auto"` 会为缺少 `reasoning_content` 的历史 assistant 消息补占位文本，
> 并对该 400 自动重试一次。升级只需替换四个 `.so` 并原地改写一次 `config.yaml`（补上
> `reasoning-replay: "auto"` 可选，默认即 auto），**不需要重启 CPA**。其余步骤与 v2.0.0 相同。

---

## 一、升级前检查

```bash
# 1. 当前版本（应为 1.12.x）
curl -s http://127.0.0.1:8317/v0/management/plugins \
  -H "Authorization: Bearer $CPA_MGMT_KEY" | python3 -m json.tool | grep -E 'cap-opencode-go|version'

# 2. 备份配置与状态
cp -a /root/cliproxyapi/config.yaml /root/cliproxyapi/config.yaml.bak-$(date +%s)
cp -a /root/cliproxyapi/data/cap-opencode-go /root/cliproxyapi/data/cap-opencode-go.bak-$(date +%s)
```

## 二、热升级步骤（不需要重启 CPA）

> CLIProxyAPI 支持插件热重载：替换 `.so` 并在原地改写 `config.yaml`（**不要替换 inode**，例如用 `>` 重定向而不是 `mv` 覆盖）即可触发重载。以下步骤不重启进程、不清空运行状态。

```bash
cd /path/to/cpa-opencodego

# 1. 构建（四个 .so）；v2.0.1 是当前版本
make build VERSION=2.0.1

# 2. 安装：会先删除遗留的 cap-opencode-go*.so，再复制新文件
bash scripts/install.sh 2.0.1

# 3. 迁移配置块与 v1 状态目录（默认 dry-run，先看输出）
bash scripts/migrate-v1.sh
bash scripts/migrate-v1.sh --apply
#    脚本会把 plugins.configs.cap-opencode-go* 改名为 cpa-opencodego*，
#    并把 data/cap-opencode-go 的 shared.json / stats-*.json 复制到新目录；
#    旧目录保留，便于回滚。

# 4. 确认 config.yaml 的四个配置块使用新 id：
#      plugins.configs.cpa-opencodego
#      plugins.configs.cpa-opencodego-responses
#      plugins.configs.cpa-opencodego-messages
#      plugins.configs.cpa-opencodego-gemini

# 5. 触发/确认热重载（宿主日志里应出现 active_version=2.0.0）
curl -s http://127.0.0.1:8317/v0/management/plugins \
  -H "Authorization: Bearer $CPA_MGMT_KEY" | python3 -m json.tool | grep -A2 cpa-opencodego
```

若宿主没有自动重载，可在管理面板「插件管理」里对四个面点一次重载，或原地改写一次 `config.yaml` 的注释行。**不要** `systemctl restart`：v2 的升级被设计为热重载完成。

## 三、升级后验证

```bash
# 1. 插件行与版本
curl -s http://127.0.0.1:8317/v0/management/plugins -H "Authorization: Bearer $CPA_MGMT_KEY"

# 2. 布局与实际端点（family 或 legacy，endpoints 为四族生效 URL）
curl -s http://127.0.0.1:8317/v0/resource/plugins/cpa-opencodego/api/state | python3 -m json.tool | head -40

# 3. 请求日志（发一条真实请求后再看）
curl -s 'http://127.0.0.1:8317/v0/resource/plugins/cpa-opencodego/api/requests?limit=20' | python3 -m json.tool | head -60

# 4. 公开验收脚本（含四族请求、统计、请求日志、页面字段）
CPA_BASE=http://127.0.0.1:8317 CPA_CLIENT_KEY=sk-... CPA_MGMT_KEY=... bash scripts/acceptance.sh
```

页面检查：插件入口 → 顶部应出现「请求日志」Tab；「状态」页能看到「端点布局」「实际请求」；「设置 → 协议与兼容」可切换布局；「设置 → 状态与调试」可关闭请求日志。

## 四、回滚到 v1.12.0（同样可热重载）

1. 恢复旧 `.so`：把 `dist/cap-opencode-go*-v1.12.0.so` 复制回 `plugins/linux/amd64/`，删除 `cpa-opencodego*.so`。
2. 恢复旧配置块名（把 `plugins.configs.cpa-opencodego*` 改回 `plugins.configs.cap-opencode-go*`；备份在 `config.yaml.bak-*`）。
3. `config.yaml` 原地写一次，等待热重载；旧状态目录 `data/cap-opencode-go` 仍在，统计历史可直接使用。
4. v2 写出的 `requests-*.jsonl` 不会影响 v1；如需彻底清理，删除 `data/cpa-opencodego`。

## 五、配置变化

新增字段（都有默认值，旧配置不写也能启动）：

| 字段 | 默认 | 说明 |
|---|---|---|
| `base-url` | `https://opencode.ai/inference/go` | 新分族布局的 Go 根；以 `/v1` 结尾的旧值仍走旧布局 |
| `endpoint-layout` | `auto` | `auto` / `legacy` / `family` |
| `base-urls` | `{}` | 逐族 API 根覆盖：`chat` / `responses` / `messages` / `gemini` |
| `developer-role` | `rewrite` | 把 OpenAI 形状请求里的 `developer` 改写为 `system`；`keep` 关闭 |
| `request-log` | `true` | 请求日志开关 |

说明：

- **旧配置继续可用**：`base-url: https://opencode.ai/inference/go/v1`（或 `/zen/go/v1`）会自动判定为 legacy 布局，路径与 v1 完全一致。
- **请求日志**：每个协议面一个 `requests-<role>.jsonl`，最多 2000 条 / 7 天；只有模型、计数、耗时、状态和错误摘要，会话存 8 位哈希，不含正文或密钥。
- **UA**：插件不伪造 User-Agent，客户端给什么就透传什么；插件只负责 `x-opencode-session`。
- **统计**：统计 Tab 的口径不变（宿主 usage 事件）；请求日志是插件自测数据，两者可能略有差异。

## 六、常见问题

**Q：升级后页面看不到 Key / 请求报“没有可用的 API Key”。**
A：配置块没有改名。执行 `scripts/migrate-v1.sh --apply`，或手动把 `plugins.configs.cap-opencode-go*` 改成 `cpa-opencodego*`，再触发一次热重载。

**Q：插件管理里出现 8 行（新旧各四个）。**
A：旧 `.so` 没删。`scripts/install.sh` 会自动清理；手动删除 `plugins/linux/amd64/cap-opencode-go*.so` 即可。

**Q：统计历史空了。**
A：状态目录没有迁移。检查 `data/cap-opencode-go` 是否被复制到 `data/cpa-opencodego`；也可手动复制 `shared.json` 与 `stats-*.json`。

**Q：请求日志为空。**
A：确认「设置 → 状态与调试 → 记录请求日志」是开的，且之后发过新请求；旧请求不会补记。
