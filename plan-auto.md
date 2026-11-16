# Porting plan: upstream PR #3874 — native auto mode

Plan for pulling
[`charmbracelet/crush#3874`](https://github.com/charmbracelet/crush/pull/3874)
("Add native auto mode: an LLM safety classifier for tool permissions") into
[`brunokrugel/crush`](https://github.com/brunokrugel/crush).

This is a separate file from [new-plan.md](new-plan.md) because it is a
different kind of work: not a small fix to port, but a feature that has to be
**rebuilt on this fork's base**, because the fork has diverged from upstream
in almost every file the PR touches.

| | |
| --- | --- |
| Upstream PR | [#3874](https://github.com/charmbracelet/crush/pull/3874) |
| Author | [`gurnben`](https://github.com/gurnben) (branch `option2-native-automode`) |
| State | Open, not a draft, mergeable against upstream `main` |
| Size | 90 files, +6179 / -513 |
| New package | `internal/automode` |
| Branch | `option2-native-automode` |

## What the feature is

An opt-in **auto mode**. When it is on, a safety classifier decides each tool
call *before* it reaches the human permission prompt:

- **ALLOW** — runs immediately, no prompt.
- **DENY** — blocked with a categorized reason; denial counters are fed back
  to the agent.
- **ESCALATE** — falls through to the normal permission prompt.

The goal is the middle ground between yolo mode and prompt-per-action:
obviously safe work proceeds silently, obviously dangerous work is blocked with
an explanation, and genuinely ambiguous actions still ask a human.

It is default-off. Nothing changes unless it is enabled.

## How it works

Three tiers, cheap to expensive:

1. **Static rules.** Read-only tools auto-allow. Dangerous patterns
   (`rm -rf`, `chmod 777`, force-push, …) and banned commands (the existing
   hard-deny lists) auto-deny. Protected paths (`.git/`, shell profiles, SSH
   keys) always route to the classifier.
2. **Two-stage LLM verdict.** A fast YES/NO filter escalates to
   chain-of-thought only for flagged actions. The classifier sees a
   **reasoning-blind transcript** — user messages and tool calls, with
   assistant prose and tool outputs stripped — so it judges intent and can
   spot prompt-injection pressure, while the stripped outputs keep the
   structural injection defence.
3. **Quotas.** 3 consecutive / 20 total denials pause auto mode back to
   prompting. Every block message carries the counters
   (`Blocked (2/3 consecutive, 5/20 total): …`).

Everything fails closed: classifier errors escalate rather than allow.

## What is in the branch

- **`internal/automode`** — the classifier package: `rules.go` (static
  classification lists and the command tokenizer), `classifier.go` (the LLM
  stages), `prompts.go`, `quotas.go`, `generate.go`, `automode.go` (the
  pipeline).
- **Permission-service integration** — verdicts feed the existing grant/deny
  flow. Two new hook events: `PrePermission` (before the prompt) and
  `PermissionDenied` (after a denial, fire-and-forget).
- **TUI** — `ctrl+y` cycles normal → auto → yolo, a *Toggle Auto Mode*
  command-palette entry, an "evaluating" indicator while classifying, and an
  **Auto Mode** slot in the model picker.
- **Config** — an `auto_mode` section, **user-level only by design** (a
  repository should never be able to grant itself auto mode): classifier
  model, token budget, timeout, fail-open, quotas, environment prose,
  prompt-file overrides, transcript cap. The classifier model falls back:
  explicit selection → small slot → large slot; with no model at all it runs
  rules-only.
- **Escalation feedback** — when a human approves an escalated action, the
  tool result carries a short note
  (`[auto-mode] this action was escalated and approved by the user`) into the
  model's live conversation, so the agent learns what sits outside auto mode's
  trust.

## Verdict: reimplement, do not merge

**Do not merge upstream's branch.** It touches 90 files, and the fork has
diverged in nearly all of them. A merge would produce a conflict storm and a
history that nobody can read. This is the same call we made for the taigrr
port (see [PLAN.md](PLAN.md)): reimplement on our base, using the upstream
branch as the design reference.

The upstream branch is still reachable as a reference:

```bash
git fetch upstream pull/3874/head:pr-3874-auto
git log --oneline pr-3874-auto
```

`gurnben` is already a credited contributor to this fork (context compaction
with checkpoints, via `feat/context-compaction`), so keeping the port
attributed to them matters. If upstream merges the PR, the fork's copy and
upstream's will collide — see "Deduplication" at the end.

## Why the merge would be a conflict storm

The fork is not a small delta from upstream. It carries:

- its own **TUI** (`internal/ui/` restructured into `chat/`, `dialog/`,
  `model/`, `completions/`, `textarea/`, …, with a rewritten styling system in
  `internal/ui/styles/`);
- its own **hooks engine** (`internal/hooks/`, with the `ConfigBuilder`
  context gate and `PreCompact` / `PostCompact` events the upstream PR does
  not have);
- its own **permission plumbing** (`WithHookApproval`, `hookedTool`, the
  `internal/agent/tools` set with `multiedit`, `question`, `download`, the
  semantic-search tools, the honcho tools, the dispatch tools);
- its own **coordinator** (`internal/agent/coordinator.go` with the named
  agents, `internal/agent/dispatch.go`, the A2A stack in `internal/a2a/`);
- its own **config** (`internal/config/` with the `crushrc` bash config, the
  `honcho` and `embeddings` sections, the `agent` definitions);
- its own **shellconfig** builtins (`internal/shellconfig/` with the
  `ConfigBuilder` gate on the context).

Concretely, of the 90 files upstream touches, these are the ones where the
fork has diverged enough that a merge is a rewrite rather than a merge:

| Upstream file | Why it is hard |
| --- | --- |
| `internal/agent/agent.go` | The fork added task sessions, the title errgroup, compaction checkpoints, and the dispatch wiring. The PR's change is in the middle of that. |
| `internal/agent/coordinator.go` | The fork's coordinator is a different shape (named agents, A2A). The PR adds classifier-model resolution to it. |
| `internal/agent/tools/*.go` (14 files) | The fork's tool set is different and larger. The PR adds a `denied_response` wrapper to each; every one needs redoing against the fork's version. |
| `internal/hooks/{hooks,input,runner}.go` | The fork's hook engine is its own: `ConfigBuilder` gating, `PreCompact`/`PostCompact`, `include_transcript` does not exist here. The PR adds `PrePermission` / `PermissionDenied` and a transcript payload — all on top of the fork's engine, not upstream's. |
| `internal/permission/permission.go` | The fork has the hook-approval path and its own skip logic. The PR's verdict integration lands in the middle of it. |
| `internal/config/{config,store}.go` | The fork's config has the `crushrc` layer and its own sections. `AutoModeConfig` is new but has to go through the fork's clone-for-write and section model. |
| `internal/shellconfig/*` | The PR adds an `auto_mode` builtin. The fork's `shellconfig` has its own builtin set and the `ConfigBuilder` gate. |
| `internal/ui/**` (14 files) | The fork's TUI is a rewrite. The `ctrl+y` cycle, the palette entry, the "evaluating" indicator, and the model-picker slot all have to be re-implemented against the fork's components. |
| `internal/workspace/*` | The fork's workspace layer carries the shared-server and client-server work. |
| `internal/backend/*`, `internal/server/*`, `internal/proto/*` | The fork has the A2A and channel-reply additions in these layers. |
| `schema.json` | The fork's schema is generated from the fork's config, and includes the fork's own sections. |

The genuinely portable parts are the new files that touch nothing the fork
changed:

- `internal/automode/*` — the classifier package itself. It imports
  `charm.land/fantasy` and little else. **Take these near-verbatim.**
- `internal/agent/noted_tool.go` — the escalation-note tool wrapper. Small
  and self-contained.
- `internal/agent/automode_wiring.go` — needs rework against the fork's
  coordinator, but the shape carries over.

## The plan

Work in this order, one commit per step, each independently reviewable.

### Step 1 — the classifier package (`internal/automode`)

Port the package as-is:

- `rules.go` — static lists, `IsReadOnly`, `IsWriteTool`, `IsLowRisk`,
  `IsDangerous`, `IsBanned`, `IsSafeCommand`, `IsProtectedPath`, plus the
  `tokenize` / `splitCommandSegments` helpers. The `tokenize` function is
  deliberately simple (it only needs to distinguish `curl` from `nocurl`),
  which is why it ports cleanly.
- `quotas.go` — per-session denial counters. In-memory, session-scoped.
  Straight port.
- `classifier.go` — the two-stage LLM verdict. Needs a check: upstream builds
  the model through the coordinator; the fork's coordinator has a different
  `buildProvider` signature.
- `prompts.go` — the stage-1 and stage-2 prompts. Port as-is; they are the
  security-critical part and should not be rewritten.
- `generate.go`, `automode.go` — the pipeline and the `Options` struct.

**Adapt the tool-name lists to the fork.** Upstream's `readOnly` set includes
tools this fork does not have and omits ones it does:

| Upstream list | Fork reality |
| --- | --- |
| `lsp_diagnostics`, `lsp_references`, `lsp_symbols`, `lsp_definition`, `lsp_call_hierarchy` | all present — keep |
| `sourcegraph`, `crush_info`, `crush_logs` | present — keep |
| `fetch`, `agentic_fetch` | present — keep |
| `web_search` | present as `web_search` — keep |
| `read`, `search` | **not in this fork** — drop |
| `todos`, `question` | present — keep |
| `job_output`, `job_kill` | present — keep |
| — | **add**: `glob`, `grep`, `lsp_restart`, `list_mcp_prompts`, `call_mcp_prompt`, `list_mcp_resources`, `read_mcp_resource` |
| — | **decide on**: `semantic_search`, `semantic_index`, `recall`, `honcho_search`, `honcho_chat` (read-only, but `honcho_chat` calls a model and is slow — upstream's `honcho_*` equivalents are absent) |

`download` is in neither read-only nor write — upstream's list leaves it to
the classifier. Keep that.

**The dangerous-pattern and banned-command lists** are new to this fork. The
fork has no hard-deny list today — it has `internal/agent/tools/safe.go`, a
shell-AST analysis that decides which bash commands are read-only enough to
skip the permission prompt (`isSafeReadOnly`, used at
`internal/agent/tools/bash.go:256`). That is the *opposite* direction from
auto mode's dangerous list: the fork asks "is this safe to run without
asking?", upstream asks "is this dangerous enough to refuse?".

Two consequences to work through:

- The fork's `safe.go` is an AST-based analysis of the parsed shell command;
  upstream's `rules.go` is a string `Contains` scan over a tokenized
  command. They will disagree on edge cases (`ls $(rm -rf /)`, `timeout 5 rm
  -rf /tmp/x`, `env rm -rf /tmp/x` — the fork already has tests for these).
  Decide which is authoritative, or the two layers will fight.
- Introducing a deny list where none exists changes behaviour for everyone:
  a command that auto mode denies but plain Crush would have prompted for is
  now blocked outright. That is a product decision, not a porting detail.

**Tests.** Port `internal/automode/automode_test.go` and `rules` coverage
verbatim, and add fork-specific cases for the tool names above.

### Step 2 — config (`internal/config`)

- Add `AutoModeConfig` and `AutoModeClassifier` to `internal/config/config.go`
  — the upstream struct definition ports as-is.
- Add `AutoMode *AutoModeConfig` to `Config`, and add it to `cloneForWrite()`
  (the fork's config clones before mutating; a new pointer field must be
  cloned too or a config reload will mutate the live config).
- Add `SelectedModelTypeAutoMode` to the selected-model-type enum.
- **User-level only.** Upstream deliberately refuses to read `auto_mode` from
  a repository-level config. The fork has the same repository-vs-user config
  split (and its own `crushrc` layer), so this constraint must be enforced
  where the fork loads project config, not just copied from upstream's call
  site. This is a security property — a repository must not be able to grant
  itself auto mode — so it deserves an explicit test.
- Regenerate `schema.json` with the fork's own generator.

### Step 3 — `crushrc` support (`internal/shellconfig`)

Port `internal/shellconfig/automode.go` and register the `auto_mode` builtin
in `internal/shellconfig/register.go`:

```bash
auto_mode on|off
auto_mode model <provider>/<id>
auto_mode max-tokens N
auto_mode timeout N
auto_mode fail-open true|false
auto_mode environment "prose describing trusted repos/domains"
auto_mode prompt stage1|stage2 <file>
auto_mode transcript-max-chars N
auto_mode max-consecutive-denials N
auto_mode max-total-denials N
```

The fork's `shellconfig` has the `ConfigBuilder` context gate and its own
builtin set (`provider`, `model`, `mcp`, `lsp`, `permissions`, `hook`,
`honcho`, `option`, `embeddings`, `agent`), so the new builtin must use the
fork's `b.section(...)` / `childMap` / `appendArr` helpers rather than
upstream's.

Also: the `crushrc` config is the fork's primary config format. An
`auto_mode` section that only exists in `crush.json` would be a second-class
feature here, so the builtin is not optional.

### Step 4 — permission integration (`internal/permission`, `internal/agent`)

This is the part that needs the most care.

- **Verdict integration.** The classifier's ALLOW / DENY / ESCALATE feeds the
  existing grant/deny flow. The fork's `permissionService.Request` already
  has the hook-approval short-circuit and the allowlist check, and the
  classifier's verdict has to slot in *after* the hard denies and *before*
  the prompt.
- **Order matters.** The fork has no hard-deny list today, so there is
  nothing for auto mode to accidentally override — but if a deny list is
  introduced (see step 1), it must stay a hard deny: auto mode cannot turn a
  hard-denied command into an allowed one.
- **Fail closed.** A classifier error escalates to the prompt. The fork's
  `fail_open` option defaults to false; keep that.
- **`noted_tool.go`** — the escalation-note wrapper. When a human approves an
  escalated action, the tool result carries
  `[auto-mode] this action was escalated and approved by the user` into the
  model's live conversation. Small, self-contained, port as-is.
- **Denied response** — the PR wraps the tools so a denial returns a
  categorized reason to the model plus the quota counters. The fork has
  40+ tools; upstream changed 14. Work out the rule: the wrapper applies to
  any tool that can be denied, which is every tool that asks for permission.
  Do this once, generically, in the coordinator or the tool decorator — not
  tool by tool, or the fork will drift from upstream on the next merge.

**Recommendation: implement the denial path centrally.** The fork already has
`internal/agent/hooked_tool.go`, a decorator that wraps every tool at the
coordinator level. That is the natural place for the auto-mode verdict: one
wrapper, applied once, instead of fourteen per-tool edits. This diverges
from upstream deliberately, and it is the single biggest reason this port is
cheaper in the fork than upstream.

### Step 5 — hook events (`internal/hooks`)

The PR adds two events to upstream's hook engine:

- `PrePermission` — fires right before the permission service is about to
  prompt. Returning allow or deny short-circuits the prompt entirely;
  silence falls through to the normal prompt.
- `PermissionDenied` — fires after a permission request has been denied.
  Fire-and-forget: it does not block the denial. Carries the permission
  request payload.

Plus a **reasoning-blind transcript** in the hook payload: user messages and
tool calls only, assistant prose and tool outputs stripped, exposed as
`transcript` in the JSON payload and `CRUSH_TRANSCRIPT` in the environment.
Opt-in via `include_transcript` on the hook config.

The fork's hook engine is its own, so this is a port onto the fork's engine,
not upstream's:

- Add the two event constants to `internal/hooks/hooks.go`. The fork already
  has `EventPreToolUse`, `EventPreCompact`, `EventPostCompact`.
- Add `Transcript` to the `Payload` struct in `internal/hooks/input.go`, and
  the `BuildPayloadWithTranscript` / `BuildEnvWithTranscript` variants
  (`CRUSH_TRANSCRIPT`). The fork's existing `BuildPayload` / `BuildEnv` keep
  their signatures and delegate with an empty transcript, so existing hooks
  are unaffected.
- Add a `TranscriptProvider` function type to the runner, and thread it
  through `internal/hooks/runner.go`. The fork's runner has its own
  `ConfigBuilder` gating — the provider has to respect it.
- Add `IncludeTranscript bool` to `HookConfig` in `internal/config/config.go`.
- Wire `PrePermission` / `PermissionDenied` into the permission service.

**The hook events are worth taking on their own.** Upstream says so
explicitly: "Happy to split the hook events into a separate PR if that
reviews better." They are useful to anyone wiring an external classifier, and
they are the mechanism auto mode itself uses. Splitting them into their own
commit — and their own upstream PR request — keeps this port reviewable.

**The transcript payload is a separate decision.** It is a genuinely useful
capability (a hook that wants to know what the session has been doing), but
it is not required for auto mode, which builds its own transcript internally.
Recommend: take the two events in this port, and treat `include_transcript`
as a follow-up.

### Step 6 — TUI (`internal/ui`)

Upstream's TUI changes, and what the fork has to rebuild:

| Upstream | Fork work |
| --- | --- |
| `ctrl+y` cycles normal → auto → yolo | Re-implement against the fork's key handling in `internal/ui/model/keys.go`. The fork's permission-mode cycling already exists (the fork has yolo), so this is an extension of an existing control, not a new one. |
| *Toggle Auto Mode* palette entry | The fork's palette lives in `internal/ui/dialog/commands.go` and `commands_item.go`. The fork has its own palette entries (Skills, MCP, Channels, Fork, …) — add one more, matching the fork's patterns. |
| "evaluating" indicator while classifying | The fork's status/pill surface is `internal/ui/model/status.go` and the pills panel. Needs the fork's own rendering. |
| Auto Mode slot in the model picker | The fork's model picker is `internal/ui/dialog/models.go`. Upstream adds a third slot next to large/small; the fork's picker has its own layout. |
| `internal/ui/styles/{quickstyle,styles}.go` | The fork has its own token-driven styling system (`quickStyle` + themes). New colours go in as theme tokens, never as literals. |

Note the fork also has a `PermissionDenied`-style "no" indicator and its own
notification surface (`internal/ui/notification/`) — the upstream "evaluating"
and "denied" visuals should be matched to the fork's existing shapes rather
than ported pixel for pixel.

### Step 7 — workspace, backend, server, proto

The PR touches `internal/workspace/*`, `internal/backend/*`,
`internal/server/*`, `internal/proto/proto.go` and `internal/client/proto.go`.
The reason is the permission verdict has to survive the client/server split
(the fork has both, plus the A2A stack). These are mechanical once steps 4
and 5 are done, but the fork's versions of these layers carry the channel
reply routing and the A2A work, so expect conflicts.

### Step 8 — docs

- `docs/hooks/README.md` — document `PrePermission` and `PermissionDenied`.
  The fork keeps this file at the same path as upstream, so the upstream doc
  change applies nearly as-is (the fork's copy is larger and has its own
  sections — check the diff before merging). The Docusaurus page
  `docs-site/docs/features/hooks.md` needs the new events too.
- A new docs page for auto mode: `docs-site/docs/features/auto-mode.md`, plus
  a tile on the home page, a row in `docs-site/docs/fork.md` (with the
  source named as `gurnben/crush`), and the `crushrc` configuration section
  in `docs-site/docs/configuration/crushrc.md`.
- `schema.json` regenerated.

## Sequencing and dependencies

```
Step 1  internal/automode            (no dependencies)
Step 2  internal/config              (needs 1 for the model selection)
Step 3  internal/shellconfig         (needs 2)
Step 4  internal/permission + agent  (needs 1, 2)
Step 5  internal/hooks               (independent of 1-4; can land first)
Step 6  internal/ui                  (needs 4)
Step 7  workspace/backend/server     (needs 4, 5)
Step 8  docs                        (needs everything)
```

**Step 5 can and should land first**, as its own commit and its own upstream
PR request. The two hook events are useful without auto mode.

## Conflicts with the rest of the port list

[PR #4073](https://github.com/charmbracelet/crush/pull/4073) (see
[new-plan.md](new-plan.md)) fixes the same three tools that auto mode also
touches — `download`, `lsp_rename`, `lsp_replace_symbol` — because they never
pass `ToolCallID` on their permission request, so a `PreToolUse` hook's
`allow` decision is ignored.

**#4073 must land first.** Auto mode is built on the same permission-request
path, and the fork's auto-mode port will re-touch those three tools. Landing
#4073 first means the auto-mode port starts from a correct base instead of
re-deriving it.

Note also that this fork's `PreToolUse` hook implementation and auto mode's
`PrePermission` event are two different mechanisms that overlap: `PreToolUse`
runs before the tool, `PrePermission` runs at the permission prompt. They
compose — a `PreToolUse` allow pre-approves, and `PrePermission` can still
veto — but the interaction needs to be specified, not just implemented.

## Open questions for the reviewer

1. **Does the fork want a second permission mode at all?** The fork already
   has yolo (persistent `--yolo`, `ctrl+y`-adjacent cycling) and a hooks
   engine that can block tool calls outright. Auto mode is a third
   mechanism in the same space. The case for it: it is strictly between
   yolo and prompt-per-action, and it is the only one that *explains* a
   denial. The case against: three overlapping permission surfaces is a lot
   to reason about, and the fork's hooks engine already lets a user write
   their own classifier as a shell command.
2. **Rebuild or vendor?** Porting `internal/automode` near-verbatim is the
   right default. But the fork's `hooks` package could implement a
   user-supplied classifier hook instead, with no in-process LLM at all —
   `PrePermission` already gives that capability, and the fork's hooks
   already support external commands. That would be a much smaller port and
   a different (arguably better) product. Worth deciding before step 1.
3. **Who owns the classifier model?** Upstream falls back
   explicit → small → large slot, and runs rules-only with no model. The
   fork's config has its own model slots. Confirm the fallback order matches
   the fork's.
4. **Cost visibility.** The classifier is an LLM call per ambiguous tool
   call. The fork's config has a `embeddings` section for a similar
   background-model concern — is there a precedent for how the fork wants to
   expose "this costs money" to the user?

## Deduplication

If upstream merges #3874 before this port lands, the fork's copy and
upstream's will have to be reconciled. Two options:

- **Rebase the fork's port onto upstream's merge** and drop the fork's
  implementation, keeping only the fork-specific adaptations (the central
  denial wrapper, the fork's UI, the fork's config and `crushrc` plumbing).
  This is the preferred outcome and the reason to keep the port small and
  behaviour-compatible with upstream.
- Keep the fork's version and carry the divergence, as with the taigrr
  features.

To make the first option viable, the port should deliberately match upstream's
behaviour and file layout wherever the fork has no reason to differ. The
places we knowingly diverge (the central denial wrapper, the fork's UI) are
listed in step 4 and step 6 above.

## Status

| Step | Effort | Status |
| --- | --- | --- |
| 1. `internal/automode` package | M | Not started |
| 2. Config | S | Not started |
| 3. `crushrc` builtin | S | Not started |
| 4. Permission integration | L | Not started |
| 5. Hook events (split out, land first) | M | Not started |
| 6. TUI | L | Not started |
| 7. Workspace / backend / server / proto | M | Not started |
| 8. Docs | S | Not started |

Overall: **L**. This is a feature port, not a fix port. The classifier
package itself is the easy part; the permission integration, the hook events
and the TUI are the work.
