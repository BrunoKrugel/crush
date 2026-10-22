package goal

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
)

// Verdict is the evaluator's judgment about a goal.
type Verdict struct {
	// Met reports whether the goal is achieved.
	Met bool
	// Reason is the evaluator's explanation.
	Reason string
	// Note explains an unreadable reply, so a broken verdict is visible
	// without being fatal.
	Note string
}

// Request is one evaluation request: the goal, and the assistant's last
// message to judge it against.
type Request struct {
	// Goal is the goal being judged.
	Goal string
	// LastMessage is the assistant's final message of the turn.
	LastMessage string
}

// Evaluator judges whether a goal is met. The implementation calls a
// small model; the interface keeps the goal loop testable without one.
type Evaluator interface {
	Evaluate(ctx context.Context, req Request) (Verdict, error)
}

// EvaluatorPrompt asks for a verdict and nothing else: the
// evaluator's reply is read by ParseVerdict, so a chatty answer only
// costs tokens.
const EvaluatorPrompt = `You are a goal evaluator. You judge whether a stated goal has been met.

Answer with a single JSON object and nothing else:
{"met": true|false, "reason": "<one short sentence>"}

Answer "met": true only when the goal is fully achieved and verified. Answer "met": false when work remains.`

// EvalPrompt renders the evaluator's prompt: the goal, and the message
// that ends the turn being judged.
func EvalPrompt(req Request) string {
	var sb strings.Builder
	sb.WriteString("Goal:\n")
	sb.WriteString(req.Goal)
	sb.WriteString("\n\nThe assistant's final message of this turn:\n")
	sb.WriteString(req.LastMessage)
	sb.WriteString("\n\nIs the goal met? Answer with the JSON object only.")
	return sb.String()
}

// verdictJSON is the shape ParseVerdict reads out of a reply.
type verdictJSON struct {
	Met    *bool  `json:"met"`
	Reason string `json:"reason"`
}

// metPattern finds a met flag in a reply that is not plain JSON, so a
// model that answers in prose still yields a verdict.
var metPattern = regexp.MustCompile(`(?i)"?\bmet\b"?\s*[:=]\s*(true|false)`)

// claimsMet finds a plain-language claim of success in a reply that is
// not JSON at all.
var claimsMet = regexp.MustCompile(`(?i)\b(goal (is )?met|success|achieved|complete[ds]?|done)\b`)

// ParseVerdict reads an evaluator's reply.
//
// The happy path is a JSON object with a "met" flag and a "reason". A
// reply that is not JSON is read leniently: a "met": true anywhere in
// it means the goal is met. A reply that says nothing usable is an
// unmet verdict with a note — the loop keeps going rather than crashing,
// because a broken verdict must never take the run down.
func ParseVerdict(reply string) Verdict {
	trimmed := strings.TrimSpace(reply)
	if trimmed == "" {
		return Verdict{Note: "the evaluator returned nothing"}
	}
	if v, ok := parseVerdictJSON(trimmed); ok {
		return v
	}
	// A fenced block, or a reply with prose around the JSON.
	if fenced := fencedBody(trimmed); fenced != "" {
		if v, ok := parseVerdictJSON(fenced); ok {
			return v
		}
	}
	if m := metPattern.FindStringSubmatch(trimmed); m != nil {
		met := strings.EqualFold(m[1], "true")
		return Verdict{
			Met:    met,
			Reason: trimmed,
			Note:   "the evaluator's reply was not JSON",
		}
	}
	if claimsMet.MatchString(trimmed) {
		return Verdict{
			Met:    true,
			Reason: trimmed,
			Note:   "the evaluator's reply was not JSON",
		}
	}
	return Verdict{
		Note: "could not read the evaluator's reply: " + truncate(trimmed, 200),
	}
}

// parseVerdictJSON reads a JSON verdict, reporting whether it could.
func parseVerdictJSON(s string) (Verdict, bool) {
	var parsed verdictJSON
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return Verdict{}, false
	}
	if parsed.Met == nil {
		return Verdict{}, false
	}
	return Verdict{Met: *parsed.Met, Reason: strings.TrimSpace(parsed.Reason)}, true
}

// fencedBody returns the body of a fenced code block, or an empty string
// when the text is not fenced.
func fencedBody(s string) string {
	if !strings.HasPrefix(s, "```") {
		return ""
	}
	rest := strings.TrimPrefix(s, "```")
	if i := strings.IndexByte(rest, '\n'); i >= 0 {
		rest = rest[i+1:]
	} else {
		rest = ""
	}
	if i := strings.Index(rest, "```"); i >= 0 {
		rest = rest[:i]
	}
	return strings.TrimSpace(rest)
}

// truncate shortens a string to at most n characters.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
