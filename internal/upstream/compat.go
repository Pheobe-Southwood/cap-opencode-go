package upstream

import (
	"bytes"
	"encoding/json"

	"github.com/Pheobe-Southwood/cpa-opencodego/internal/roles"
)

// normalizeDeveloperRoles rewrites the OpenAI "developer" message role to
// "system" for the OpenAI-shaped families.
//
// "developer" is a legal Chat Completions role (and the role newer OpenAI
// reasoning models use for the system prompt), but OpenCode Go's new inference
// router only accepts system/user/assistant/tool on some of its gateway nodes
// and answers HTTP 400 "chat message role `developer` is not supported by the
// router gRPC frontend". For every provider behind OpenCode Go the two roles
// carry the same meaning, so rewriting them leaves the model's behaviour
// unchanged while removing the intermittent 400.
//
// The rewrite is deliberately lazy: a body without the literal string is
// returned untouched, and an unmatched JSON body is never rewritten.
func normalizeDeveloperRoles(role roles.Role, body []byte, rewrite bool) []byte {
	if !rewrite {
		return body
	}
	if role.Name != roles.Chat && role.Name != roles.Responses {
		return body
	}
	if !bytes.Contains(body, []byte(`"developer"`)) {
		return body
	}
	var object map[string]json.RawMessage
	if errUnmarshal := json.Unmarshal(body, &object); errUnmarshal != nil {
		return body
	}
	changed := false
	for _, field := range []string{"messages", "input"} {
		raw, ok := object[field]
		if !ok {
			continue
		}
		rewritten, ok := rewriteRoleArray(raw)
		if !ok {
			continue
		}
		object[field] = rewritten
		changed = true
	}
	if !changed {
		return body
	}
	out, errMarshal := json.Marshal(object)
	if errMarshal != nil {
		return body
	}
	return out
}

// rewriteRoleArray rewrites every item whose role is "developer" in one
// messages/input array and reports whether anything changed. Items that are not
// objects, or that carry an untouched role, are preserved verbatim.
func rewriteRoleArray(raw json.RawMessage) (json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return raw, false
	}
	var items []json.RawMessage
	if errUnmarshal := json.Unmarshal(trimmed, &items); errUnmarshal != nil {
		return raw, false
	}
	changed := false
	for index, item := range items {
		var entry map[string]json.RawMessage
		if errUnmarshal := json.Unmarshal(item, &entry); errUnmarshal != nil {
			continue
		}
		value, ok := entry["role"]
		if !ok {
			continue
		}
		var roleName string
		if errUnmarshal := json.Unmarshal(value, &roleName); errUnmarshal != nil || roleName != "developer" {
			continue
		}
		entry["role"] = json.RawMessage(`"system"`)
		encoded, errMarshal := json.Marshal(entry)
		if errMarshal != nil {
			continue
		}
		items[index] = encoded
		changed = true
	}
	if !changed {
		return raw, false
	}
	out, errMarshal := json.Marshal(items)
	if errMarshal != nil {
		return raw, false
	}
	return out, true
}
