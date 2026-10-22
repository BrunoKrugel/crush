package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/goal"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
)

// goalContinuationPrompt is the prompt the goal loop's continuation
// turn carries. The directive the model has to follow is injected by
// the ladder — the same channel the todo nudge uses — so this prompt
// only has to carry the turn itself.
const goalContinuationPrompt = "Keep working toward the active goal."

// goalEvaluatorMaxTokens bounds the evaluator's answer: a verdict is a
// JSON object, and a model that rambles past this is not cheaper.
const goalEvaluatorMaxTokens = 200

// goalRunner drives goal mode for a session agent: it judges the goal
// after every turn, decides whether another turn is due, and reports the
// goal's outcome.
//
// The goal itself lives on the session, the same way the todo list
// does, so the ladder reads it without asking anybody and the mode
// survives a restart.
type goalRunner struct {
	sessions  session.Service
	notify    pubsub.Publisher[notify.Notification]
	evaluator goal.Evaluator
	settings  config.TodoEnforcementSettings
}

// newGoalRunner returns the goal runner for an agent. A nil runner is
// the no-op fast path: every method tolerates it.
func newGoalRunner(
	sessions session.Service,
	notifier pubsub.Publisher[notify.Notification],
	evaluator goal.Evaluator,
	settings config.TodoEnforcementSettings,
) *goalRunner {
	if sessions == nil || evaluator == nil {
		return nil
	}
	return &goalRunner{
		sessions:  sessions,
		notify:    notifier,
		evaluator: evaluator,
		settings:  settings,
	}
}

// goalRun is the goal ladder's state for one Run: the goal the session
// carried when the run started, and whether the continuation directive
// has already been injected into this turn.
type goalRun struct {
	runner   *goalRunner
	goal     *goal.Goal
	injected bool
}

// newRun starts the goal ladder for one run, seeded with the session's
// goal.
func (r *goalRunner) newRun(g *goal.Goal) *goalRun {
	if r == nil {
		return nil
	}
	return &goalRun{runner: r, goal: g}
}

// claim returns the goal the turn owes a continuation for, and consumes
// that continuation: the turn after this one does not owe it again.
// The consumed state is written back, so a session that was interrupted
// does not inject a stale directive later.
func (r *goalRun) claim(ctx context.Context, sessionID string) (*goal.Goal, bool) {
	if r == nil || r.injected {
		return nil, false
	}
	g := r.goal
	if !g.Active() || !g.Pending {
		return nil, false
	}
	r.injected = true
	g.Pending = false
	if _, err := r.runner.sessions.SetGoal(ctx, sessionID, g); err != nil {
		slog.Warn("Failed to consume the goal's continuation", "session_id", sessionID, "error", err)
	}
	return g, true
}

// carriedContinuation reports whether this turn carried the goal's
// continuation directive. It is what the goal's own kill counter
// counts: a continuation the model ignored, not a turn that ended
// short.
func (r *goalRun) carriedContinuation() bool {
	return r != nil && r.injected
}

// afterTurn judges the goal after a turn ended and decides what the
// loop does next. It returns the goal's new state and whether a
// continuation turn is due.
//
// The verdict decides everything: a met verdict ends the goal, an unmet
// verdict spends a turn of the budget and asks for another one, and the
// budget (or the ladder's kill threshold) ends a goal that keeps
// failing.
func (r *goalRunner) afterTurn(ctx context.Context, sessionID, lastMessage string, carriedContinuation bool) (*goal.Goal, bool) {
	if r == nil {
		return nil, false
	}
	sess, err := r.sessions.Get(ctx, sessionID)
	if err != nil {
		slog.Warn("Failed to read the session's goal", "session_id", sessionID, "error", err)
		return nil, false
	}
	g := sess.Goal
	if !g.Active() {
		return nil, false
	}
	verdict := r.judge(ctx, g, lastMessage)
	g.TurnsUsed++
	g.Verdict = verdict.Reason
	g.Note = verdict.Note
	if verdict.Met {
		g.State = goal.StateMet
		g.Pending = false
		r.save(ctx, sessionID, g)
		r.report(sessionID, fmt.Sprintf("Goal met: %s", g.Text))
		return g, false
	}
	// A turn that carried a continuation and still did not meet the
	// goal ignored that continuation: this is the goal mode's own kill
	// counter, the same shape as the ladder's ignored nudges.
	if carriedContinuation {
		g.Ignored++
	}
	if g.TurnsUsed >= g.TurnBudget {
		g.State = goal.StateExhausted
		g.Pending = false
		r.save(ctx, sessionID, g)
		r.report(sessionID, fmt.Sprintf("Goal not met after %d turn(s), budget spent: %s", g.TurnsUsed, g.Text))
		return g, false
	}
	if threshold := r.abandonThreshold(g); threshold > 0 && carriedContinuation && g.Ignored >= threshold {
		g.State = goal.StateExhausted
		g.Pending = false
		r.save(ctx, sessionID, g)
		r.report(sessionID, fmt.Sprintf("Goal abandoned after %d ignored continuation(s): %s", g.Ignored, g.Text))
		return g, false
	}
	g.Pending = true
	r.save(ctx, sessionID, g)
	return g, true
}

// judge asks the evaluator whether the goal is met. A broken evaluator
// is an unmet verdict with a note: the loop keeps going rather than
// dying, because a failed check must never take the run down.
func (r *goalRunner) judge(ctx context.Context, g *goal.Goal, lastMessage string) goal.Verdict {
	if r.evaluator == nil {
		return goal.Verdict{Note: "no evaluator is available"}
	}
	verdict, err := r.evaluator.Evaluate(ctx, goal.Request{
		Goal:        g.Text,
		LastMessage: lastMessage,
	})
	if err != nil {
		return goal.Verdict{Note: "the evaluator failed: " + err.Error()}
	}
	return verdict
}

// abandonThreshold is the number of ignored continuations that ends a
// goal early. An explicitly configured kill_after_nudges gives up
// sooner — the ladder's kill rung, mapped onto goal mode — while the
// goal's own turn budget is the bound otherwise.
func (r *goalRunner) abandonThreshold(g *goal.Goal) int {
	if r.settings.KillAfterNudgesSet && r.settings.KillAfterNudges > 0 {
		return r.settings.KillAfterNudges
	}
	return g.TurnBudget
}

// save writes the goal's state back to the session.
func (r *goalRunner) save(ctx context.Context, sessionID string, g *goal.Goal) {
	if _, err := r.sessions.SetGoal(ctx, sessionID, g); err != nil {
		slog.Warn("Failed to save the session's goal", "session_id", sessionID, "error", err)
	}
}

// report publishes the goal's outcome, so the user learns that the goal
// was met, ran out of budget, or was abandoned.
func (r *goalRunner) report(sessionID, message string) {
	if r.notify == nil {
		return
	}
	r.notify.Publish(pubsub.CreatedEvent, notify.Notification{
		SessionID: sessionID,
		Type:      notify.TypeGoal,
		Message:   message,
	})
}

// cancelGoal stops an active goal: the user asked for it, so the goal
// does not stay active and keep asking for turns.
func (r *goalRunner) cancelGoal(ctx context.Context, sessionID string) {
	if r == nil {
		return
	}
	sess, err := r.sessions.Get(ctx, sessionID)
	if err != nil || !sess.Goal.Active() {
		return
	}
	g := sess.Goal
	g.State = goal.StateCancelled
	g.Pending = false
	r.save(ctx, sessionID, g)
	r.report(sessionID, fmt.Sprintf("Goal cancelled: %s", g.Text))
}

// goalAfterTurn judges the goal after a turn ended and, while the goal
// is unmet and the budget allows, queues the continuation turn.
//
// The continuation is queued, not run: the run's own queue handoff
// picks it up and runs it as the next turn, the same way a queued
// prompt or a steer re-enters the agent. There is no second loop.
func (a *sessionAgent) goalAfterTurn(ctx context.Context, call SessionAgentCall, assistant *message.Message, run *goalRun) {
	if a.goal == nil || assistant == nil {
		return
	}
	if assistant.FinishReason() != message.FinishReasonEndTurn {
		// The turn did not end on the model's own terms — it was
		// canceled or cut short — so the model never claimed the goal
		// was done, and the goal is not judged against it.
		return
	}
	g, cont := a.goal.afterTurn(ctx, call.SessionID, assistant.Content().Text, run.carriedContinuation())
	if !cont {
		return
	}
	slog.Info("Goal mode continues",
		"session_id", call.SessionID,
		"turns_used", g.TurnsUsed,
		"turn_budget", g.TurnBudget,
	)
	mu := a.sessionMu(call.SessionID)
	mu.Lock()
	a.enqueueCall(SessionAgentCall{
		SessionID:         call.SessionID,
		Prompt:            goalContinuationPrompt,
		HiddenUserMessage: true,
		PermissionPolicy:  call.PermissionPolicy,
		ContentWidth:      call.ContentWidth,
		MaxOutputTokens:   call.MaxOutputTokens,
		ProviderOptions:   call.ProviderOptions,
		Temperature:       call.Temperature,
		TopP:              call.TopP,
		TopK:              call.TopK,
		FrequencyPenalty:  call.FrequencyPenalty,
		PresencePenalty:   call.PresencePenalty,
	})
	mu.Unlock()
}

// injectLadderDirective injects the turn's single ladder directive: the
// goal continuation when the goal owes one, the todo nudge otherwise.
//
// The goal continuation supersedes the nudge and folds the nudge's
// content into it, because a turn carries one injected message: the
// model reads the last thing it was told, and two directives compete
// for that place.
func (a *sessionAgent) injectLadderDirective(
	ctx context.Context,
	sessionID string,
	goalRun *goalRun,
	todoRun *todoEnforcementRun,
	msgs []fantasy.Message,
) []fantasy.Message {
	// The nudge is asked for first: it is folded into the goal
	// directive when both want the turn.
	nudge := todoRun.pendingNudge()
	g, hasGoal := goalRun.claim(ctx, sessionID)
	switch {
	case hasGoal:
		return a.appendGoalDirective(ctx, sessionID, g.Directive(g.Verdict, nudge), msgs)
	case nudge != "":
		return a.appendTodoNudge(ctx, sessionID, nudge, msgs)
	default:
		return msgs
	}
}

// appendGoalDirective persists the goal's continuation directive as a
// hidden user message and appends it to the step's input. Hidden
// because it is harness scaffolding: it belongs in the model's history
// and in nobody else's transcript, and a fork must not inherit it.
func (a *sessionAgent) appendGoalDirective(
	ctx context.Context,
	sessionID, directive string,
	msgs []fantasy.Message,
) []fantasy.Message {
	if directive == "" {
		return msgs
	}
	userMessage, err := a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: directive, Hidden: true}},
	})
	if err != nil {
		slog.Warn("Failed to persist the goal continuation", "session_id", sessionID, "error", err)
		return msgs
	}
	return append(msgs, userMessage.ToAIMessage()...)
}

// Evaluate implements goal.Evaluator: the goal is judged with the
// agent's small model, so the check costs one cheap call per turn. The
// call carries the goal and the turn's last message, and nothing else.
func (a *sessionAgent) Evaluate(ctx context.Context, req goal.Request) (goal.Verdict, error) {
	small := a.smallModel.Get()
	if small.Model == nil {
		return goal.Verdict{}, errors.New("no small model is available to judge the goal")
	}
	evaluator := fantasy.NewAgent(
		small.Model,
		fantasy.WithSystemPrompt(goal.EvaluatorPrompt),
		fantasy.WithMaxOutputTokens(goalEvaluatorMaxTokens),
		fantasy.WithUserAgent(userAgent),
	)
	resp, err := evaluator.Stream(ctx, fantasy.AgentStreamCall{
		Prompt:  goal.EvalPrompt(req),
		Headers: sessionHeaders(tools.GetSessionFromContext(ctx)),
	})
	if err != nil {
		return goal.Verdict{}, err
	}
	return goal.ParseVerdict(resp.Response.Content.Text()), nil
}
