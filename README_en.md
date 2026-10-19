<div align="center">

# OpenCode Go Provider Plugin

**Seamlessly integrate all 37 OpenCode Go models into CLIProxyAPI as a native, zero-residue plugin**

[ 简体中文 ](README.md) | [ English ](README_en.md) | [ 日本語 ](README_ja.md) | [ Français ](README_fr.md) | [ 한국어 ](README_ko.md)

<p>
  <img src="https://img.shields.io/badge/CPA-v8.0.27+-blue?style=flat-square" alt="CPA Version">
  <img src="https://img.shields.io/badge/OpenCode_Go-37_Models-success?style=flat-square" alt="Models">
  <img src="https://img.shields.io/badge/Protocol-4_Families-orange?style=flat-square" alt="Protocols">
  <img src="https://img.shields.io/badge/License-MIT-lightgrey?style=flat-square" alt="License">
</p>

</div>

---

> **v2.0.1**: fixes the intermittent `reasoning_content` HTTP 400 on long DeepSeek thinking-mode runs (history backfill + one retry). v2.0.0 renamed the plugin id and repository to `cpa-opencodego`, added the request-log tab, and added the new OpenCode Go per-family endpoints (the legacy `/v1` endpoints keep working). Read the [v2 upgrade guide](docs/UPGRADE-v2.md) before upgrading.


## Core Value & Key Highlights

OpenCode Go hosts 37 leading AI models (DeepSeek V4.1, Claude Haiku 5.5, Gemini 4 Argon, GPT 6 Luna, Grok 4.7, Qwen 3.8, Kimi K3, and more). However, upstream models are distributed across 4 incompatible protocol endpoints, and different models demand strict routing policies.

This plugin delivers a unified, turnkey integration for CLIProxyAPI (CPA):
- **Universal Protocol Coverage**: Unifies `chat/completions`, `responses`, `messages`, and `generateContent`. Downstream clients can query any model using any protocol format.
- **Granular Per-Model Proxy Control**: Choose between **Proxy**, **Direct connection**, or **Inherit global** independently for each model to satisfy regional compliance and performance needs.
- **Native Embedded Management WebUI**: Manage everything directly from the CPA sidebar — weighted multi-key pools, real-time quota progress bars, and live throughput/latency analytics.
- **Zero-Invasion & Zero Residue**: Runs as standalone dynamic libraries (`.so`) without patching CPA source code or touching internal databases. Clean uninstallation leaves no trace.

---

## Feature Tour

### 1. Per-Model Proxy Selection & Model Aliasing

> **Highlight**: Configure customized egress routing rules model by model, and assign public aliases for unified client access.

![Per-Model Proxy Selection & Model Aliasing](docs/screenshots/desktop-models.png)

- **Dedicated Proxy per Model**:
  - `proxy`: Route through an egress proxy (essential for regional-restricted models like GPT 6 Luna and Grok 4.7).
  - `direct`: Force direct internet connection, bypassing proxy hops (e.g., Gemini 4 Argon Contributor requires direct traffic to prevent 403 errors).
  - `inherit`: Seamlessly fall back to plugin-level or CPA host global proxy configurations.
- **Model Renaming (Aliases)**: Assign client-facing names (e.g., exposing `deepseek-v4.1-flash` as `DeepSeek-V4.1-Flash`). Downstream model lists and router mappings automatically adopt the alias.
- **Automatic Protocol Inference & Probing**: New upstream models are automatically mapped by prefix heuristics, with a one-click "Probe Protocol" tool to empirically verify upstream compatibility.

---

### 2. Multi-Key Pool & Live Multi-Window Quota Tracking

> **Highlight**: Real-time quota visibility across 5-hour rolling, weekly, and monthly windows with prioritized multi-key scheduling.

![Multi-Key Pool & Real-Time Quota Tracking](docs/screenshots/desktop-accounts.png)

- **Live Multi-Window Quota Visualization**: Queries official upstream `/usage` metrics directly. Displays rolling 5-hour, weekly, and monthly remaining percentages alongside reset countdown timers.
- **Priority-Based Key Scheduling**: Add multiple keys with priority ratings. CPA automatically handles failover and load distribution.
- **In-Place Key Renaming**: Click any key label to rename it instantly. Settings hot-reload automatically without server downtime.

---

### 3. Comprehensive Usage & Performance Analytics

> **Highlight**: Full visibility into token consumption, list price conversions, end-to-end TPS, pure generation TPS, and time-to-first-token (TTFT).

![Comprehensive Usage & Performance Analytics](docs/screenshots/desktop-stats.png)

- **Empirical Performance Metrics**:
  - **Overall TPS**: Effective output tokens divided by total host round-trip time, capturing genuine end-to-end system throughput.
  - **Generation TPS**: Measured from first content frame to last content frame, benchmarking pure model decoding speed.
  - **Average TTFT**: Precise timing from upstream request dispatch to the first substantive token.
- **Cost & Quota Projections**: Automatic valuation based on embedded official pricing sheets for Go and Go Plus tiers, detailing per-key quota utilization per model.
- **Staleness Alerts**: Proactively prompts for price verification every 30 days or whenever upstream model catalogs change.
- **Per-Request Log**: A new tab lists every upstream attempt with its time, input/output/total tokens, TTFT, generation TPS, end-to-end TPS, status and error. Failed requests and the upstream `x-opencode-log-id` are visible for troubleshooting.

---

### 4. 4 Unified Protocol Faces

> **Highlight**: One coherent product spanning all four API protocol families, eliminating downstream protocol mismatch.

![4 Unified Protocol Faces](docs/screenshots/desktop-status.png)

- **Unified Operation**: The main plugin handles the WebUI and configuration source of truth, while the 3 subordinate protocol faces (Responses, Messages, Gemini) share runtime state automatically.
- **Seamless Cross-Protocol Translation**: OpenAI-format clients can call Claude models; Anthropic-format clients can query Gemini 4 or DeepSeek effortlessly.
- **Zero-Config Protocol Faces**: Only the main face requires configuration; subordinate faces run out-of-the-box.

---

## Quick Start

### Step 1: Deploy Plugin Libraries

Copy the 4 compiled `.so` files to your CPA plugins folder (typically `plugins/linux/amd64/`):
- `cpa-opencodego-v2.0.1.so`
- `cpa-opencodego-responses-v2.0.1.so`
- `cpa-opencodego-messages-v2.0.1.so`
- `cpa-opencodego-gemini-v2.0.1.so`

### Step 2: Add Configuration

Append the plugin block to `plugins:` in CPA's `config.yaml`:

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    cpa-opencodego:
      enabled: true
      base-url: "https://opencode.ai/inference/go"
      endpoint-layout: "auto"
      developer-role: "rewrite"
      reasoning-replay: "auto"   # 补齐思考模型历史 reasoning_content，修偶发 400
      request-log: true
      keys:
        - name: "primary-key"
          key: "oc_sk_your_api_key_here"
          priority: 10
          enabled: true
    cpa-opencodego-responses: { enabled: true }
    cpa-opencodego-messages:  { enabled: true }
    cpa-opencodego-gemini:    { enabled: true }
```

### Step 3: Manage & Enjoy

1. Launch CLIProxyAPI. For upgrades the replaced `.so` files and the config hot-reload in place, so no restart is needed.
2. Open the CPA Management WebUI and navigate to **OpenCode Go** under the "Plugins" menu.
3. Manage models, toggle custom proxies, and observe live quota and performance statistics!

---

## Technical Roadmap & Architecture

For deep-dive technical architecture, C ABI runtime protocols, SSE first-frame timing internals, comprehensive configuration references, and compilation guides, please visit:

- 📖 [Technical Architecture & Full Configuration (docs/TECHNICAL.md)](docs/TECHNICAL.md)
- 📐 [Design Specifications (docs/design.md)](docs/design.md)
- 🛡️ [Zero-Residue Rollback Guide (docs/rollback.md)](docs/rollback.md)
- 🧪 [Acceptance Test Suite & Reports (docs/acceptance.md)](docs/acceptance.md)

---

## License

This project is licensed under the [MIT License](LICENSE.txt).
