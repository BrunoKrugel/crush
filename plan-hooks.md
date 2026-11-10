# Porting plan: upstream PR #3146 — hooks for the whole agent lifecycle

Plan for pulling
[`charmbracelet/crush#3146`](https://github.com/charmbracelet/crush/pull/3146)
("feat(hooks): add more lifecycle hooks", merged upstream 2026-10-26,
36 files, +2,027/−732) into [`brunokrugel/crush`](https://github.com/brunokrugel/crush).

This is the third porting plan, after [PLAN.md](PLAN.md) (taigrr features)
and [plan-auto.md](plan-auto.md) (native auto mode). It is the highest-risk
port of the three, because the fork does not merely diverge from upstream —
it independently built a second half of the same feature.

## What upstream merged

Hooks extend beyond `PreToolUse` to the whole agent lifecycle:

| Event | Fires | Can block? |
| --- | --- | --- |
| `SessionStart` / `SessionEnd` | session lifecycle | observable |
| `TurnStart` / `TurnEnd` | every turn; `TurnEnd` carries `last_assistant_message` | observable |
| `PreToolUse` / `PostToolUse` / `PostToolUseFailure` | tool calls; `PostToolUse` can replace output (`updated_output`), add context, or halt with feedback | blockable |
| `PermissionRequest` / `PermissionResult` | the permission prompt shown; granted/denied/cancelled (result fires outside the permission lock, always follows its request) | observable |
| `PreCompact` / `PostCompact` | context compaction; `PreCompact` can block | blockable |
| `StopFailure` / `Interrupt` | run failure / interruption | observable |

Plus a structural refactor:

- **One `hooks.Event` type and one Runner** for every event, owned by the
  app (`App.hookRunner`). The runner **re-reads the config store on each
  event**, so config reloads apply without restart.
- **Claude Code compatibility**: payload fields use CC's names
  (`hook_event_name`, `tool_response`, `last_assistant_message`),
  `CLAUDE_PROJECT_DIR` is exported, and CC output forms are accepted
  (`hookSpecificOutput`, `decision: "block"`, `continue: false`).
- **Matcher rules**: matchers are only valid on the five tool-call events;
  unknown event names and misused matchers are config errors.
- **Sub-agents fire no turn or tool events.**
- Permission service publishes a "cancelled" notification when a prompt's
  caller is abandoned (UI/proto updated).
- `internal/agent/turn_hooks.go` (new file) hosts the turn/tool event
  wiring; `internal/app/hooks.go` (new file) hosts the app-owned runner.
- Docs: full `docs/hooks/README.md` rewrite; builtin hook skills updated.

## What the fork has today

The fork's hooks engine (`internal/hooks/`, ~1.7k lines) predates this PR
and already covers part of it, differently:

- **Events**: `PreToolUse`, `PreCompact`, `PostCompact` (fork inventions
  with their own payloads), and — from the auto-mode port —
  `PrePermission` and `PermissionDenied`, the latter two carrying a
  reasoning-blind transcript (`internal/transcript`) opt-in via
  `include_transcript` on the hook config.
- **Runners**: multiple, per consumer, built from a static config slice at
  construction time — the coordinator builds one for `PreToolUse`
  (`wrapToolsWithHooks` → `hookedTool`), the agent builds one per
  compaction event through a factory, and the permission service owns a
  `prePermissionRunner`. Reload happens by reconstruction, not re-read.
- **Decisions**: `aggregate()` produces allow/deny/updated-input with
  `shallowMerge` patches; `hookedTool` additionally attaches fork-specific
  hook metadata (`buildHookMetadata`) to tool results.
- **Auto mode is not a hook**: it is an in-process policy hook
  (`permission/hooks.go`, `SetPermissionHooks`) registered separately from
  the shell-hook runner, with its own verdict order inside the permission
  service.

## The collision

Both sides built permission hooks and compaction hooks independently, with
different names and semantics:

| Concern | Fork | Upstream #3146 | Collision severity |
| --- | --- | --- | --- |
| Permission events | `PrePermission` / `PermissionDenied` (fork) | `PermissionRequest` / `PermissionResult` (upstream; result also covers granted + cancelled) | **High** — same ground, different names and semantics |
| Compaction events | `PreCompact` / `PostCompact` (fork payloads, cannot block) | same names, upstream payloads, `PreCompact` can block | **High** — same names, different payloads and powers |
| Runner ownership | per-consumer runners, static config | one app-owned runner, re-reads config per event | **Medium** — structural, mechanical once decided |
| `hookedTool` | fork's decision aggregation + hook metadata | upstream's own changes | **Medium** — both sides edited the same file |
| `agent.go` | goal mode, dispatch, compaction checkpoints | turn hooks wiring | **Medium** — upstream's change lands mid-file |
| Auto mode | in-process policy hook, no shell hooks | not touched upstream | None — but the verdict order must be specified |
| `include_transcript` | fork-only field + `internal/transcript` | upstream has `tool_response`/`last_assistant_message` instead | **Medium** — decide keep/extend/drop |

## Verdict: reconcile onto upstream's shape

**Do not merge upstream's branch.** As with the auto mode port, the fork
has diverged in nearly every file the PR touches. But the *deduplication*
rule from plan-auto.md applies with full force here, because upstream is
now merged: **the fork should adopt upstream's event names, payload field
names, runner ownership, and Claude Code compatibility wherever the fork
has no stronger reason to differ**, so the next upstream sync does not
have to reconcile two vocabularies again.

Concretely, the fork renames, not upstream:

- `PrePermission` → `PermissionRequest`; `PermissionDenied` →
  `PermissionResult` (the fork's denial-only event becomes upstream's
  superset: granted, denied, *and* cancelled — the cancelled path is the
  new capability, and it is also what feeds the "caller abandoned"
  notification). User configs keep working: accept the old names as
  aliases at load time, with a one-time deprecation warning.
- `PreCompact` / `PostCompact` payloads move to upstream's shape. The
  fork's compaction-checkpoint detail (its own invention) rides in an
  extra payload field rather than a divergent base shape.

The fork's genuinely additive pieces that upstream lacks — the
reasoning-blind transcript on permission events (`include_transcript`),
hook metadata on tool results, and the auto-mode policy hook — stay, but
ride on top of the unified runner rather than beside it.

## The plan

Work in this order, one commit per step, each independently reviewable.
Steps 1–2 are the foundation; steps 3–7 adopt events in decreasing
coupling order.

### Step 1 — the event model and unified runner (`internal/hooks`)

Take upstream's `internal/hooks` package near-verbatim: the `Event` type,
the single Runner, per-event config re-read, matcher validation, and the
Claude Code payload compatibility in `input.go`. Then reconcile the fork's
additions onto it:

- Port `RunPermission`'s transcript support into the unified runner as an
  optional capability of the permission events (the fork's
  `include_transcript` field stays on `HookConfig`).
- Port the fork's `PreCompact`/`PostCompact` payload detail into
  upstream's compaction payload as extension fields.
- Keep the fork's `aggregate()` decision semantics for the blockable
  events where the fork's behaviour is a superset (updated-input merging);
  adopt upstream's `halt` / `updated_output` / add-context vocabulary as
  the canonical output form, accepting the fork's older spellings as
  aliases.
- Port the fork's test suite (aggregate, shallowMerge, runner gating) onto
  the new shape; port upstream's new tests verbatim where they do not
  collide.

### Step 2 — one runner, app-owned (`internal/app`, `internal/config`)

- Port upstream's `internal/app/hooks.go`: the app owns `hookRunner`; the
  TUI and the non-interactive runner report the current session ID for
  `SessionEnd`.
- Delete the fork's per-consumer runner construction: the coordinator's
  `PreToolUse` runner, the agent's compaction hook factory, and the
  permission service's `prePermissionRunner` all read from the app-owned
  runner instead.
- Config: adopt upstream's event-name validation and matcher rules
  (`internal/config/hooks.go`), with the fork's old event names accepted
  as aliases (`PrePermission` → `PermissionRequest`,
  `PermissionDenied` → `PermissionResult`) with a deprecation warning.
  The fork's `include_transcript` stays. The crushrc `hook` builtin needs
  no change beyond accepting the new event names.

### Step 3 — permission events (`internal/permission`, `internal/proto`, UI)

- Rename and re-seat: `PermissionRequest` fires where the fork's
  `PrePermission` fired today; `PermissionResult` replaces
  `PermissionDenied` and additionally fires on grant and on cancel.
- Adopt upstream's **result-outside-the-lock** discipline: the fork's
  runner currently runs inside the permission service path; the result
  event must not hold the permission lock (it can run user commands).
- Port upstream's "cancelled" notification: when a prompt's caller is
  abandoned, the permission service publishes a cancelled outcome
  (`internal/proto/permission.go`, `internal/server/events.go`,
  `internal/ui`), and `PermissionResult` fires for it.
- **Specify the verdict order** — the piece both engines have and only
  one document can win: hard deny → allowlist → **auto mode (policy
  hook)** → `PermissionRequest` hooks → prompt. Auto mode stays an
  in-process classifier, not a shell hook; a `PermissionRequest` hook can
  still veto (deny or force-prompt) what auto mode escalated. Write this
  down in `docs/hooks/README.md`.

### Step 4 — compaction events (`internal/agent`)

- Adopt upstream's payload and the blockable `PreCompact`; the fork's
  checkpoint detail rides along as extension fields.
- The fork's compaction checkpoints keep working: a blocked `PreCompact`
  skips compaction and leaves the conversation as it was (upstream's
  semantics already say this — the checkpoint machinery just stops
  earlier).
- Wire through the app-owned runner; delete the per-event factory.

### Step 5 — tool events beyond PreToolUse (`internal/agent`)

- Port upstream's `internal/agent/turn_hooks.go` (new file, clean port)
  for `TurnStart`/`TurnEnd`/`StopFailure`/`Interrupt`, keeping the fork's
  rule that sub-agents fire no turn/tool events.
- `PostToolUse` / `PostToolUseFailure` land in `hooked_tool.go`: adopt
  upstream's `updated_output` / halt / add-context output vocabulary; keep
  the fork's hook-metadata attachment as an additional effect. The fork's
  `shallowMerge` updated-input path for `PreToolUse` stays, expressed in
  upstream's output form.
- `agent.go` needs hand-merging: the fork's goal mode, dispatch, and
  compaction checkpoints surround every touch point.

### Step 6 — session lifecycle events (`internal/app`, `internal/workspace`)

- `SessionStart` / `SessionEnd` from upstream's `internal/app/hooks.go`
  plus the current-session tracking (TUI + non-interactive runner).
- `internal/workspace`: the fork's client/server split means
  `SessionEnd` must fire on the server side where sessions actually end —
  check upstream's placement against the fork's backend.

### Step 7 — Claude Code compatibility and docs

- Payload field names (`hook_event_name`, `tool_response`,
  `last_assistant_message`), `CLAUDE_PROJECT_DIR`, and the CC output
  forms — port upstream's `input.go` handling and tests verbatim; the
  fork's `CRUSH_TRANSCRIPT` env var stays alongside.
- Rewrite `docs/hooks/README.md` from upstream's version, re-adding the
  fork's sections (transcript, auto-mode ordering, crushrc examples).
  Port upstream's #3956 docs syntax fix while in there.
- Update the builtin `crush-hooks` / `crush-config` skills.

## Status

| Step | Status |
| --- | --- |
| 1. Unified event model + runner | **Done** (2026-10-27, 987c6dcaa) |
| 2. App-owned runner | **Done** (2026-10-27, dae49dcf2) — also fixed the gap where the permission shell hooks had no production wiring |
| 3. Permission events | **Done** (2026-10-27, d10495277) — results on every outcome incl. cancelled; cancelled notification through proto/server/client/UI |
| 4. Compaction events | **Done** (achieved by 1–2; the deny-skip semantics and fork detail verified by tests) |
| 5. Tool/turn events | **Done** (2026-10-27, b168ac145) — ErrCompactionBlocked contract adopted |
| 6. Session lifecycle | **Done** (2026-10-27, 175bb8856) |
| 7. CC compat + docs | **Done** (2026-10-27, this commit) — README and docs-site updated |

Open follow-ups (tracked below, not blocking): `include_transcript` on turn
events (open question 3); `StopFailure`-and-watchdogs design note (open
question 4); snapshot/fork remote endpoints for the client SDK.

## Sequencing and dependencies

```
Step 1  internal/hooks          (foundation; nothing else lands before it)
Step 2  app-owned runner        (needs 1; unlocks 3-6)
Step 3  permission events       (needs 1, 2; highest collision risk)
Step 4  compaction events       (needs 1, 2)
Step 5  tool/turn events        (needs 1, 2)
Step 6  session lifecycle       (needs 2)
Step 7  CC compat + docs        (after everything; partly landable per step)
```

Steps 3, 4, 5, 6 are mutually independent once 1–2 land; do 3 first while
the permission-model context is fresh. Each step must keep the fork's
existing hooks working: a user's current `PreToolUse` hook and a
`PrePermission` alias must behave exactly as before the port.

## Test strategy

- Port upstream's new test files (`internal/hooks/hooks_test.go`,
  `internal/config/hooks_test.go`, `reload_hooks_test.go`,
  `internal/agent/hooks_test.go`, `app/hooks_test.go`,
  `permission_test.go`) and reconcile with the fork's existing suites.
- Alias tests: old fork event names load and fire; deprecation warning
  fires once.
- Ordering tests for the permission path: hard deny > allowlist > auto
  mode > `PermissionRequest` hook > prompt, including the result-outside-
  the-lock rule.
- Transcript tests: `include_transcript` on `PermissionRequest`/
  `PermissionResult` carries the reasoning-blind transcript
  (`CRUSH_TRANSCRIPT`), unchanged from the fork's current behaviour.
- Goal mode + compaction: a blocked `PreCompact` leaves checkpoints
  untouched; goal continuations still fire with turn hooks active.

## Open questions

1. **Alias policy.** Keep `PrePermission`/`PermissionDenied` working
   indefinitely, or warn-and-remove after one release? (Recommend:
   aliases with a warning, removal noted in the docs but not scheduled —
   the fork's own users are few and the alias is cheap.)
2. **Hook metadata on tool results.** The fork attaches structured hook
   metadata to tool results (`buildHookMetadata`); upstream does not.
   Keep as a fork extension (default on), or gate it behind a config so
   behaviour converges with upstream by default? (Recommend: keep, it is
   additive and invisible to upstream parity.)
3. **`include_transcript` beyond permission events.** Upstream's
   `TurnEnd` carries `last_assistant_message`; a transcript on turn events
   would be the natural extension. Take it now or as a follow-up?
   (Recommend: follow-up — the port is already large.)
4. **`StopFailure` and the watchdogs.** The fork's goal-mode and dispatch
   watchdogs kill runs; does `StopFailure` fire for watchdog kills, and
   can a `StopFailure` hook observe the kill reason? Needs a small
   design note during step 5.
5. **Auto mode's future.** Once `PermissionRequest` hooks exist, a user
   can approximate auto mode with a shell hook. The plan keeps native
   auto mode (it is in-process, quota-managed, and prompt-file-driven),
   but the docs should say when to prefer which.

## Cost

Overall: **L** — comparable to the auto mode port, with more semantic
reconciliation and less new code. The unified runner (steps 1–2) is most
of the mechanical work; the permission events (step 3) and `agent.go`
hand-merging (step 5) are where the care goes.
