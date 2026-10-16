# 回滚手册

插件的设计前提是**零残留**：它不修改 CPA 源码，不新增 auth 凭据文件，不改动任何内置
provider 组（`openai-compatibility` / `gemini` / `codex` …），也不写 CPA 自己的数据库。
它拥有的只有下面三类文件。

## 1. 插件拥有的文件

| 类别 | 路径 | 由谁删除 |
|---|---|---|
| 插件库 | `<root>/plugins/<goos>/<goarch>/cap-opencode-go*-v<ver>.so` | `scripts/uninstall.sh` |
| 配置块 | `config.yaml` 的 `plugins.configs.cap-opencode-go*` | 操作者手动删除（脚本会打印清单） |
| 数据目录 | `<root>/data/cap-opencode-go/`（`shared.json`、可选 `debug.log`） | `scripts/uninstall.sh` |

`<root>` 默认 `/root/cliproxyapi`，可用 `CPA_ROOT` 覆盖。

## 2. 标准回滚流程

```bash
# 1) 删除插件库与数据目录（脚本只删自己创建的文件）
CPA_ROOT=/root/cliproxyapi bash scripts/uninstall.sh

# 2) 从 config.yaml 删除以下四个配置块
#    plugins.configs.cap-opencode-go
#    plugins.configs.cap-opencode-go-responses
#    plugins.configs.cap-opencode-go-messages
#    plugins.configs.cap-opencode-go-gemini
#    保留 plugins.enabled 与其它插件条目

# 3) 重启，让 dlopen 的插件实例彻底卸载
systemctl restart cliproxyapi

# 4) 复核
curl -s -H "Authorization: Bearer <client-key>" http://127.0.0.1:8317/v1/models | \
  python3 -c 'import sys,json;print(len(json.load(sys.stdin)["data"]))'   # 回到安装前的数量
ls /root/cliproxyapi/data/cap-opencode-go 2>/dev/null || echo "无残留"
```

## 3. 也可以在管理中心里卸载

`插件管理` 页面提供删除按钮（`DELETE /v8/management/plugins/<id>`），它会同时删除 `.so` 与
对应的 `plugins.configs` 块。对已加载的插件该接口可能返回
`409 plugin_delete_requires_restart`——此时先删配置块并重启，再删除文件即可。
四个实例需要分别处理，或直接用上面的脚本一次删净。

## 4. 回滚验证结果

在隔离实例（`/tmp/cpa-test`）上按上述流程执行后：

| 检查项 | 结果 |
|---|---|
| `GET /v1/models` | 0 个模型（回到基线） |
| `GET /v8/management/plugins` | `plugins = []` |
| `GET /v0/resource/plugins/cap-opencode-go/dashboard` | 404 |
| `<root>/data/cap-opencode-go/` | 不存在 |
| `<root>/plugins/linux/amd64/*.so` | 0 个 |
| `<root>/auth` 目录 | 0 个文件（插件从未写入凭据文件） |

## 5. v1.2.0 之后仍然零残留

v1.2.0 只改了页面代码与新增一个**由插件自己声明的管理路由**，没有引入任何新的落盘物：

| 新东西 | 卸载后 |
|---|---|
| 顶部操作栏 / 逐模型设置面板 / 搜索框 | 只存在于 `.so` 里的 HTML 字符串 |
| 复用管理中心登录态读密钥 | 只发生在浏览器内存与 fetch 请求头里，插件不保存 |
| `GET /v0/management/opencode-go/probe` | 路由声明随 `.so` 一起消失（不写 config.yaml） |
| `--host-header` 顶部内移 | 运行时读取父页面高度，**不修改**宿主页面（无注入、无样式残留） |

因此回滚清单与 v1.1.1 完全一致：4 个 `.so` + 4 个 `plugins.configs.*` 块 + `data/cap-opencode-go/`。

## 6. 常见疑问

**会不会影响我原有的 `openai-compatibility` / `gemini` 组？**
不会。插件只通过 `model_router` 在运行期认领模型 ID，不读写这些配置块。

**同名模型（例如 `gpt-6-luna`）回滚后会怎样？**
立即回到插件加载前的解析路径（例如由 codex OAuth 凭据提供）；插件卸载后其路由声明一并消失。

**删除后 `/v1/models` 里还有 `opencode` 归属的模型？**
说明仍有 `.so` 被加载或 `plugins.configs` 块未删除，重启后再确认一次。
