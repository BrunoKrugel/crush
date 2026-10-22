package goal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalizeBudget pins the budget rules: the default applies when
// the user says nothing, and the hard cap wins over any larger ask.
func TestNormalizeBudget(t *testing.T) {
	t.Parallel()
	assert.Equal(t, DefaultTurnBudget, NormalizeBudget(0), "no budget means the default")
	assert.Equal(t, DefaultTurnBudget, NormalizeBudget(-3), "a negative budget means the default")
	assert.Equal(t, 5, NormalizeBudget(5), "a smaller budget is kept")
	assert.Equal(t, MaxTurnBudget, NormalizeBudget(100), "the hard cap wins")
	assert.Equal(t, 25, MaxTurnBudget, "the cap is 25 turns")
	assert.Equal(t, 25, DefaultTurnBudget, "the default budget is 25 turns")
}

// TestParseVerdictJSON pins the happy path: a JSON verdict says whether
// the goal is met and why.
func TestParseVerdictJSON(t *testing.T) {
	t.Parallel()
	v := ParseVerdict(`{"met": true, "reason": "all tests pass"}`)
	assert.True(t, v.Met)
	assert.Equal(t, "all tests pass", v.Reason)
	assert.Empty(t, v.Note, "a parseable verdict needs no note")
}

// TestParseVerdictUnmet pins the negative verdict.
func TestParseVerdictUnmet(t *testing.T) {
	t.Parallel()
	v := ParseVerdict(`{"met": false, "reason": "the build is still red"}`)
	assert.False(t, v.Met)
	assert.Equal(t, "the build is still red", v.Reason)
}

// TestParseVerdictFencedJSON pins that a fenced JSON block (what many
// models emit) is read like bare JSON.
func TestParseVerdictFencedJSON(t *testing.T) {
	t.Parallel()
	v := ParseVerdict("```json\n{\"met\": true, \"reason\": \"done\"}\n```")
	assert.True(t, v.Met)
	assert.Equal(t, "done", v.Reason)
}

// TestParseVerdictProse pins the tolerant path: a model that answers in
// prose still yields a verdict when it says whether the goal is met.
func TestParseVerdictProse(t *testing.T) {
	t.Parallel()
	v := ParseVerdict("The goal is not met yet: the tests still fail.")
	assert.False(t, v.Met, "prose that does not claim success means unmet")
	assert.NotEmpty(t, v.Note, "a prose verdict is noted")
}

// TestParseVerdictGarbage pins the never-crash rule: an unreadable
// reply is an unmet verdict with a note, never a panic.
func TestParseVerdictGarbage(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{"", "I am not sure.", "{\"met\":", "```"} {
		v := ParseVerdict(reply)
		assert.False(t, v.Met, "an unreadable reply is unmet: %q", reply)
		assert.NotEmpty(t, v.Note, "an unreadable reply is noted: %q", reply)
	}
}

// TestParseVerdictClaimsSuccess pins that prose that clearly claims
// success is read as met.
func TestParseVerdictClaimsSuccess(t *testing.T) {
	t.Parallel()
	v := ParseVerdict(`{"met": true}`)
	assert.True(t, v.Met)
}

// TestDirectiveCarriesGoalAndReason pins the continuation directive: it
// states the goal, repeats the evaluator's reason, and tells the model
// to keep going.
func TestDirectiveCarriesGoalAndReason(t *testing.T) {
	t.Parallel()
	g := &Goal{Text: "make the tests green", State: StateActive, TurnBudget: 3}
	directive := g.Directive("the suite still fails", "")
	assert.Contains(t, directive, "make the tests green")
	assert.Contains(t, directive, "the suite still fails")
	assert.Contains(t, strings.ToLower(directive), "goal")
}

// TestDirectiveFoldsTheNudge pins the ladder's one-injection rule: the
// continuation directive carries the todo nudge's content with it.
func TestDirectiveFoldsTheNudge(t *testing.T) {
	t.Parallel()
	g := &Goal{Text: "make the tests green", State: StateActive, TurnBudget: 3}
	directive := g.Directive("the suite still fails", "Call the todos tool now.")
	assert.Contains(t, directive, "Call the todos tool now.")
	assert.Contains(t, directive, "make the tests green")
}

// TestActive pins the lifecycle predicate: only an active goal runs.
func TestActive(t *testing.T) {
	t.Parallel()
	var nilGoal *Goal
	assert.False(t, nilGoal.Active(), "a session without a goal has no active goal")
	assert.True(t, (&Goal{State: StateActive}).Active())
	assert.False(t, (&Goal{State: StateMet}).Active())
	assert.False(t, (&Goal{State: StateExhausted}).Active())
	assert.False(t, (&Goal{State: StateCancelled}).Active())
}

// TestEvalPromptCarriesGoalAndAnswer pins what the evaluator sees: the
// goal and the assistant's last message, and nothing else.
func TestEvalPromptCarriesGoalAndAnswer(t *testing.T) {
	t.Parallel()
	prompt := EvalPrompt(Request{Goal: "ship the release", LastMessage: "I fixed the typo."})
	assert.Contains(t, prompt, "ship the release")
	assert.Contains(t, prompt, "I fixed the typo.")
	require.NotEmpty(t, prompt)
}
