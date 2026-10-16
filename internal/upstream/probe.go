package upstream

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/cpa-opencodego/cap-opencode-go/internal/abi"
	"github.com/cpa-opencodego/cap-opencode-go/internal/roles"
)

// probeTimeout bounds one protocol-family probe so the dashboard never waits on
// the full request timeout while an unused endpoint hangs.
const probeTimeout = 20 * time.Second

// FamilyProbe is the outcome of trying one upstream protocol family.
type FamilyProbe struct {
	// Family is the protocol family that was tried.
	Family string `json:"family"`
	// OK reports whether the family served the model.
	OK bool `json:"ok"`
	// Status is the upstream HTTP status, when there was a response.
	Status int `json:"status,omitempty"`
	// Error carries the upstream message when the family refused the model.
	Error string `json:"error,omitempty"`
	// ElapsedMS is the wall-clock time the probe took.
	ElapsedMS int64 `json:"elapsed_ms"`
}

// ProbeFamilies asks every upstream endpoint whether it serves a model, using a
// one-token request. It exists because GET {base}/models only publishes ids:
// when OpenCode Go adds a model, the endpoint that serves it has to be found,
// and the id-prefix rule in the catalogue package is only a guess.
//
// The preferred family is tried first so the common case answers immediately.
// Probing is read-only apart from consuming a few tokens on the endpoint that
// does serve the model only for the families that answer.
func (c *Client) ProbeFamilies(ctx context.Context, model string, preferred roles.Name) []FamilyProbe {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return nil
	}
	settings := c.Settings()

	order := make([]roles.Role, 0, len(roles.All))
	for _, role := range roles.All {
		if role.Name == preferred {
			order = append(order, role)
		}
	}
	for _, role := range roles.All {
		if role.Name != preferred {
			order = append(order, role)
		}
	}

	results := make([]FamilyProbe, 0, len(order))
	for _, role := range order {
		results = append(results, c.probeFamily(ctx, role, trimmed, settings.Session))
	}
	return results
}

func (c *Client) probeFamily(ctx context.Context, role roles.Role, model, session string) FamilyProbe {
	started := time.Now()
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	result, errExecute := c.Execute(probeCtx, Call{
		Role:  role,
		Model: model,
		Body:  probeBody(role, model),
		// A probe must not disturb the key pool's failure accounting beyond
		// what a real request would do, so it reports its own outcome only.
		Session: session,
	})
	probe := FamilyProbe{Family: string(role.Name), ElapsedMS: time.Since(started).Milliseconds()}
	if errExecute == nil {
		probe.OK = true
		probe.Status = result.StatusCode
		return probe
	}
	probe.Status = abi.Status(errExecute)
	probe.Error = errExecute.Error()
	return probe
}

// probeBody builds the smallest valid request body for one protocol family.
func probeBody(role roles.Role, model string) []byte {
	var body any
	switch role.Name {
	case roles.Responses:
		body = map[string]any{
			"model":             model,
			"input":             "ping",
			"max_output_tokens": 16,
			"stream":            false,
		}
	case roles.Gemini:
		body = map[string]any{
			"contents": []any{map[string]any{
				"parts": []any{map[string]any{"text": "ping"}},
			}},
		}
	default:
		body = map[string]any{
			"model":      model,
			"max_tokens": 8,
			"messages": []any{map[string]any{
				"role":    "user",
				"content": "ping",
			}},
		}
	}
	encoded, errMarshal := json.Marshal(body)
	if errMarshal != nil {
		return []byte(`{}`)
	}
	return encoded
}
