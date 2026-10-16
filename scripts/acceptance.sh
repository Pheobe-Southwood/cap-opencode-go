#!/usr/bin/env bash
# End-to-end acceptance run against a CLIProxyAPI instance.
#
# Usage:
#   CPA_BASE=http://127.0.0.1:8317 \
#   CPA_CLIENT_KEY=sk-xxx \
#   CPA_MGMT_KEY=mgmt-key \
#   bash scripts/acceptance.sh
#
# The management key is optional: it is only needed for the plugin-list check.
# Every protocol check asserts a non-empty body, not just the status code,
# because a translation gap surfaces as an empty HTTP 200.
set -uo pipefail

BASE="${CPA_BASE:-http://127.0.0.1:8399}"
CLIENT_KEY="${CPA_CLIENT_KEY:-}"
MGMT_KEY="${CPA_MGMT_KEY:-}"
BODY_FILE="/tmp/cpa-opencodego-acceptance-body.txt"

if [ -z "${CLIENT_KEY}" ]; then
  echo "CPA_CLIENT_KEY is required" >&2
  exit 2
fi

PASS=0
FAIL=0

# check <label> <expected-status> <method> <path> <body> [marker]
check() {
  local label="$1" expected="$2" method="$3" path="$4" payload="${5:-}" marker="${6:-}"
  local args=(-s -o "${BODY_FILE}" -w '%{http_code}' -X "${method}" "${BASE}${path}"
              -H "Authorization: Bearer ${CLIENT_KEY}" -H 'Content-Type: application/json')
  if [ -n "${payload}" ]; then
    args+=(-d "${payload}")
  fi
  local status size
  status="$(timeout 180 curl "${args[@]}" || echo 000)"
  size="$(wc -c < "${BODY_FILE}" | tr -d ' ')"

  if [ "${status}" != "${expected}" ]; then
    printf '  FAIL  %-46s got %s want %s\n' "${label}" "${status}" "${expected}"
    printf '        %s\n' "$(head -c 200 "${BODY_FILE}")"
    FAIL=$((FAIL + 1))
    return
  fi
  if [ -n "${marker}" ] && ! grep -q -- "${marker}" "${BODY_FILE}"; then
    printf '  FAIL  %-46s status ok but body lacks %s (%s bytes)\n' "${label}" "${marker}" "${size}"
    printf '        %s\n' "$(head -c 200 "${BODY_FILE}")"
    FAIL=$((FAIL + 1))
    return
  fi
  printf '  PASS  %-46s %s  %s bytes\n' "${label}" "${status}" "${size}"
  PASS=$((PASS + 1))
}

echo "== OpenCode Go acceptance @ ${BASE} =="

# The plugin only answers for models the operator enabled, so pick one enabled
# model per protocol family from the live state instead of assuming a fixed set.
STATE_JSON="$(timeout 60 curl -s "${BASE}/v0/resource/plugins/cap-opencode-go/api/state" 2>/dev/null || echo '{}')"

pick() {
  printf '%s' "${STATE_JSON}" | python3 -c '
import sys, json
family = sys.argv[1]
try:
    payload = json.load(sys.stdin)
except Exception:
    raise SystemExit
for row in payload.get("models", []):
    if row.get("enabled") and row.get("family") == family:
        # An alias replaces the model id, so the id clients must call is the
        # first client-facing one; the configured id is only the config key.
        clients = row.get("client_ids") or []
        print(clients[0] if clients else row["id"])
        break
' "$1" 2>/dev/null || true
}

CHAT_MODEL="$(pick chat)"
RESP_MODEL="$(pick responses)"
GEMINI_MODEL="$(pick gemini)"
MSG_MODEL="$(pick messages)"

printf 'served models: chat=%s responses=%s gemini=%s messages=%s\n\n' \
  "${CHAT_MODEL:-none}" "${RESP_MODEL:-none}" "${GEMINI_MODEL:-none}" "${MSG_MODEL:-none}"

skip() { printf '  SKIP  %-46s %s\n' "$1" "$2"; }

# --- chat/completions family -------------------------------------------------
if [ -n "${CHAT_MODEL}" ]; then
  check "chat/completions non-stream" 200 POST /v1/chat/completions \
    "{\"model\":\"${CHAT_MODEL}\",\"max_tokens\":16,\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}" '"choices"'
  check "chat/completions stream" 200 POST /v1/chat/completions \
    "{\"model\":\"${CHAT_MODEL}\",\"max_tokens\":8,\"stream\":true,\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}" 'data:'
  check "chat model via claude client" 200 POST /v1/messages \
    "{\"model\":\"${CHAT_MODEL}\",\"max_tokens\":8,\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}" '"content"'
  check "chat model via responses client" 200 POST /v1/responses \
    "{\"model\":\"${CHAT_MODEL}\",\"input\":\"hi\"}" '"object":"response"'
else
  skip "chat/completions family" "no chat model enabled"
fi

# --- responses family --------------------------------------------------------
if [ -n "${RESP_MODEL}" ]; then
  check "responses non-stream" 200 POST /v1/responses \
    "{\"model\":\"${RESP_MODEL}\",\"input\":\"hi\"}" '"object":"response"'
  check "responses stream" 200 POST /v1/responses \
    "{\"model\":\"${RESP_MODEL}\",\"input\":\"hi\",\"stream\":true}" 'response.'
  check "responses model via chat client" 200 POST /v1/chat/completions \
    "{\"model\":\"${RESP_MODEL}\",\"max_tokens\":16,\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}" '"choices"'
  check "responses model via claude client" 200 POST /v1/messages \
    "{\"model\":\"${RESP_MODEL}\",\"max_tokens\":16,\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}" '"content"'
else
  skip "responses family" "no responses model enabled"
fi

# --- gemini family (the reason this plugin exists) ---------------------------
if [ -n "${GEMINI_MODEL}" ]; then
  check "gemini generateContent" 200 POST "/v1beta/models/${GEMINI_MODEL}:generateContent" \
    '{"contents":[{"parts":[{"text":"hi"}]}]}' '"candidates"'
  check "gemini streamGenerateContent" 200 POST "/v1beta/models/${GEMINI_MODEL}:streamGenerateContent" \
    '{"contents":[{"parts":[{"text":"hi"}]}]}' '"candidates"'
  check "gemini model via chat client" 200 POST /v1/chat/completions \
    "{\"model\":\"${GEMINI_MODEL}\",\"max_tokens\":16,\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}" '"choices"'
else
  skip "gemini family" "no gemini model enabled"
fi

# --- messages family ---------------------------------------------------------
if [ -n "${MSG_MODEL}" ]; then
  check "messages non-stream" 200 POST /v1/messages \
    "{\"model\":\"${MSG_MODEL}\",\"max_tokens\":8,\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}" '"content"'
  check "messages stream" 200 POST /v1/messages \
    "{\"model\":\"${MSG_MODEL}\",\"max_tokens\":8,\"stream\":true,\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}" 'event:'
else
  skip "messages family" "no messages model enabled"
fi

models="$(timeout 60 curl -s -H "Authorization: Bearer ${CLIENT_KEY}" "${BASE}/v1/models" |
  python3 -c 'import sys,json; print(len(json.load(sys.stdin).get("data", [])))' 2>/dev/null || echo 0)"
if [ "${models}" -ge 3 ]; then
  printf '  PASS  %-46s %s models\n' "/v1/models catalogue" "${models}"
  PASS=$((PASS + 1))
else
  printf '  FAIL  %-46s got %s\n' "/v1/models catalogue" "${models}"
  FAIL=$((FAIL + 1))
fi

if [ -n "${MGMT_KEY}" ]; then
  menu="$(timeout 60 curl -s -H "Authorization: Bearer ${MGMT_KEY}" \
    "${BASE}/v8/management/plugins" 2>/dev/null |
    python3 -c '
import sys, json
try:
    payload = json.load(sys.stdin)
except Exception:
    raise SystemExit
for plugin in payload.get("plugins", []):
    for menu in plugin.get("menus", []):
        print(menu.get("menu", ""), menu.get("path", ""))
' 2>/dev/null || true)"
  if printf '%s' "${menu}" | grep -q "OpenCode Go"; then
    printf '  PASS  %-46s %s\n' "sidebar menu entry" "${menu}"
    PASS=$((PASS + 1))
  else
    printf '  FAIL  %-46s menu not found\n' "sidebar menu entry"
    FAIL=$((FAIL + 1))
  fi
fi

state="$(timeout 60 curl -s "${BASE}/v0/resource/plugins/cap-opencode-go/api/state" 2>/dev/null || true)"
if printf '%s' "${state}" | grep -q '"models"'; then
  printf '  PASS  %-46s\n' "dashboard state resource"
  PASS=$((PASS + 1))
else
  printf '  FAIL  %-46s\n' "dashboard state resource"
  FAIL=$((FAIL + 1))
fi

merged="$(timeout 120 curl -s "${BASE}/v0/resource/plugins/cap-opencode-go/api/models" 2>/dev/null || true)"
if printf '%s' "${merged}" | grep -q '"models"'; then
  upstream_count="$(printf '%s' "${merged}" | python3 -c 'import sys,json;print((json.load(sys.stdin).get("upstream") or {}).get("count",0))' 2>/dev/null || echo 0)"
  printf '  PASS  %-46s upstream %s models\n' "live model list resource" "${upstream_count}"
  PASS=$((PASS + 1))
else
  printf '  FAIL  %-46s\n' "live model list resource"
  FAIL=$((FAIL + 1))
fi

quota="$(timeout 120 curl -s "${BASE}/v0/resource/plugins/cap-opencode-go/api/quota" 2>/dev/null || true)"
if printf '%s' "${quota}" | grep -q '"windows"'; then
  printf '  PASS  %-46s\n' "quota resource"
  PASS=$((PASS + 1))
else
  printf '  WARN  %-46s no window data returned\n' "quota resource"
fi

# The protocol probe is a management route, so it needs the management key; the
# plugin itself never sees unauthenticated probe traffic.
if [ -n "${MGMT_KEY}" ]; then
  probe_model="${RESPONSES_MODEL:-gpt-6-luna}"
  # Plugin management routes live under /v0/management; newer builds also mirror
  # the host API under /v8, so try both like the dashboard does.
  probe=""
  for prefix in v8 v0; do
    probe="$(timeout 120 curl -s -o /tmp/cpa-probe.$$ -w '%{http_code}' \
      -H "Authorization: Bearer ${MGMT_KEY}" \
      "${BASE}/${prefix}/management/opencode-go/probe?model=${probe_model}" 2>/dev/null || true)"
    [ "${probe}" = "200" ] && break
  done
  if [ "${probe}" = "200" ] && grep -q '"results"' /tmp/cpa-probe.$$ 2>/dev/null; then
    family="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1])).get("best","none"))' /tmp/cpa-probe.$$ 2>/dev/null || echo none)"
    printf '  PASS  %-46s best=%s\n' "management protocol probe" "${family:-none}"
    PASS=$((PASS + 1))
  else
    printf '  FAIL  %-46s HTTP %s\n' "management protocol probe" "${probe:-none}"
    FAIL=$((FAIL + 1))
  fi
  rm -f /tmp/cpa-probe.$$
fi

echo
echo "== ${PASS} passed, ${FAIL} failed =="
[ "${FAIL}" -eq 0 ]
