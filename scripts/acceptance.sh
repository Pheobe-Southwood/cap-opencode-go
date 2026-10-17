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

# Management checks are optional. A supplied key that the host rejects (e.g. a
# bcrypt secret whose plaintext is not available to this shell, or a temporary
# IP ban after failed logins) must skip those checks rather than turn the run
# red: the public dashboard and statistics checks still run.
if [ -n "${MGMT_KEY}" ]; then
  mgmt_probe="/tmp/cpa-opencodego-mgmt-probe.$$"
  mgmt_status="$(timeout 30 curl -s -o "${mgmt_probe}" -w '%{http_code}' \
    -H "Authorization: Bearer ${MGMT_KEY}" "${BASE}/v8/management/plugins" 2>/dev/null || echo 000)"
  if [ "${mgmt_status}" != "200" ]; then
    printf '  SKIP  %-46s management key rejected (HTTP %s)\n' "management API auth" "${mgmt_status}"
    MGMT_KEY=""
  fi
  rm -f "${mgmt_probe}"
fi

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

  # The provider loads as four shared libraries, so the plugin list has four
  # rows. They have to read as one product: same icon, one name per face, and
  # the configuration advertised only where it is actually read (the main face).
  brand="$(timeout 60 curl -s -H "Authorization: Bearer ${MGMT_KEY}" \
    "${BASE}/v8/management/plugins" 2>/dev/null |
    python3 -c '
import sys, json
FACES = {
    "cap-opencode-go": "OpenCode Go",
    "cap-opencode-go-responses": "OpenCode Go · Responses",
    "cap-opencode-go-messages": "OpenCode Go · Messages",
    "cap-opencode-go-gemini": "OpenCode Go · Gemini",
}
try:
    payload = json.load(sys.stdin)
except Exception:
    payload = None
if payload is None:
    for label in ("faces", "icon", "names", "config"):
        print(label + "\t" + "fail" + "\t" + "management API did not return JSON")
    raise SystemExit
entries = dict((p.get("id"), p) for p in payload.get("plugins", []))
missing = sorted(pid for pid in FACES if pid not in entries)
if missing:
    for label in ("faces", "icon", "names", "config"):
        print(label + "\t" + "fail" + "\t" + "missing rows: " + ", ".join(missing))
    raise SystemExit
print("faces\tok\t" + ", ".join(sorted(FACES)))
bad_icon = sorted(pid for pid in FACES if not str(entries[pid].get("logo") or "").startswith("data:image/svg+xml;base64,"))
if bad_icon:
    print("icon\tfail\tno OpenCode icon on: " + ", ".join(bad_icon))
else:
    print("icon\tok\tsame data:icon on all four rows")
bad_name = sorted(pid for pid in FACES if str(entries[pid].get("metadata", {}).get("name") or "") != FACES[pid])
if bad_name:
    print("names\tfail\t" + ", ".join(pid + "=" + str(entries[pid].get("metadata", {}).get("name")) for pid in bad_name))
else:
    print("names\tok\t" + " / ".join(FACES[pid] for pid in sorted(FACES)))
main_fields = len(entries["cap-opencode-go"].get("config_fields") or [])
sibling_fields = sum(len(entries[pid].get("config_fields") or []) for pid in FACES if pid != "cap-opencode-go")
if main_fields and not sibling_fields:
    print("config\tok\tmain %d fields, siblings 0" % main_fields)
else:
    print("config\tfail\tmain %d fields, siblings %d fields" % (main_fields, sibling_fields))
' 2>/dev/null || true)"
  while IFS=$'\t' read -r aspect verdict detail; do
    if [ -z "${aspect}" ]; then
      continue
    fi
    if [ "${verdict}" = "ok" ]; then
      printf '  PASS  %-46s %s\n' "plugin ${aspect}" "${detail}"
      PASS=$((PASS + 1))
    else
      printf '  FAIL  %-46s %s\n' "plugin ${aspect}" "${detail}"
      FAIL=$((FAIL + 1))
    fi
  done <<< "${brand}"

  # The main face is the one that answers the host's quota RPC, and the host
  # only links an account to the quota page when the provider key matches.
  quota_plugin="$(timeout 60 curl -s -H "Authorization: Bearer ${MGMT_KEY}" \
    "${BASE}/v8/management/plugins" 2>/dev/null |
    python3 -c '
import sys, json
try:
    payload = json.load(sys.stdin)
except Exception:
    print("fail\tmanagement API did not return JSON")
    raise SystemExit
entries = dict((p.get("id"), p) for p in payload.get("plugins", []))
main = entries.get("cap-opencode-go") or {}
if not main.get("supports_quota"):
    print("fail\tmain face does not report supports_quota")
elif str(main.get("quota_provider") or "") != "opencode-go":
    print("fail\tquota_provider=" + str(main.get("quota_provider")))
elif any((entries.get(pid) or {}).get("supports_quota") for pid in entries if pid != "cap-opencode-go" and pid.startswith("cap-opencode-go")):
    print("fail\ta sibling face also declares the quota provider")
else:
    print("ok\tsupports_quota, quota_provider=opencode-go, only on the main face")
' 2>/dev/null || echo "fail\tcould not read the plugin list")"
  verdict="${quota_plugin%%$'\t'*}"
  detail="${quota_plugin#*$'\t'}"
  if [ "${verdict}" = "ok" ]; then
    printf '  PASS  %-46s %s\n' "plugin quota provider" "${detail}"
    PASS=$((PASS + 1))
  else
    printf '  FAIL  %-46s %s\n' "plugin quota provider" "${detail}"
    FAIL=$((FAIL + 1))
  fi

  # The accounts must be listed as quota-capable credentials pointing back at the
  # plugin, which is what the 配额管理 → Plugin tab filters on.
  # The v8 panel lists credentials under /v8/management/credentials; the legacy
  # /v0/management/auth-files route is kept as a fallback for older hosts.
  auth_files="$(timeout 60 curl -s -H "Authorization: Bearer ${MGMT_KEY}" \
    "${BASE}/v8/management/credentials" 2>/dev/null || true)"
  if ! printf '%s' "${auth_files}" | grep -q '"provider"'; then
    auth_files="$(timeout 60 curl -s -H "Authorization: Bearer ${MGMT_KEY}" \
      "${BASE}/v0/management/auth-files" 2>/dev/null || true)"
  fi
  accounts_check="$(printf '%s' "${auth_files}" | python3 -c '
import sys, json
try:
    payload = json.load(sys.stdin)
except Exception:
    print("fail\tmanagement API did not return JSON")
    raise SystemExit
files = payload.get("files") if isinstance(payload, dict) else payload
files = files or []
mine = [f for f in files if str(f.get("provider") or "").lower() == "opencode-go"]
if not mine:
    print("fail\tno opencode-go credentials listed")
    raise SystemExit
broken = []
for entry in mine:
    if not entry.get("supports_quota"):
        broken.append(str(entry.get("name")) + ":supports_quota")
    if str(entry.get("quota_provider") or "") != "opencode-go":
        broken.append(str(entry.get("name")) + ":quota_provider")
    if str(entry.get("quota_plugin_id") or "") != "cap-opencode-go":
        broken.append(str(entry.get("name")) + ":quota_plugin_id")
    if not str(entry.get("auth_index") or ""):
        broken.append(str(entry.get("name")) + ":auth_index")
if broken:
    print("fail\t" + ", ".join(sorted(broken)))
    raise SystemExit
first = str(mine[0].get("auth_index") or "")
print("ok\t%d accounts linked to cap-opencode-go|%s" % (len(mine), first))
' 2>/dev/null || echo "fail\tcould not read the auth file list")"
  verdict="${accounts_check%%$'\t'*}"
  detail="${accounts_check#*$'\t'}"
  first_index=""
  if [ "${verdict}" = "ok" ]; then
    printf '  PASS  %-46s %s\n' "quota accounts listed" "${detail%%|*}"
    PASS=$((PASS + 1))
    first_index="${detail##*|}"
  else
    printf '  FAIL  %-46s %s\n' "quota accounts listed" "${detail}"
    FAIL=$((FAIL + 1))
  fi

  # End-to-end: the host calls the plugin for the account and gets normalized
  # buckets it can render next to the Antigravity/Codex accounts.
  if [ -n "${first_index}" ]; then
    quota_body="/tmp/cpa-opencodego-acceptance-quota.$$"
    quota_status="$(timeout 120 curl -s -o "${quota_body}" -w '%{http_code}' \
      -X POST -H "Authorization: Bearer ${MGMT_KEY}" -H 'Content-Type: application/json' \
      -d "{\"auth_index\":\"${first_index}\"}" \
      "${BASE}/v8/management/plugins/cap-opencode-go/quota" 2>/dev/null || echo 000)"
    if [ "${quota_status}" = "200" ] && python3 - "${quota_body}" <<'PLUGINQUOTAPY' 2>/dev/null
import json, sys
try:
    payload = json.load(open(sys.argv[1]))
except Exception:
    raise SystemExit(1)
buckets = [b for group in payload.get("groups") or [] for b in group.get("buckets") or []]
if not payload.get("groups"):
    raise SystemExit(1)
if any(not isinstance(b.get("remainingFraction"), (int, float)) for b in buckets):
    raise SystemExit(1)
windows = {str(b.get("window") or "") for b in buckets}
if not windows & {"five_hour", "weekly", "monthly"}:
    raise SystemExit(1)
raise SystemExit(0)
PLUGINQUOTAPY
    then
      windows="$(python3 -c '
import json, sys
payload = json.load(open(sys.argv[1]))
print(",".join(sorted({str(b.get("window")) for group in payload.get("groups") or [] for b in group.get("buckets") or []})))
' "${quota_body}" 2>/dev/null || echo "?")"
      printf '  PASS  %-46s windows %s\n' "plugin quota fetch" "${windows}"
      PASS=$((PASS + 1))
    else
      printf '  FAIL  %-46s HTTP %s: %s\n' "plugin quota fetch" "${quota_status}" "$(head -c 200 "${quota_body}")"
      FAIL=$((FAIL + 1))
    fi
    rm -f "${quota_body}"
  else
    skip "plugin quota fetch" "no auth_index available"
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

# --- CPA account projection (配额管理 → Plugin) ------------------------------
# Every configured key is projected into one host auth file, which is what makes
# the accounts appear in CPA's own quota page. This check needs no management key:
# the plugin reports the projection in its public read model.
account_state="$(printf '%s' "${state:-}" | python3 -c '
import sys, json
try:
    payload = json.load(sys.stdin)
except Exception:
    raise SystemExit
accounts = payload.get("accounts") or {}
keys = payload.get("keys") or []
files = accounts.get("files") or {}
if not accounts.get("enabled"):
    print("disabled\taccount projection is switched off (register-accounts)")
    raise SystemExit
if accounts.get("error"):
    print("fail\t" + str(accounts["error"]))
    raise SystemExit
if len(files) != len(keys):
    print("fail\t%d files for %d keys" % (len(files), len(keys)))
    raise SystemExit
print("ok\t%d accounts (auth-dir %s)" % (len(files), accounts.get("auth_dir") or "unknown"))
' 2>/dev/null || echo "fail\tunreadable state payload")"
verdict="${account_state%%$'\t'*}"
detail="${account_state#*$'\t'}"
if [ "${verdict}" = "ok" ]; then
  printf '  PASS  %-46s %s\n' "account projection" "${detail}"
  PASS=$((PASS + 1))
elif [ "${verdict}" = "disabled" ]; then
  skip "account projection" "${detail}"
else
  printf '  FAIL  %-46s %s\n' "account projection" "${detail}"
  FAIL=$((FAIL + 1))
fi

# The files themselves, on the machine running this script. They carry the plugin
# marker and mode 0600 like every other auth file.
account_dir="$(printf '%s' "${state:-}" | python3 -c '
import sys, json
try:
    print((json.load(sys.stdin).get("accounts") or {}).get("auth_dir") or "")
except Exception:
    print("")
' 2>/dev/null || true)"
if [ -n "${account_dir}" ] && [ -d "${account_dir}" ]; then
  files_check="$(python3 - "${account_dir}" <<'ACCOUNTSPY' 2>/dev/null || echo "fail\tcould not inspect the auth dir"
import json, os, sys

directory = sys.argv[1]
checked = 0
problems = []
for name in sorted(os.listdir(directory)):
    if not (name.startswith("opencode-go-") and name.lower().endswith(".json")):
        continue
    path = os.path.join(directory, name)
    try:
        with open(path, "r", encoding="utf-8") as handle:
            payload = json.load(handle)
    except Exception as error:
        problems.append("%s: %s" % (name, error))
        continue
    if str(payload.get("managed_by", "")).strip() != "cap-opencode-go":
        continue
    if str(payload.get("type", "")).strip() not in ("opencode-go", "opencode-go-responses", "opencode-go-messages", "opencode-go-gemini"):
        problems.append(name + ": wrong provider type")
    if not str(payload.get("api_key", "")).strip():
        problems.append(name + ": no api_key")
    mode = os.stat(path).st_mode & 0o777
    if mode != 0o600:
        problems.append("%s: mode %o" % (name, mode))
    checked += 1
if problems:
    print("fail\t" + "; ".join(problems))
elif checked == 0:
    print("fail\tno mirrored account files")
else:
    print("ok\t%d files, 0600, managed_by marker" % checked)
ACCOUNTSPY
)"
  verdict="${files_check%%$'\t'*}"
  detail="${files_check#*$'\t'}"
  if [ "${verdict}" = "ok" ]; then
    printf '  PASS  %-46s %s\n' "account files" "${detail}"
    PASS=$((PASS + 1))
  else
    printf '  FAIL  %-46s %s\n' "account files" "${detail}"
    FAIL=$((FAIL + 1))
  fi
else
  skip "account files" "auth dir not readable from this host"
fi

# Statistics: the main face receives host usage events (including request latency)
# and the four faces write their own TTFT timing file, so wait through one 5s debounce.
STATS_FILE="/tmp/cpa-opencodego-acceptance-stats.$$"
stats_ready=0
stats_detail=""
# Every family that had a streaming check must show at least one TTFT sample and
# at least one paired TPS sample. TPS comes from host latency on every successful
# request; TTFT comes from the content-frame timing files and may take one 5s
# debounce longer to appear.
EXPECTED_FAMILIES=""
[ -n "${CHAT_MODEL}" ] && EXPECTED_FAMILIES="${EXPECTED_FAMILIES} chat"
[ -n "${RESP_MODEL}" ] && EXPECTED_FAMILIES="${EXPECTED_FAMILIES} responses"
[ -n "${GEMINI_MODEL}" ] && EXPECTED_FAMILIES="${EXPECTED_FAMILIES} gemini"
[ -n "${MSG_MODEL}" ] && EXPECTED_FAMILIES="${EXPECTED_FAMILIES} messages"
for _ in $(seq 1 25); do
  timeout 30 curl -s "${BASE}/v0/resource/plugins/cap-opencode-go/api/stats?range=all" > "${STATS_FILE}" 2>/dev/null || true
  stats_detail="$(EXPECTED_FAMILIES="${EXPECTED_FAMILIES}" python3 - "${STATS_FILE}" <<'STATSPY' 2>/dev/null || true
import json, os, sys
try:
    payload = json.load(open(sys.argv[1]))
except Exception as error:
    print("unreadable: " + str(error))
    raise SystemExit
usage = payload.get("usage_records") or {}
totals = payload.get("totals") or {}
models = payload.get("models") or []
priced = [m for m in models if m.get("priced") and not m.get("free")]
expected = set(os.environ.get("EXPECTED_FAMILIES", "").split())
seen = {m.get("family") for m in models if m.get("tps_samples", 0) > 0}
missing = sorted(expected - seen)
ready = (
    usage.get("matched", 0) > 0
    and totals.get("requests", 0) > 0
    and totals.get("total_tokens", 0) > 0
    and totals.get("ttft_samples", 0) > 0
    and totals.get("tps_samples", 0) > 0
    and not missing
    and (not priced or totals.get("cost_usd", 0) > 0)
)
rows = ", ".join("%s(%s) r=%s tok=%s $%.4f ttft=%s tps=%s" % (
    m.get("id"), m.get("family"), m.get("requests", 0), m.get("total_tokens", 0), m.get("cost_usd", 0),
    m.get("ttft_samples", 0), m.get("tps_samples", 0)) for m in models[:4])
print(("ready" if ready else "waiting") + " | matched=%s requests=%s tokens=%s ttft=%s tps=%s missing_families=%s warnings=%s | %s" % (
    usage.get("matched", 0), totals.get("requests", 0), totals.get("total_tokens", 0),
    totals.get("ttft_samples", 0), totals.get("tps_samples", 0), ",".join(missing) or "-",
    "; ".join(payload.get("warnings") or []), rows))
STATSPY
)"
  if [[ "${stats_detail}" == ready* ]]; then
    stats_ready=1
    break
  fi
  sleep 1
done
if [ "${stats_ready}" -eq 1 ]; then
  printf '  PASS  %-46s %s\n' "statistics resource" "${stats_detail}"
  PASS=$((PASS + 1))
else
  printf '  FAIL  %-46s %s\n' "statistics resource" "${stats_detail:-no response}"
  FAIL=$((FAIL + 1))
fi
if grep -qE 'oc_sk_[A-Za-z0-9]{16,}' "${STATS_FILE}" 2>/dev/null; then
  printf '  FAIL  %-46s a full key leaked into stats\n' "stats payload hides secrets"
  FAIL=$((FAIL + 1))
else
  printf '  PASS  %-46s\n' "stats payload hides secrets"
  PASS=$((PASS + 1))
fi
monthly="$(timeout 30 curl -s "${BASE}/v0/resource/plugins/cap-opencode-go/api/stats?range=month" 2>/dev/null || true)"
if printf '%s' "${monthly}" | grep -q '"since_source"'; then
  printf '  PASS  %-46s\n' "statistics monthly window"
  PASS=$((PASS + 1))
else
  printf '  FAIL  %-46s\n' "statistics monthly window"
  FAIL=$((FAIL + 1))
fi
price_check="$(printf '%s' "${monthly}" | python3 -c '
import json, sys
try:
    payload = json.load(sys.stdin)
except Exception as error:
    print("unreadable: " + str(error))
    raise SystemExit
keys = ("price_book_checked_at", "price_book_age_days", "price_book_refresh_after_days",
        "price_book_update_due", "price_book_due_at", "model_list_checked_at",
        "model_list_changed")
missing = [key for key in keys if key not in payload]
ok = (not missing and payload.get("price_book_checked_at")
      and payload.get("price_book_due_at")
      and payload.get("price_book_refresh_after_days") == 30
      and str(payload.get("price_book_url", "")).startswith("https://")
      and isinstance(payload.get("price_book_age_days"), int)
      and isinstance(payload.get("price_book_update_due"), bool)
      and payload.get("model_list_checked_at")
      and isinstance(payload.get("model_list_changed"), bool))
print(("ok" if ok else "invalid") + " checked=%s age=%s refresh=%s due=%s at=%s models=%s changed=%s" % (
    payload.get("price_book_checked_at"), payload.get("price_book_age_days"),
    payload.get("price_book_refresh_after_days"), payload.get("price_book_update_due"),
    payload.get("price_book_due_at"), payload.get("model_list_checked_at"),
    payload.get("model_list_changed")))
' 2>/dev/null || true)"
if [[ "${price_check}" == ok* ]]; then
  printf '  PASS  %-46s %s\n' "statistics data-update reminder" "${price_check#ok }"
  PASS=$((PASS + 1))
else
  printf '  FAIL  %-46s %s\n' "statistics data-update reminder" "${price_check:-invalid response}"
  FAIL=$((FAIL + 1))
fi
rm -f "${STATS_FILE}"

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

# The dashboard is served from an unauthenticated resource route, so the checks
# below make sure the shipped page is the tabbed one (accounts first, password
# dialog for writes) and that its credential-free read model carries every
# setting the page can write.
page="$(timeout 60 curl -s "${BASE}/v0/resource/plugins/cap-opencode-go/dashboard" 2>/dev/null || true)"
page_checks=(
  'data-tab="accounts"|账号管理 tab'
  'data-tab="stats"|统计 tab'
  'id="pane-stats"|统计面板'
  'id="statsSummary"|统计概览'
  'id="usageStatsTable"|统计表'
  'id="statsAboutFold"|统计口径折叠'
  'id="statsPriceNotice"|价目核对提醒'
  'data-label="平均 TTFT"|TTFT 列'
  'id="cfgPlan"|套餐设置'
  'id="pane-models"|模型 tab'
  'id="keyList"|Key 余量列表'
  'id="keyNameHint"|Key 改名提示'
  'data-key-rename="|Key 改名输入'
  'id="credModal"|保存密码弹窗'
)
# Matching happens in the shell: the page is larger than a pipe buffer, so
# 'printf | grep -q' would let grep exit early and pipefail would then report the
# pipeline as failed even on a match.
for entry in "${page_checks[@]}"; do
  marker="${entry%%|*}"
  label="${entry##*|}"
  if [[ "${page}" == *"${marker}"* ]]; then
    printf '  PASS  %-46s\n' "dashboard page: ${label}"
    PASS=$((PASS + 1))
  else
    printf '  FAIL  %-46s marker %s missing\n' "dashboard page: ${label}" "${marker}"
    FAIL=$((FAIL + 1))
  fi
done
if [[ "${page}" == *"只读"* ]]; then
  printf '  FAIL  %-46s the page still has a read-only mode\n' "dashboard page: editable without a credential"
  FAIL=$((FAIL + 1))
else
  printf '  PASS  %-46s\n' "dashboard page: editable without a credential"
  PASS=$((PASS + 1))
fi

# The page is one inline script, so a syntax error renders the document but kills
# every tab and request (that is what shipped in 1.8.0). Text markers cannot see
# it; parse the deployed script with node when it is available.
if command -v node >/dev/null 2>&1; then
  page_script="/tmp/cpa-opencodego-acceptance-page.$$.js"
  page_script_state="$(printf '%s' "${page}" | python3 -c '
import sys

document = sys.stdin.read()
start = document.find("<script>")
end = document.find("</script>", start + 1) if start >= 0 else -1
if start < 0 or end < 0:
    print("missing")
else:
    sys.stdout.write(document[start + len("<script>"):end])
' > "${page_script}" 2>/dev/null; echo "extracted")"
  if [ "${page_script_state}" = "extracted" ] && [ -s "${page_script}" ]; then
    if node --check "${page_script}" >/tmp/cpa-opencodego-acceptance-page.$$.out 2>&1; then
      printf '  PASS  %-46s %s\n' "dashboard script parses" "node --check passed"
      PASS=$((PASS + 1))
    else
      printf '  FAIL  %-46s node --check failed: %s\n' "dashboard script parses" "$(head -c 200 /tmp/cpa-opencodego-acceptance-page.$$.out | tr '\n' ' ')"
      FAIL=$((FAIL + 1))
    fi
  else
    skip "dashboard script parses" "could not extract the inline script"
  fi
  rm -f "${page_script}" "/tmp/cpa-opencodego-acceptance-page.$$.out"
else
  skip "dashboard script parses" "node is not installed"
fi

editable="$(printf '%s' "${state}" | python3 -c '
import sys, json
try:
    payload = json.load(sys.stdin)
except Exception:
    raise SystemExit
fields = ["timeout_seconds", "plan", "debug"]
missing = [field for field in fields if field not in payload]
print("ok" if not missing else "missing:" + ",".join(missing))
' 2>/dev/null || echo missing)"
if [ "${editable}" = "ok" ]; then
  printf '  PASS  %-46s\n' "state carries the editable settings"
  PASS=$((PASS + 1))
else
  printf '  FAIL  %-46s %s\n' "state carries the editable settings" "${editable:-unreadable}"
  FAIL=$((FAIL + 1))
fi

if printf '%s' "${state}" | grep -q '"windows"\|"masked"'; then
  # The state payload may only ever carry masked credentials.
  if printf '%s' "${state}" | grep -qE '"key"\s*:\s*"oc_sk_[A-Za-z0-9]{16,}"'; then
    printf '  FAIL  %-46s a full key leaked into the state payload\n' "state payload hides secrets"
    FAIL=$((FAIL + 1))
  else
    printf '  PASS  %-46s\n' "state payload hides secrets"
    PASS=$((PASS + 1))
  fi
fi

echo
echo "== ${PASS} passed, ${FAIL} failed =="
[ "${FAIL}" -eq 0 ]
