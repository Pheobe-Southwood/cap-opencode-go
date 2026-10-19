# 回滚手册

插件的设计前提是**零残留**：它不修改 CPA 源码，不改动任何内置 provider 组
（`openai-compatibility` / `gemini` / `codex` …），也不写 CPA 自己的数据库。
v1.8–v1.11 它曾**投影**认证文件（每个 Key 一个，见 §10），v1.12 已移除该功能，
现在插件完全不碰 auth 目录。它落盘的东西只有下面这些。

## 1. 插件拥有的文件

| 类别 | 路径 | 由谁删除 |
|---|---|---|
| 插件库 | `<root>/plugins/<goos>/<goarch>/cpa-opencodego*-v<ver>.so` | `scripts/uninstall.sh` |
| 配置块 | `config.yaml` 的 `plugins.configs.cpa-opencodego*` | 操作者手动删除（脚本会打印清单） |
| 数据目录 | `<root>/data/cpa-opencodego/`（`shared.json`、`stats-<role>.json`、`requests-<role>.jsonl`、可选的 `replay-tracker-import.json` / `migrated-from-v1.json` / `debug.log`） | `scripts/uninstall.sh` |
| v1 数据目录（升级遗留） | `<root>/data/cap-opencode-go/`（升级时只复制、不删除，保留用于回滚） | `scripts/uninstall.sh` |
| 历史投影账号（v1.8–v1.11） | `<auth-dir>/opencode-go-<key 名>.json`（带 `managed_by: cpa-opencodego`） | v1.12 不再生成；`scripts/uninstall.sh` 按标记兜底清理 |

`<root>` 默认 `/root/cliproxyapi`，可用 `CPA_ROOT` 覆盖。

## 2. v2.0 热回滚（不需要重启）

v2.0 支持热重载，回滚同样不需要重启：

```bash
# 1) 恢复 v1 插件库（旧 .so 应仍在 dist/ 或备份目录里）
cp dist/cap-opencode-go*-v1.12.0.so /root/cliproxyapi/plugins/linux/amd64/
rm -f /root/cliproxyapi/plugins/linux/amd64/cpa-opencodego*.so

# 2) 把 config.yaml 的四个配置块名改回 v1（备份在 config.yaml.bak-*）
#    plugins.configs.cpa-opencodego*  ->  plugins.configs.cap-opencode-go*

# 3) 原地改写 config.yaml 触发热重载（不要 mv 覆盖，inode 变化不会触发热重载）
#    宿主机日志出现 active_version=1.12.0 即完成；旧状态目录 data/cap-opencode-go 仍在。

# 4) 复核
curl -s http://127.0.0.1:8317/v0/management/plugins -H "Authorization: Bearer $CPA_MGMT_KEY" | grep -E 'cap-opencode-go|cpa-opencodego'
```

v2 写出的 `data/cpa-opencodego/requests-*.jsonl` 与统计文件不会影响 v1；确认不再回滚后可删除新目录。

## 3. 标准卸载/回滚流程（需要彻底卸载插件时）

```bash
# 1) 删除插件库与数据目录（脚本只删自己创建的文件）
CPA_ROOT=/root/cliproxyapi bash scripts/uninstall.sh

# 2) 从 config.yaml 删除以下四个配置块
#    plugins.configs.cpa-opencodego
#    plugins.configs.cpa-opencodego-responses
#    plugins.configs.cpa-opencodego-messages
#    plugins.configs.cpa-opencodego-gemini
#    若升级尚未完成，还要删除 v1 名称的四个块（cap-opencode-go*）
#    保留 plugins.enabled 与其它插件条目

# 3) 重启，让 dlopen 的插件实例彻底卸载
systemctl restart cliproxyapi

# 4) 复核
curl -s -H "Authorization: Bearer <client-key>" http://127.0.0.1:8317/v1/models | \
  python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]))'   # 回到安装前的数量
ls /root/cliproxyapi/data/cpa-opencodego 2>/dev/null || echo "无残留"
```

## 4. 也可以在管理中心里卸载

`插件管理` 页面提供删除按钮（`DELETE /v8/management/plugins/<id>`），它会同时删除 `.so` 与
对应的 `plugins.configs` 块。对已加载的插件该接口可能返回
`409 plugin_delete_requires_restart`——此时先删配置块并重启，再删除文件即可。
四个实例需要分别处理，或直接用上面的脚本一次删净。

## 5. 回滚验证结果

在隔离实例（`/tmp/cpa-test`）上按上述流程执行后：

| 检查项 | 结果 |
|---|---|
| `GET /v1/models` | 0 个模型（回到基线） |
| `GET /v8/management/plugins` | `plugins = []` |
| `GET /v0/resource/plugins/cpa-opencodego/dashboard` | 404 |
| `<root>/data/cpa-opencodego/` | 不存在 |
| `<root>/plugins/linux/amd64/*.so` | 0 个 |
| `<root>/auth` 目录 | 0 个文件（v1.12 起插件不写 auth 目录；历史投影文件见 §10） |

## 6. v1.2.0 之后仍然零残留

v1.2.0 只改了页面代码与新增一个**由插件自己声明的管理路由**，没有引入任何新的落盘物：

| 新东西 | 卸载后 |
|---|---|
| 顶部操作栏 / 逐模型设置面板 / 搜索框 | 只存在于 `.so` 里的 HTML 字符串 |
| 复用管理中心登录态读密钥 | 只发生在浏览器内存与 fetch 请求头里，插件不保存 |
| `GET /v0/management/opencode-go/probe` | 路由声明随 `.so` 一起消失（不写 config.yaml） |
| `--host-header` 顶部内移 | 运行时读取父页面高度，**不修改**宿主页面（无注入、无样式残留） |

因此回滚清单与 v1.1.1 完全一致：4 个 `.so` + 4 个 `plugins.configs.*` 块 + `data/cpa-opencodego/`。

## 7. v1.3.0 之后仍然零残留

v1.3.0 只改了内存里的**解析结果**与页面渲染，没有任何新的落盘物：

| 新东西 | 卸载后 |
|---|---|
| 别名重命名（`Entry.BaseID`、`Resolver.Shadowed()` 归属） | 纯内存结构，随进程消失 |
| `/api/models` 的 `client_ids` / `aliased` / `shadowed_aliases` 字段 | 只是响应体里的字段，不写任何文件 |
| 页面显示「对外 id」 | 只存在于 `.so` 里的 HTML 字符串 |

别名本身仍然只写在宿主拥有的 `config.yaml`（`plugins.configs.cpa-opencodego.models.<id>.aliases`）里，
插件从不写 config.yaml，也不为别名建立任何索引文件或缓存。
因此回滚清单与 v1.1.1 / v1.2.0 完全一致：4 个 `.so` + 4 个 `plugins.configs.*` 块 + `data/cpa-opencodego/`。

## 8. v1.4 之后仍然零残留

v1.4（含 1.4.1）只改了页面与只读接口，没有新增任何落盘物：

| 位置 | v1.4.1 之后 |
|---|---|
| `plugins/linux/amd64/*.so` | 只有插件库（v1.4.1 与保留的旧版本） |
| `data/cpa-opencodego/` | 仍然只有 `shared.json`（+ 可选 `debug.log`） |
| `config.yaml` | 只多了一行说明性注释；写入完全走宿主的配置 API |
| 浏览器 | 默认不存任何东西；只有操作者主动勾「在本标签页内记住」时，
才会把管理密钥写进 `sessionStorage`（关闭标签页即失效，且属于浏览器而非插件目录） |

因此「删除插件 → 没有任何残留」的结论不变，`scripts/uninstall.sh` 也无需改动。

## 9. v1.5 之后仍然零残留

v1.5.0 只改了注册元数据（名称、图标、配置字段声明）与页面呈现，图标是**编译期内嵌**的
`internal/brand/opencode.svg`（以 data URI 随注册负载下发），没有任何新的落盘物：

| 位置 | v1.5.0 之后 |
|---|---|
| `plugins/linux/amd64/*.so` | 只有插件库（v1.5.0 与保留的旧版本） |
| `data/cpa-opencodego/` | 仍然只有 `shared.json`（+ 可选 `debug.log`） |
| `config.yaml` | 只多了 2 行说明性注释；写入完全走宿主的配置 API |
| 浏览器 | 与 v1.4 相同：默认不存任何东西，`sessionStorage` 只在操作者主动勾选时使用 |
| 插件图标资源 | 不存在独立文件／路由，随 `.so` 一起消失 |

**注意**：三个从属面的 `plugins.configs.<id>` 块不能删——`enabled: true` 是宿主的加载开关，
删掉它们就等于卸载对应协议族（该族模型随即不可路由）。回滚清单仍与 v1.1.1 一致：
4 个 `.so` + 4 个 `plugins.configs.*` 块 + `data/cpa-opencodego/`。

## 10. v1.6 / v1.7 之后仍然零残留

v1.6.0 新增了统计面板，它会写自己的计数器文件；这仍然全部落在上面那个数据目录里：

| 位置 | v1.6.0 之后 |
|---|---|
| `plugins/linux/amd64/*.so` | 只有插件库（当前版本；重启会清理旧版本，回滚备份需另存） |
| `data/cpa-opencodego/` | `shared.json` + `stats-chat.json` / `stats-responses.json` / `stats-messages.json` / `stats-gemini.json`（有请求后才出现）+ 可选 `debug.log` |
| `config.yaml` | 可能多一行 `plan: "go"`；旧版本把它当未知字段忽略，不会破坏配置 |
| 浏览器 | 与 v1.4 相同：默认不存任何东西，`sessionStorage` 只在操作者主动勾选时使用 |

统计文件只有模型 id、计数、费用、延迟与时间戳，不含 Key、提示词或响应内容。v1.7 的
`latency_output_tokens` / `latency_nano` / `latency_samples` / `stream_output_samples` 是
additive 字段：回滚到 v1.6.1 时旧版本会忽略它们，并继续用旧的 `stream_output_tokens` /
`generation_nano` 计算旧口径 TPS；v1.7.2 的生成 TPS 也读取这两个旧字段，所以回滚不丢数据；
回滚到 1.5.0 时这些文件被整体忽略。`replay-tracker-import.json` 只是回放标记，不影响运行；
`scripts/uninstall.sh` 会连同整个数据目录一起删除。
回滚清单仍与 v1.1.1 一致：4 个 `.so` + 4 个 `plugins.configs.*` 块 + `data/cpa-opencodego/`。

**注意**：`usage_plugin` 能力声明只存在于主面。如果只升级主面、不升级三个从属面，
统计仍能拿到 token 与总延迟，但从属面的 TTFT 不会出现；四个 `.so` 必须同版本安装。
v1.7 回放工具只在停服时读 `token-usage-tracker.db`，不修改该库；重复导入默认被 marker 拒绝。
1.7.1 只改页面脚注，回滚 1.7.0 / 1.6.1 都不影响 stats schema。
1.7.8 的价目/模型核对提醒只新增 `/api/stats` 字段和页面元素，不改 stats 文件、不新增状态文件；
回滚到 1.7.3 / 1.7.1 时这些字段会被旧页面忽略，提醒消失，统计数字不变。

## 11. v1.12 起不再写认证文件（历史投影由 uninstall.sh 兜底）

v1.8–v1.11 为了把 Key 显示在 CPA 自带的「配额管理 → Plugin」里，会往宿主的 auth 目录写
`opencode-go-<key 名>.json`（v1.11 起每 Key 1 个 chat 面文件）。v1.12 已整体移除该功能：
插件不再创建、改写或删除任何 auth 文件，也不再声明宿主配额 Provider。

| 位置 | v1.12.0 之后 |
|---|---|
| `plugins/linux/amd64/*.so` | 只有插件库（当前版本；重启会清理旧版本） |
| `data/cpa-opencodego/` | 与 v1.6 / v1.7 相同 |
| `<auth-dir>/opencode-go-*.json` | **不再生成**；从旧版本升级时手动删一次，`uninstall.sh` 也会按 `managed_by` 标记兜底 |
| `config.yaml` | 不再有 `register-accounts`（旧配置里残留该行会被宽松 YAML 忽略） |
| 浏览器 | 与 v1.4 相同，默认不存任何东西 |

从 v1.8–v1.11 升级到 v1.12：热重载完成后删一次历史文件即可（1.11 只留 1 个 chat 面文件，
更早版本是 4 个协议面文件）：

```bash
rm -f "${HOME}/.cli-proxy-api"/opencode-go-*.json   # auth-dir 若改过，用实际路径
# 或只做标记清理：CPA_AUTH_DIR=… bash scripts/uninstall.sh
```

**回滚到 v1.11 或更早会重新出现这些文件**：旧版本仍会投影（1.11 = 1 个 chat 面文件，
v1.8–v1.10 = 4 个协议面文件）。这是旧版本的正常行为；再升级回 v1.12 后按上面命令再删一次。

v1.12 的卸载流程因此更简单：`Shutdown` 只 flush 统计文件，auth 目录不再需要任何清理；
`scripts/uninstall.sh` 仍会扫描 `${CPA_AUTH_DIR:-}`、`<root>/auth`、`~/.cli-proxy-api` 并删除
带 `managed_by: cpa-opencodego` 标记的历史文件，其它 provider 的认证文件与操作者自己放的同名
文件不会被碰。

## 12. 常见疑问

**会不会影响我原有的 `openai-compatibility` / `gemini` 组？**
不会。插件只通过 `model_router` 在运行期认领模型 ID，不读写这些配置块。

**同名模型（例如 `gpt-6-luna`）回滚后会怎样？**
立即回到插件加载前的解析路径（例如由 codex OAuth 凭据提供）；插件卸载后其路由声明一并消失。

**删除后 `/v1/models` 里还有 `opencode` 归属的模型？**
说明仍有 `.so` 被加载或 `plugins.configs` 块未删除，重启后再确认一次。
