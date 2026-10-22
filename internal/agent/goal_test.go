package agent

import (
	"context"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/goal"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newGoalTestAgent builds a session agent wired for goal mode, with a
// notification sink so the goal's outcome can be observed.
func newGoalTestAgent(t *testing.T, env fakeEnv, model fantasy.LanguageModel, notifier pubsub.Publisher[notify.Notification], settings config.TodoEnforcementSettings, ts ...fantasy.AgentTool) *sessionAgent {
	t.Helper()
	if len(ts) == 0 {
		ts = []fantasy.AgentTool{probeTool()}
	}
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel: Model{
			Model:      model,
			CatwalkCfg: catwalkModelCfg(),
		},
		SmallModel: Model{
			Model:      model,
			CatwalkCfg: catwalkModelCfg(),
		},
		SystemPrompt:    "system",
		IsYolo:          true,
		Sessions:        env.sessions,
		Messages:        env.messages,
		Tools:           ts,
		Notify:          notifier,
		TodoEnforcement: settings,
	}).(*sessionAgent)
	env.joinTitles(sa)
	return sa
}

// goalSession creates a session carrying an active goal.
func goalSession(t *testing.T, env fakeEnv, text string, budget int) session.Session {
	t.Helper()
	sess, err := env.sessions.Create(t.Context(), "goal session")
	require.NoError(t, err)
	_, err = env.sessions.SetGoal(t.Context(), sess.ID, &goal.Goal{
		Text:       text,
		State:      goal.StateActive,
		TurnBudget: budget,
	})
	require.NoError(t, err)
	return sess
}

// goalNotifications collects the notifications a run publishes.
type goalNotifications struct {
	*goalBroker
	events <-chan pubsub.Event[notify.Notification]
}

// goalBroker is a notification publisher the tests can subscribe to.
type goalBroker struct {
	*pubsub.Broker[notify.Notification]
}

func newGoalNotifications() *goalNotifications {
	broker := pubsub.NewBroker[notify.Notification]()
	return &goalNotifications{
		goalBroker: &goalBroker{Broker: broker},
		events:     broker.Subscribe(context.Background()),
	}
}

// goalMessages returns the goal notifications received so far.
func (g *goalNotifications) goalMessages() []string {
	var out []string
	for {
		select {
		case ev := <-g.events:
			if ev.Payload.Type == notify.TypeGoal {
				out = append(out, ev.Payload.Message)
			}
		default:
			return out
		}
	}
}

// TestGoalLoop_UnmetVerdictRunsAnotherTurn pins the loop's core: while
// the evaluator says the goal is unmet and the budget allows, another
// turn runs, and it carries the continuation directive.
func TestGoalLoop_UnmetVerdictRunsAnotherTurn(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	model := &scriptedModel{
		steps: []scriptedStep{
			{text: "first turn"},
			{text: "second turn"},
		},
		verdict: `{"met": false, "reason": "the suite still fails"}`,
	}
	notifications := newGoalNotifications()
	sa := newGoalTestAgent(t, env, model, notifications, config.TodoEnforcementSettings{
		Enabled:        true,
		NudgeThreshold: 2,
	})
	sess := goalSession(t, env, "make the tests green", 2)

	_, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "make the tests green",
	})
	require.NoError(t, err)

	prompts := model.promptTexts()
	require.Len(t, prompts, 2, "the unmet verdict must run a second turn")
	second := strings.Join(prompts[1], "\n")
	assert.Contains(t, second, "make the tests green", "the directive states the goal")
	assert.Contains(t, second, "the suite still fails", "the directive repeats the evaluator's reason")
}

// TestGoalLoop_MetVerdictStopsTheLoop pins the stop condition: a met
// verdict ends the loop, and the session says so.
func TestGoalLoop_MetVerdictStopsTheLoop(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	model := &scriptedModel{
		steps: []scriptedStep{
			{text: "first turn"},
			{text: "second turn"},
		},
		verdict: `{"met": true, "reason": "all tests pass"}`,
	}
	notifications := newGoalNotifications()
	sa := newGoalTestAgent(t, env, model, notifications, config.TodoEnforcementSettings{
		Enabled:        true,
		NudgeThreshold: 2,
	})
	sess := goalSession(t, env, "make the tests green", 5)

	_, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "make the tests green",
	})
	require.NoError(t, err)

	require.Len(t, model.promptTexts(), 1, "a met verdict must not run another turn")
	fetched, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.Goal)
	assert.Equal(t, goal.StateMet, fetched.Goal.State)
	assert.Equal(t, "all tests pass", fetched.Goal.Verdict)
	messages := notifications.goalMessages()
	require.Len(t, messages, 1, "the goal's outcome is reported once")
	assert.Contains(t, messages[0], "make the tests green")
}

// TestGoalLoop_BudgetExhaustedStopsTheLoop pins the budget: the goal
// stops once its turns are spent, and the session says it ran out.
func TestGoalLoop_BudgetExhaustedStopsTheLoop(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	model := &scriptedModel{
		steps: []scriptedStep{
			{text: "first turn"},
			{text: "second turn"},
			{text: "third turn"},
		},
		verdict: `{"met": false, "reason": "still not done"}`,
	}
	notifications := newGoalNotifications()
	sa := newGoalTestAgent(t, env, model, notifications, config.TodoEnforcementSettings{
		Enabled:        true,
		NudgeThreshold: 2,
	})
	sess := goalSession(t, env, "make the tests green", 2)

	_, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "make the tests green",
	})
	require.NoError(t, err)

	require.Len(t, model.promptTexts(), 2, "the budget bounds the loop")
	fetched, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.Goal)
	assert.Equal(t, goal.StateExhausted, fetched.Goal.State)
	assert.Equal(t, 2, fetched.Goal.TurnsUsed)
	messages := notifications.goalMessages()
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0], "make the tests green")
}

// TestGoalLoop_UnreadableVerdictKeepsTheLoopAlive pins the never-crash
// rule: a verdict the evaluator could not express is an unmet verdict,
// so the loop continues instead of dying.
func TestGoalLoop_UnreadableVerdictKeepsTheLoopAlive(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	model := &scriptedModel{
		steps: []scriptedStep{
			{text: "first turn"},
			{text: "second turn"},
		},
		verdict: "I could not say.",
	}
	notifications := newGoalNotifications()
	sa := newGoalTestAgent(t, env, model, notifications, config.TodoEnforcementSettings{
		Enabled:        true,
		NudgeThreshold: 2,
	})
	sess := goalSession(t, env, "make the tests green", 2)

	_, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "make the tests green",
	})
	require.NoError(t, err)

	require.Len(t, model.promptTexts(), 2, "an unreadable verdict is an unmet verdict")
	fetched, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.Goal)
	assert.NotEmpty(t, fetched.Goal.Note, "the unreadable verdict is noted")
}

// TestGoalDirective_SupersedesAndFoldsTheNudge pins the ladder's one
// injection rule: while a goal continuation is due, the directive
// supersedes the todo nudge and carries the nudge's content with it.
func TestGoalDirective_SupersedesAndFoldsTheNudge(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	model := &scriptedModel{steps: []scriptedStep{{text: "done"}}}
	notifications := newGoalNotifications()
	sa := newGoalTestAgent(t, env, model, notifications, config.TodoEnforcementSettings{
		Enabled:        true,
		NudgeThreshold: 2,
	}, probeTool(), mutatingTool("bash"))
	sess := goalSession(t, env, "make the tests green", 5)

	// A goal that owes a continuation, and a todo ladder that wants to
	// nudge in the same turn: the directive carries both.
	goalRun := sa.goal.newRun(&goal.Goal{
		Text:       "make the tests green",
		State:      goal.StateActive,
		TurnBudget: 5,
		Verdict:    "the suite still fails",
		Pending:    true,
	})
	todoRun := sa.todoEnforcement.newRun(false, nil, map[string]fantasy.AgentTool{
		"bash": mutatingTool("bash"),
	})
	todoRun.recordToolCall("bash", "{}")

	msgs := sa.injectLadderDirective(t.Context(), sess.ID, goalRun, todoRun, nil)
	require.NotEmpty(t, msgs)
	var texts []string
	for _, msg := range msgs {
		for _, part := range msg.Content {
			if text, ok := part.(fantasy.TextPart); ok {
				texts = append(texts, text.Text)
			}
		}
	}
	joined := strings.Join(texts, "\n")
	assert.Contains(t, joined, "the suite still fails", "the goal directive is injected")
	assert.Contains(t, joined, todoNudgeMessage, "the todo nudge is folded into the goal directive")
}

// TestGoalContinuationIsHidden pins that the goal's continuation
// scaffolding stays out of the transcript the user reads and out of the
// forks taken from it.
func TestGoalContinuationIsHidden(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	model := &scriptedModel{
		steps: []scriptedStep{
			{text: "first turn"},
			{text: "second turn"},
		},
		verdict: `{"met": false, "reason": "the suite still fails"}`,
	}
	notifications := newGoalNotifications()
	sa := newGoalTestAgent(t, env, model, notifications, config.TodoEnforcementSettings{
		Enabled:        true,
		NudgeThreshold: 2,
	})
	sess := goalSession(t, env, "make the tests green", 3)

	_, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "make the tests green",
	})
	require.NoError(t, err)

	messages, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var hidden int
	for _, m := range messages {
		if m.Role != message.User {
			continue
		}
		if m.Content().Hidden {
			hidden++
		}
	}
	assert.Positive(t, hidden, "the goal's continuation turns are hidden messages")
}

// TestGoalLoop_WithoutGoalDoesNothing pins that a session without a
// goal runs exactly one turn: the loop never starts on its own.
func TestGoalLoop_WithoutGoalDoesNothing(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	model := &scriptedModel{
		steps: []scriptedStep{
			{text: "first turn"},
			{text: "second turn"},
		},
		verdict: `{"met": false, "reason": "the suite still fails"}`,
	}
	notifications := newGoalNotifications()
	sa := newGoalTestAgent(t, env, model, notifications, config.TodoEnforcementSettings{
		Enabled:        true,
		NudgeThreshold: 2,
	})
	sess, err := env.sessions.Create(t.Context(), "plain session")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "do the work",
	})
	require.NoError(t, err)

	require.Len(t, model.promptTexts(), 1, "a session without a goal runs one turn")
	assert.Empty(t, notifications.goalMessages(), "a session without a goal reports nothing")
}

// TestGoalLoop_KillAfterNudgesAbandonsTheGoal pins the mapping of the
// ladder's kill threshold onto goal mode: a model that keeps ending
// its turn while the evaluator disagrees is abandoned.
func TestGoalLoop_KillAfterNudgesAbandonsTheGoal(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	model := &scriptedModel{
		steps: []scriptedStep{
			{text: "first turn"},
			{text: "second turn"},
			{text: "third turn"},
		},
		verdict: `{"met": false, "reason": "still not done"}`,
	}
	notifications := newGoalNotifications()
	sa := newGoalTestAgent(t, env, model, notifications, config.TodoEnforcementSettings{
		Enabled:            true,
		NudgeThreshold:     2,
		KillAfterNudges:    1,
		KillAfterNudgesSet: true,
	})
	sess := goalSession(t, env, "make the tests green", 25)

	_, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "make the tests green",
	})
	require.NoError(t, err)

	require.Len(t, model.promptTexts(), 2, "the kill threshold ends the loop")
	fetched, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.Goal)
	assert.Equal(t, goal.StateExhausted, fetched.Goal.State)
	assert.Equal(t, 1, fetched.Goal.Ignored)
}

// TestGoalLoop_ContinuationRunsWithTheSessionPolicy pins that the
// loop stops when the budget is spent and that the goal says so.
func TestGoalLoop_ContinuationRunsWithTheSessionPolicy(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	model := &scriptedModel{
		steps: []scriptedStep{
			{text: "first turn"},
			{text: "second turn"},
		},
		verdict: `{"met": false, "reason": "the suite still fails"}`,
	}
	notifications := newGoalNotifications()
	sa := newGoalTestAgent(t, env, model, notifications, config.TodoEnforcementSettings{
		Enabled:        true,
		NudgeThreshold: 2,
	})
	sess := goalSession(t, env, "make the tests green", 2)

	ctx := context.Background()
	_, err := sa.Run(ctx, SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "make the tests green",
	})
	require.NoError(t, err)

	fetched, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.Goal)
	assert.Equal(t, goal.StateExhausted, fetched.Goal.State, "the budget bounds the loop")
	assert.False(t, fetched.Goal.Pending, "a spent goal owes no continuation")
}
