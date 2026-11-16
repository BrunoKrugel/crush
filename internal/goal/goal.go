// Package goal implements goal mode: a turn-budgeted loop in which a
// cheap evaluator judges whether a stated goal is met, and the loop keeps
// running while it is not.
//
// The state lives on the session (the same way the todo list does), so
// the ladder can read it without asking anybody, and the mode survives a
// restart. The package holds the domain rules only — the budget, the
// lifecycle, the directive text, and the reading of an evaluator's
// reply — so they can be tested without a model.
package goal

import (
	"fmt"
	"strings"
)

// State is the lifecycle state of a goal.
type State string

const (
	// StateActive means the goal is still running: the loop judges it
	// after every turn and continues while it is unmet.
	StateActive State = "active"
	// StateMet means the evaluator judged the goal achieved.
	StateMet State = "met"
	// StateExhausted means the goal ran out of turns, or was abandoned
	// because the model kept claiming success while the evaluator
	// disagreed.
	StateExhausted State = "exhausted"
	// StateCancelled means the user stopped the goal.
	StateCancelled State = "cancelled"
)

const (
	// DefaultTurnBudget is how many turns a goal may consume when the
	// user does not say.
	DefaultTurnBudget = 25
	// MaxTurnBudget is the hard cap: no goal runs more turns than this,
	// whatever the user asks for.
	MaxTurnBudget = 25
)

// Goal is the goal-mode state a session carries.
type Goal struct {
	// Text is the goal the evaluator judges.
	Text string `json:"text"`
	// State is the goal's lifecycle state.
	State State `json:"state"`
	// TurnBudget is the number of turns the goal may consume.
	TurnBudget int `json:"turn_budget"`
	// TurnsUsed counts the turns the goal has already consumed.
	TurnsUsed int `json:"turns_used"`
	// EvaluatorModel names the model that judges the goal. It is a hint:
	// an empty value means the agent's own small model.
	EvaluatorModel string `json:"evaluator_model,omitempty"`
	// Verdict is the evaluator's last reason.
	Verdict string `json:"verdict,omitempty"`
	// Note explains an unreadable evaluator reply.
	Note string `json:"note,omitempty"`
	// Ignored counts the turns that ended while the evaluator said the
	// goal was unmet — the goal mode's own kill counter.
	Ignored int `json:"ignored,omitempty"`
	// Pending marks a continuation the next turn owes: the evaluator
	// said the goal is unmet and the budget allows another turn.
	Pending bool `json:"pending,omitempty"`
}

// NormalizeBudget clamps a requested turn budget into the range the
// goal mode allows: the default when nothing was asked for, and the
// hard cap when more was.
func NormalizeBudget(budget int) int {
	switch {
	case budget <= 0:
		return DefaultTurnBudget
	case budget > MaxTurnBudget:
		return MaxTurnBudget
	default:
		return budget
	}
}

// Active reports whether the goal is still running. A nil goal (a
// session without one) is never active.
func (g *Goal) Active() bool {
	return g != nil && g.State == StateActive
}

// BudgetLeft reports how many turns the goal may still consume.
func (g *Goal) BudgetLeft() int {
	if g == nil {
		return 0
	}
	left := g.TurnBudget - g.TurnsUsed
	if left < 0 {
		return 0
	}
	return left
}

// Directive renders the continuation directive injected into the next
// turn: the goal, the evaluator's reason for calling the last turn
// short, and — when the todo ladder wants to nudge in the same turn —
// the nudge's text folded in, because the goal continuation supersedes
// the nudge and there is only one injection per turn.
func (g *Goal) Directive(reason, nudge string) string {
	if g == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Goal mode is active. The goal is:\n")
	sb.WriteString(g.Text)
	sb.WriteString("\n\n")
	sb.WriteString("Your previous turn ended without meeting the goal")
	if reason != "" {
		sb.WriteString(": ")
		sb.WriteString(reason)
	}
	sb.WriteString(". Keep working toward the goal: the goal is judged after every turn, and the turn only ends when there is nothing left to do.")
	if g.BudgetLeft() > 0 {
		fmt.Fprintf(&sb, " %d turn(s) left in the budget.", g.BudgetLeft())
	}
	if nudge != "" {
		sb.WriteString(" ")
		sb.WriteString(nudge)
	}
	return sb.String()
}
