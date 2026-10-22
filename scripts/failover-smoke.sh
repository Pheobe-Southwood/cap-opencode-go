#!/usr/bin/env bash
# Isolated end-to-end check that CPA (not the plugin) performs key failover:
# two keys are projected into host auth files, key A returns 429, and the same
# client request must still be served by key B.
#
# It needs a CLIProxyAPI binary and a built chat plugin:
#   CPA_BIN=/root/cliproxyapi/cli-proxy-api \
#   PLUGIN_SO=dist/cpa-opencodego-v2.0.3.so \
#   bash scripts/failover-smoke.sh
#
# Nothing outside the temporary directory is touched. Missing inputs are a skip,
# not a failure, so the script is safe to leave in the repository.
set -euo pipefail

CPA_BIN="${CPA_BIN:-/root/cliproxyapi/cli-proxy-api}"
PLUGIN_SO="${PLUGIN_SO:-dist/cpa-opencodego-v2.0.3.so}"
PORT="${PORT:-18399}"
UPSTREAM_PORT="${UPSTREAM_PORT:-18400}"
CLIENT_KEY="sk-client"
MODEL="${MODEL:-deepseek-v4.1-flash}"

if [ ! -x "${CPA_BIN}" ]; then
  echo "skip: CPA binary not found: ${CPA_BIN}"
  exit 0
fi
if [ ! -f "${PLUGIN_SO}" ]; then
  echo "skip: plugin not built: ${PLUGIN_SO}"
  exit 0
fi
for tool in curl python3; do
  if ! command -v "${tool}" >/dev/null 2>&1; then
    echo "skip: ${tool} is required"
    exit 0
  fi
done

TMP="$(mktemp -d "${TMPDIR:-/tmp}/cpa-failover.XXXXXX")"
HOST_PID=""
UPSTREAM_PID=""
cleanup() {
  [ -n "${HOST_PID}" ] && kill "${HOST_PID}" 2>/dev/null || true
  [ -n "${UPSTREAM_PID}" ] && kill "${UPSTREAM_PID}" 2>/dev/null || true
  wait 2>/dev/null || true
  rm -rf "${TMP}"
}
trap cleanup EXIT INT TERM

mkdir -p "${TMP}/auth" "${TMP}/state" "${TMP}/plugins/linux/amd64"
cp "${PLUGIN_SO}" "${TMP}/plugins/linux/amd64/cpa-opencodego.so"

cat > "${TMP}/upstream.py" <<'PY'
import http.server
import json
import sys

class Handler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0") or "0")
        self.rfile.read(length)
        authorization = self.headers.get("Authorization", "")
        if "sk-a" in authorization:
            payload = {
                "error": {
                    "message": "GoUsageLimitError: Go usage limit exceeded",
                    "type": "rate_limit_error",
                    "code": "rate_limit_exceeded",
                }
            }
            status = 429
        else:
            payload = {
                "id": "chatcmpl-failover-test",
                "object": "chat.completion",
                "model": "deepseek-v4.1-flash",
                "choices": [{
                    "index": 0,
                    "message": {"role": "assistant", "content": "served by B"},
                    "finish_reason": "stop",
                }],
                "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
            }
            status = 200
        body = json.dumps(payload).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass

http.server.HTTPServer(("127.0.0.1", int(sys.argv[1])), Handler).serve_forever()
PY

cat > "${TMP}/config.yaml" <<YAML
server:
  port: ${PORT}
access:
  api-keys:
    - "${CLIENT_KEY}"
routing:
  strategy: "round-robin"
oauth:
  auth-dir: "${TMP}/auth"
  retry:
    request-retry: 3
    max-retry-credentials: 0
    max-retry-interval: 30
  cooldown:
    disable-cooling: false
    save-cooldown-status: false
plugins:
  enabled: true
  dir: "${TMP}/plugins"
  configs:
    cpa-opencodego:
      enabled: true
      base-url: "http://127.0.0.1:${UPSTREAM_PORT}"
      endpoint-layout: "legacy"
      state-dir: "${TMP}/state"
      keys:
        - {name: "A", key: "sk-a", priority: 10}
        - {name: "B", key: "sk-b", priority: 1}
YAML

python3 "${TMP}/upstream.py" "${UPSTREAM_PORT}" >"${TMP}/upstream.log" 2>&1 &
UPSTREAM_PID=$!
# Run the host from the temporary directory: plugin-owned shared state and the
# request log default to ./data/cpa-opencodego when state-dir is not honored.
( cd "${TMP}" && exec "${CPA_BIN}" -config "${TMP}/config.yaml" ) >"${TMP}/host.log" 2>&1 &
HOST_PID=$!

fail() {
  echo "FAIL: $*" >&2
  echo "--- host.log (tail) ---" >&2
  tail -n 60 "${TMP}/host.log" >&2 || true
  exit 1
}

ready=0
for _ in $(seq 1 100); do
  if ! kill -0 "${HOST_PID}" 2>/dev/null; then
    fail "CPA exited before becoming ready"
  fi
  if curl -fsS -m 1 -H "Authorization: Bearer ${CLIENT_KEY}" \
      "http://127.0.0.1:${PORT}/v1/models" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 0.1
done
[ "${ready}" = "1" ] || fail "CPA did not answer /v1/models"

# The projection is asynchronous: the plugin learns the auth directory from the
# model registration summary, writes one file per key and protocol face, and the
# host watcher loads them.
projected=0
for _ in $(seq 1 100); do
  count=$(find "${TMP}/auth" -maxdepth 1 -name 'opencode-go-*.json' 2>/dev/null | wc -l | tr -d ' ')
  if [ "${count}" = "8" ]; then
    projected=1
    break
  fi
  sleep 0.1
done
[ "${projected}" = "1" ] || fail "plugin projected ${count:-0} auth files, want 8 (2 keys x 4 protocol faces)"

python3 - "${TMP}/auth" <<'PY' || fail "projected auth files are incomplete"
import json, os, sys
auth_dir = sys.argv[1]
files = sorted(name for name in os.listdir(auth_dir) if name.startswith("opencode-go-"))
if len(files) != 8:
    raise SystemExit("unexpected files: %r" % files)
expected = {"opencode-go-a.json": ("A", "sk-a", 10), "opencode-go-b.json": ("B", "sk-b", 1)}
for name, (key_name, api_key, priority) in expected.items():
    with open(os.path.join(auth_dir, name), encoding="utf-8") as handle:
        payload = json.load(handle)
    if payload.get("type") != "opencode-go":
        raise SystemExit("%s: type=%r" % (name, payload.get("type")))
    if payload.get("key_name") != key_name or payload.get("api_key") != api_key:
        raise SystemExit("%s: credential fields are wrong" % name)
    if payload.get("priority") != priority:
        raise SystemExit("%s: priority=%r, want %r" % (name, payload.get("priority"), priority))
    if payload.get("managed_by") != "cpa-opencodego":
        raise SystemExit("%s: marker=%r" % (name, payload.get("managed_by")))
PY

# The watcher processes the eight CREATE events incrementally; give it a beat
# before the first routed request.
sleep 0.5

status=$(curl -sS -o "${TMP}/response.json" -w '%{http_code}' -m 60 \
  -H "Authorization: Bearer ${CLIENT_KEY}" -H "Content-Type: application/json" \
  -d "{\"model\":\"${MODEL}\",\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}" \
  "http://127.0.0.1:${PORT}/v1/chat/completions")
[ "${status}" = "200" ] || fail "chat request returned ${status}: $(head -c 300 "${TMP}/response.json")"

LOG="$(find "${TMP}" -name 'requests-chat.jsonl' -print -quit 2>/dev/null || true)"
[ -n "${LOG}" ] || fail "plugin request log was not written"

python3 - "${LOG}" <<'PY' || fail "the request log does not show CPA's 429 -> 200 failover"
import json, sys
rows = [json.loads(line) for line in open(sys.argv[1], encoding="utf-8") if line.strip()]
if len(rows) < 2:
    raise SystemExit("rows=%d, want at least 2 (A failed, B succeeded)" % len(rows))
first, second = rows[0], rows[1]
if first.get("key") != "A" or first.get("status") != 429:
    raise SystemExit("first row = %r, want A/429" % first)
if second.get("key") != "B" or second.get("status") != 200:
    raise SystemExit("second row = %r, want B/200" % second)
PY

echo "PASS: CPA failed over A(429) -> B(200); plugin projected 8 auth files (2 keys x 4 faces)"
