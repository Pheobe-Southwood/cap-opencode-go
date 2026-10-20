# 发布、Tag 与插件商店管理

本仓库按社区 CPA 插件的通用方式发布：**语义化 Git tag → GitHub Actions 构建多平台动态库 → GitHub Release 附件 + `checksums.txt` → 插件商店 registry 条目**。npm 包 `cpa-opencodego` 只是这些 release 的安装器，不重复打包二进制。

---

## 1. 版本号与 Tag

- 版本号遵循 `MAJOR.MINOR.PATCH`，与 `Makefile` 的 `VERSION`、README 中的文件名一致。
- Git tag 必须是 `v<version>`（例如 `v2.0.2`），不能带其他前缀。
- 插件商店要求 release tag 为 `v<version>`，并从最新 release 读取版本；因此**不要**创建不带 `v` 的正式 release。

```bash
# 确认工作区干净且测试通过
make test
make vet

git tag -a v2.0.2 -m "v2.0.2"
git push origin v2.0.2
```

推送 tag 会触发 [`.github/workflows/release.yml`](../.github/workflows/release.yml)：

1. `test`：`go test ./...` + `go vet ./...`
2. `build`：linux/amd64、linux/arm64、darwin/amd64、darwin/arm64、windows/amd64 五个平台，各编译 4 个协议面
3. `release`：汇总附件并创建 / 更新 GitHub Release

也可以在 Actions 页面用 `workflow_dispatch` 指定一个已经存在的 tag 重新发布。

---

## 2. Release 附件命名

每个平台生成：

```text
cpa-opencodego_<version>_<goos>_<goarch>.zip                 # 主面（chat）
cpa-opencodego-responses_<version>_<goos>_<goarch>.zip       # Responses 面
cpa-opencodego-messages_<version>_<goos>_<goarch>.zip        # Messages 面
cpa-opencodego-gemini_<version>_<goos>_<goarch>.zip          # Gemini 面
cpa-opencodego-bundle_<version>_<goos>_<goarch>.zip          # 四面合集（手动安装 / npm）
checksums.txt                                                 # 所有 zip 的 sha256
```

每个 zip 根目录只放一个动态库（`cpa-opencodego*.so|dylib|dll`），符合 CLIProxyAPI 插件商店的安装约束。四个协议面需要一起启用，因此普通用户应使用 bundle 或 npm 安装器。

本地复现：

```bash
make release-linux-amd64 VERSION=2.0.2
ls dist/release
```

---

## 3. 插件商店 registry

官方商店仓库是 [`router-for-me/CLIProxyAPI-Plugins-Store`](https://github.com/router-for-me/CLIProxyAPI-Plugins-Store)，只维护 `registry.json`。本仓库根目录的 [`registry.json`](../registry.json) 保存同一份条目，便于第三方管理器读取和复制。

条目字段：

| 字段 | 值 |
| --- | --- |
| `id` | `cpa-opencodego`（主面）、`cpa-opencodego-responses`、`cpa-opencodego-messages`、`cpa-opencodego-gemini` |
| `name` | `OpenCode Go` / `OpenCode Go · Responses` / `· Messages` / `· Gemini` |
| `author` | `Pheobe-Southwood` |
| `repository` | `https://github.com/Pheobe-Southwood/cpa-opencodego` |
| `logo` | `https://raw.githubusercontent.com/Pheobe-Southwood/cpa-opencodego/main/assets/logo.png` |
| `license` | `MIT` |
| `tags` | 主面：`Provider`, `OpenCode`, `Management`, `Usage`, `Quota`, `Dashboard`；从属面追加对应协议标签 |

四个协议面各占一个 registry 条目，因为 CLIProxyAPI 的插件 id 来自库文件名，且商店安装器一次只安装一个动态库。四个条目指向同一仓库和同一 release；安装全部四个才能获得完整 37 模型目录。

发布新版本后，向商店仓库提交只修改 `registry.json` 的 PR，附上 release 链接、tag 与附件证据即可。商店读取最新 release，不需要为每个版本改 registry。

---

## 4. npm 包

`npm/` 是独立的安装器包：

```bash
cd npm
npm test
npm pack --dry-run
npm publish --access public
```

发布前必须确保对应版本的 GitHub Release 已存在，否则安装器无法下载附件。包内只包含 CLI、README 与 LICENSE，不包含二进制。

---

## 5. 品牌资源

- `assets/logo.svg` / `assets/logo.png`：OpenCode Go 官方方形图标，用于 README、商店 `logo` 与插件元数据。
- `assets/social-preview.png`：1280×640 仓库社交预览图（GitHub 仓库设置中手动上传）。
- `internal/brand/opencode.svg`：编译进插件的同一图标；修改时需同步 `assets/logo.svg` 并运行 `go test ./internal/brand`。

OpenCode 图标 © 2025 opencode，MIT 许可，仅用于描述本插件对接的上游服务。
