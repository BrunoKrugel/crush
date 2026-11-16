---
id: goal-mode
title: Goal mode
sidebar_position: 15
description: An autonomous, turn-budgeted loop — a small-model evaluator checks the goal after each turn and injects a continuation until it is met, the budget runs out, or you cancel it.
---

# Goal mode

:::info[Fork feature]
Goal mode is an addition in this fork, reimplemented from the
[`taigrr/crush`](https://github.com/taigrr/crush) design (PLAN.md #5).
Nothing about it exists upstream. See [What this fork adds](/fork).
:::

Some tasks have a finish line instead of a question. *"Make the CI green."*
*"Migrate every caller of `oldApi` to `newApi`."* *"Bring the configuration
docs back in line with the code."* A prompt gets you one turn, and a turn ends
when the model decides it is done — which is often before the job is.

Goal mode takes the outcome instead of the instruction. You state the goal
once, and Crush keeps working on it: when a turn ends, a cheap small-model
evaluator looks at what happened and answers a single question — *is the goal
met?* If it is not, Crush injects a continuation directive and runs another
turn. The loop stops when the evaluator says the goal is met, when the turn
budget runs out, or when you cancel it.

## When to use it

While a goal is active, the header shows a `GOAL` badge with the
remaining turn budget, so the state is visible at a glance.

Goal mode is for work that is long, mechanical, and checkable:

- *"make the CI green"* — one failure at a time, until the build passes;
- *"migrate every caller of `oldApi`"* — a long tail of small edits;
- *"update the docs for the changed commands"* — a diff you would otherwise
  have to check by hand.

It is a poor fit for open-ended work. The evaluator is a small model reading
the transcript, so it can only judge the goal as you phrase it — *"make the
codebase nicer"* has no finish line for it to find. Say the goal so that
"done" is visible.

## How the loop works

1. You type `/goal make the CI green`. The goal is recorded on the session.
2. Crush runs a turn, exactly as it would for any prompt.
3. When the turn ends, the evaluator — a cheap small model, not the main one —
   reads the turn and returns a verdict: met, or not met.
4. Not met, and budget left: Crush injects a **continuation directive** and
   starts another turn. The directive is written fresh each time; it is not
   your original prompt replayed.
5. Met: the goal is done and the loop stops. Budget gone: the goal is
   exhausted and the loop stops.

The goal rides on the session the way the rest of the session state does: the
goal text, the turn budget, the evaluator's model, and the state — active,
met, or exhausted. Nothing else is needed to resume the loop, because there
is nothing else: the loop runs inside the same run machinery the rest of the
agent uses, not beside it.

## What bounds it

Goal mode adds a *progress signal*, not a second enforcement layer. It lives
inside the machinery this fork already has — the
[todo enforcement ladder](/agents/todo-enforcement) and its watchdogs — and
those stay authoritative. Goal mode never loosens them.

| Bound | What it does |
| --- | --- |
| Turn budget | 25 turns by default, and 25 is the hard cap: no goal runs for more than 25 turns. The budget counts turns, not wall-clock time — one long turn is still one turn. |
| `hard_timeout` | Ends a run that has lasted too long, goal or no goal. |
| `inactivity_timeout` | Ends a served run that yields no events at all; the run ends as failed rather than killed. |
| `stall_window` | Ends a run that has stopped making progress — for a run with a todo list, a list that has not changed for that long. |
| `kill_after_nudges` | Counts ignored **continuations** the way it counts ignored nudges. A run that keeps declaring victory while the evaluator disagrees trips the same kill rung. |

The watchdogs are goal-agnostic: they do not know that a goal is running, and
they end the same runs they would end without one.

The kill rung is wired exactly as it is for nudges. Only a dispatched run has
a parent that can re-dispatch it, so only a dispatched run can be killed for
ignoring continuations — and when it is, the reason the parent gets is the
evaluator's verdict.

## Starting and stopping

```text
/goal make the CI green
/goal off
```

- `/goal <text>` — set the goal on the current session and start the loop. A
  new goal replaces the one already running.
- `/goal off` — cancel the goal. The loop stops: no continuation is injected
  and no further turn is started.

## One injection channel

The ladder injects at most one instruction per turn, and goal mode does not
add a second channel next to it. The order is fixed:

**goal continuation, else todo nudge, else nothing.**

When the goal continuation is the instruction for a turn, the todo nudge
folds into it — the directive carries the nudge's own reminder along, so the
agent is still told to refresh its todo list. A turn never gets two
injections fighting over it, and the nudge is never silently dropped.

## What lands in the transcript

While a goal is active, the continuation directive is recorded as a
**hidden** message. Hidden messages are harness scaffolding, not
conversation: a [fork](/features/forking) leaves them behind, so a branched
session never replays a continuation as something you said, and a hidden
message never triggers a [snapshot](/features/snapshots) of its own.

## Limits

- **The evaluator is small.** It reads the transcript and judges the goal as
  written. A vague goal gets a vague verdict.
- **The budget is a ceiling, not a promise.** When the turns run out the goal
  is exhausted, not met — the work may still be unfinished, and the
  transcript says which of the two happened.
- **Goal-seeded dispatches are not here.** A `/goal` that started a
  dispatched worker would inherit the A2A records, the agent card, inspect
  mode, and every watchdog for free — but it would move the work into a
  child session, out of the conversation you started it in. That is a
  deliberate alternative to the design above, reconsidered when the swarm
  work (PLAN.md #3) lands; not in this pass.
