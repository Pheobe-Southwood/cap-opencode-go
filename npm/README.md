# cpa-opencodego

Install the **OpenCode Go** provider plugin for [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI).

The plugin is four CGO shared libraries (one per upstream protocol family), so npm
is used as a small, dependency-free installer. The libraries themselves are
downloaded from the matching [GitHub release](https://github.com/Pheobe-Southwood/cpa-opencodego/releases)
and verified against its `checksums.txt` before they are written.

## Install

```bash
npx cpa-opencodego install --dir /opt/cliproxyapi
```

Or install it globally and reuse it:

```bash
npm install -g cpa-opencodego
cpa-opencodego install --dir /opt/cliproxyapi
```

The installer detects `linux`/`darwin`/`windows` and `amd64`/`arm64`, downloads
the four libraries, and writes them to:

```text
<CLIProxyAPI root>/plugins/<goos>/<goarch>/
```

### Options

| Option | Description |
| --- | --- |
| `--dir <path>` | CLIProxyAPI root directory. Defaults to `$CPA_ROOT` or the current directory. |
| `--plugins-dir <path>` | Override the plugin directory. Defaults to `<root>/plugins`. |
| `--version <version>` | Install a specific release instead of the latest one. |
| `--face <name>` | Install one face: `chat`, `responses`, `messages`, `gemini`, or `all` (default). |
| `--force` | Rewrite libraries even when the installed bytes are identical. |
| `--dry-run` | Resolve the release and print the plan without downloading. |

## Enable the plugin

Add the block printed by the installer to `config.yaml`:

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    cpa-opencodego:
      enabled: true
      base-url: "https://opencode.ai/inference/go"
      endpoint-layout: "auto"
      keys:
        - name: "primary-key"
          key: "oc_sk_your_api_key_here"
          priority: 10
          enabled: true
    cpa-opencodego-responses: { enabled: true }
    cpa-opencodego-messages: { enabled: true }
    cpa-opencodego-gemini: { enabled: true }
```

Restart CLIProxyAPI (or replace the libraries in place — the host hot-reloads
plugin files) and open **Plugins → OpenCode Go** in the management console.

## What the plugin adds

- All **37 OpenCode Go models** behind one shared key pool.
- Four protocol faces: OpenAI chat-completions, OpenAI Responses (Codex),
  Anthropic Messages, and Google GenAI.
- Per-model proxy routing and alias renaming.
- Live 5-hour / weekly / monthly quota tracking.
- Token, cost, TPS, TTFT and per-request logs in an embedded dashboard.

## Links

- [Repository and documentation](https://github.com/Pheobe-Southwood/cpa-opencodego)
- [Releases](https://github.com/Pheobe-Southwood/cpa-opencodego/releases)
- [Issues](https://github.com/Pheobe-Southwood/cpa-opencodego/issues)

## License

MIT. The embedded OpenCode icon is © 2025 opencode and used descriptively under
the MIT license; it is not covered by this package's license.
