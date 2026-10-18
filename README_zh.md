<div align="center">

# OpenCode Go Provider 插件

**将 OpenCode Go 全量 37 个模型无缝接入 CLIProxyAPI 的原生插件**

[ 简体中文 ](README.md) | [ English ](README_en.md) | [ 日本語 ](README_ja.md) | [ Français ](README_fr.md) | [ 한국어 ](README_ko.md)

<p>
  <img src="https://img.shields.io/badge/CPA-v8.0.27+-blue?style=flat-square" alt="CPA Version">
  <img src="https://img.shields.io/badge/OpenCode_Go-37_Models-success?style=flat-square" alt="Models">
  <img src="https://img.shields.io/badge/Protocol-4_Families-orange?style=flat-square" alt="Protocols">
  <img src="https://img.shields.io/badge/License-MIT-lightgrey?style=flat-square" alt="License">
</p>

</div>

---

## 插件亮点与核心价值

OpenCode Go 聚合了大量顶级大语言模型（DeepSeek V4.1、Claude Haiku 5.5、Gemini 4 Argon、GPT 6 Luna、Grok 4.7、Qwen 3.8、Kimi K3 等），但不同模型分散在 4 个完全不同的上游协议端点，且各模型对网络代理环境要求严苛。

本插件为 CLIProxyAPI (CPA) 带来了开箱即用的完整整合方案：
- **四大协议族全覆盖**：统一接入 `chat/completions`、`responses`、`messages` 与 `generateContent` 四大端点，客户端用任意协议都能无缝调用所有模型。
- **逐模型独立代理通道**：每个模型均可独立设定为「走代理」、「直连」或「继承全局」，完美应对不同地区路由策略。
- **原生内嵌控制台**：在 CPA 管理中心拥有独立可视化操作面板，多 Key 池加权调度、实时上游额度监控、吞吐速率及首字延迟统计一目了然。
- **纯净安全零残留**：纯动态库架构，不入侵 CPA 源码，不污染数据库，卸载后无残留。

---

## 功能特性导览

### 1. 逐模型独立代理与别名重命名

> **核心亮点**：支持针对每个模型单独指定网络出口策略，并支持重命名别名统一对外调度。

![逐模型独立代理与别名定制](docs/screenshots/desktop-models.png)

- **每模型各自选代理**：
  - `走代理 (proxy)`：精准指定需要代理的中转模型（如 GPT 6 Luna、Grok 4.7 等 Responses 家族模型）。
  - `直连 (direct)`：强制绕过代理，保障特定直连端点的顺畅访问（如 Gemini 4 Argon Contributor 强制直连，防止地区限制报错）。
  - `继承 (inherit)`：无缝沿用插件级或 CPA 宿主全局代理配置。
- **重命名别名**：为模型设置对外标识（如将 `deepseek-v4.1-flash` 重命名为 `DeepSeek-V4.1-Flash`），客户端列表与路由完全认领别名。
- **智能协议推断与实测**：上游新出模型自动根据 ID 前缀推测所属端点，同时提供「探测协议」按钮发送微量请求进行实测验证。

---

### 2. 多账号 Key 池与实时多窗口额度追踪

> **核心亮点**：直观掌握上游 5 小时、本周与本月多维度配额余量，支持多 Key 优先级调度。

![多 Key 池与实时额度看板](docs/screenshots/desktop-accounts.png)

- **多窗口配额可视化**：直接从 OpenCode Go 上游 `/usage` 接口拉取真实额度，同时呈现 5 小时滚动、周额度、月额度剩余百分比与刷新倒计时。
- **优先级加权调度**：支持配置多个 Key，按照优先级降序调度与故障转移，主备切换平滑无感。
- **即时就地改名**：直接在界面中点击 Key 名称即可修改，配置自动热重载，无需重启服务。

---

### 3. 全维度用量分析与真实性能统计

> **核心亮点**：Token 消耗、刊例价折算、端到端整体 TPS、解码生成 TPS 与首字延迟（TTFT）全量透视。

![全维度用量与性能监控](docs/screenshots/desktop-stats.png)

- **多维性能量化**：
  - **整体 TPS**：成功请求的有效输出 ÷ 宿主总耗时，反映端到端真实吞吐表现。
  - **生成 TPS**：插件实测的首个内容帧到最后一个内容帧耗时，精确度量模型纯解码速度。
  - **平均 TTFT**：精准捕捉发起请求到接收首个有效字耗时，直观反映网络建连与排队开销。
- **费用与配额进度**：内置官方价目快照，按 Go / Go Plus 套餐智能推算逐模型月度限额消耗进度，并在每行清晰展示各 Key 的余量分布。
- **更新提醒**：内置价目表每 30 天或上游模型变动时在面板顶部温和提醒核对，保持数据准确。

---

### 4. 四大协议族一体化架构

> **核心亮点**：一套配置，同时驱动四大协议面，让客户端彻底摆脱协议异构限制。

![四大协议族一体化架构](docs/screenshots/desktop-status.png)

- **四面合一**：主面负责控制台与统一配置，从属面（Responses、Messages、Gemini）自动共享快照，协同提供服务。
- **跨协议调用**：OpenAI 格式客户端可直接调用 Claude 模型；Claude 格式客户端亦可直接调用 Gemini 4 或 DeepSeek。
- **极简维护**：仅主面需要配置，从属面零配置运行，架构清晰统一。

---

## 快速使用

### 第一步：获取与部署插件

将编译好的 4 个 `.so` 文件复制到 CPA 宿主的插件目录（通常为 `plugins/linux/amd64/`）：
- `cap-opencode-go-v1.12.0.so`
- `cap-opencode-go-responses-v1.12.0.so`
- `cap-opencode-go-messages-v1.12.0.so`
- `cap-opencode-go-gemini-v1.12.0.so`

### 第二步：添加宿主配置

在 CPA 的 `config.yaml` 的 `plugins:` 节点中追加启用配置：

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    cap-opencode-go:
      enabled: true
      base-url: "https://opencode.ai/inference/go/v1"
      keys:
        - name: "primary-key"
          key: "oc_sk_your_api_key_here"
          priority: 10
          enabled: true
    cap-opencode-go-responses: { enabled: true }
    cap-opencode-go-messages:  { enabled: true }
    cap-opencode-go-gemini:    { enabled: true }
```

### 第三步：在控制台畅享使用

1. 启动或重启 CLIProxyAPI 服务。
2. 打开 CPA 管理面板，在左侧导航栏「插件」中点击 **OpenCode Go** 入口。
3. 即可在可视化界面中管理所有模型开关、为各模型挑选代理策略、监控实时额度与性能指标！

---

## 深入技术路线与架构

想要了解 4 个 `.so` 的底层设计原理、C ABI 通讯、流式响应首帧捕获机制、全量配置参数或二次构建说明，请阅读详细文档：

- 📖 [技术路线与全量配置文档 (docs/TECHNICAL.md)](docs/TECHNICAL.md)
- 📐 [核心设计方案 (docs/design.md)](docs/design.md)
- 🛡️ [零残留与回滚指南 (docs/rollback.md)](docs/rollback.md)
- 🧪 [验收与实测报告 (docs/acceptance.md)](docs/acceptance.md)

---

## 开源协议

本项目采用 [MIT 许可证](LICENSE.txt)。
