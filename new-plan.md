# Porting plan: upstream pull requests into this fork

Tracks the [`charmbracelet/crush`](https://github.com/charmbracelet/crush) pull
requests we want in [`brunokrugel/crush`](https://github.com/brunokrugel/crush)
but that are not upstream `main` yet.

This file covers the small, self-contained fixes. The large auto-mode PR has
its own file: see [plan-auto.md](plan-auto.md).

## Status

| PR | Title | Size | Status |
| --- | --- | --- | --- |
| [#4105](https://github.com/charmbracelet/crush/pull/4105) | fix(agent): skip automatic titles for task sessions | 2 files, +73/-3 | **Already in** (as `dff81b949`) |
| [#4090](https://github.com/charmbracelet/crush/pull/4090) | fix(run): accept --yolo, which crush run already implies | 4 files, +92/-3 | **Superseded** — the fork already made `--yolo` persistent; only the help text is missing |
| [#4073](https://github.com/charmbracelet/crush/pull/4073) | fix(tools): pass the tool call ID with download and LSP edit permission requests | 4 files, +56/-0 | **Not started** — the bug is present in this fork |

## How we pull a PR in

Every PR here is merged, not cherry-picked by hand, so the merge commit keeps
the upstream authorship and the upstream SHA in the history:

```bash
git fetch upstream pull/4073/head:pr-4073
git merge --no-ff pr-4073
git push origin fork
```

`git log --oneline --merges` then shows the upstream PR number, and when
upstream merges its own copy the duplicate history drops out cleanly. This is
the same rule the rest of the fork follows — see
[PLAN.md](PLAN.md) for the taigrr port and
[docs/fork.md](docs-site/docs/fork.md) for the inventory of what the fork
carries and where each piece came from.

## #4105 — skip automatic titles for task sessions

**Verdict: already in the fork. Nothing to do.**

The upstream fix adds `currentSession.ParentSessionID == ""` to the guard that
starts automatic title generation, so a task session (which already has a
title assigned by the coordinator) does not spend a small-model request
generating a title that then overwrites the assigned one.

The fork carries the same fix, commit `dff81b949` ("fix(agent): skip automatic
titles for task sessions"), landed 2026-10-20 from the same upstream branch
(`fix/subagent-title-requests`). Our `internal/agent/agent.go:1101` reads:

```go
if currentSession.ParentSessionID == "" && !hasUserTextMessage(msgs) {
```

which is the upstream fix plus our own change to how the goroutine is spawned
(the fork runs it through an `errgroup` — `a.titles.Go(...)` — instead of a
bare `go` statement, so the title generation is tracked and waited on during
shutdown).

The upstream test file `internal/agent/subagent_title_test.go` came with the
merge and is in the fork as well.

**Do not re-merge.** A second merge of the same branch would reintroduce the
bare `go a.GenerateTitle(...)` call and drop our errgroup wiring, because the
context lines differ. If upstream lands its own copy, the duplicate history
drops out on the next rebase and nothing changes.

## #4090 — accept `--yolo` on `crush run`

**Verdict: the behaviour is already there; only the documentation part is
missing. Take the text, skip the code.**

### What upstream does

Upstream registers `--yolo` / `-y` on the `run` command as a *local* flag,
hidden, and then reads it through a `yoloFlag(cmd)` helper that only honours
it on the root command. The point is that `crush run --yolo` should not die
with `Unknown flag: --yolo` — the run already auto-approves its own session
permissions, so the flag is a no-op that scripts pass around.

The upstream author also splits the flag two ways on purpose:

- `crush --yolo` keeps working exactly as before (root command, forwarded to
  the workspace).
- `crush run --yolo` parses but is ignored, so it does not silently turn on
  YOLO for a shared server workspace that other clients attach to. The
  upstream reasoning: workspace flags are first-wins, so forwarding the flag
  would make every later client of that directory skip permission prompts too.

### What this fork does instead

The fork solved the same problem earlier and differently. `internal/cmd/root.go:68`
registers `--yolo` as a **persistent** flag:

```go
rootCmd.PersistentFlags().BoolP("yolo", "y", false, "Automatically accept all permissions (dangerous mode)")
```

with the comment explaining why:

```
// yolo must be PERSISTENT: a root-local flag is invisible to subcommands,
// ... `crush --yolo run …` — died with "unknown flag", which crash-looped
```

A persistent flag is inherited by every subcommand, so `crush run --yolo` and
`crush run -y` already parse in this fork. The upstream approach of
registering a *local* flag on `runCmd` would **panic at startup** here —
cobra rejects a second `yolo` flag with `run flag redefined: yolo`. Upstream
says as much in its own PR body, about a different PR (#2831, sysadmin mode)
that also makes the flag persistent; the same conflict applies to us.

So: **do not take the code.** Take only the documentation half.

### What to take

1. **The `crush run --help` text.** Upstream adds a note saying that nobody is
   around to answer permission prompts in non-interactive mode, so `crush run`
   accepts its session's tool permissions automatically, and that passing
   `--yolo` is allowed but has no extra effect. This is genuinely missing
   from the fork and is worth having — it explains a behaviour that otherwise
   looks like a bug.

   File: `internal/cmd/run.go` (the `run` command's `Long` description).

2. **The README paragraph.** Upstream adds the same explanation to the "You
   only live once" section of `README.md`. The fork's README is heavily
   rewritten (it is the fork's front page), so this needs a hand-written
   version in the fork's own voice rather than a merge.

3. **Optionally the upstream test file** `internal/cmd/yolo_flag_test.go` —
   but only the parts that still make sense. `TestRunCmdAcceptsYolo` and
   `TestYoloFlagHonoredByRoot` are worth having as regression tests for the
   persistent-flag behaviour. `TestYoloFlagIgnoredByRun` tests upstream's
   `yoloFlag` helper, which does not exist here and should not: in this fork
   `crush run --yolo` *is* forwarded to the run's workspace, because the flag
   is persistent. Port that test as a statement of what the fork does, or
   leave it out.

### Note on `crush run --yolo` semantics here

Because our flag is persistent, `crush run --yolo` in this fork *does* forward
YOLO to the run's workspace, unlike upstream. That is the pre-existing fork
behaviour for every subcommand, not something this PR changes. If the shared
server workspace concern (upstream's reason for ignoring the flag on `run`)
turns out to matter here, the fix is a fork-local `yoloFlag(cmd)` helper that
only honours the flag on the root command — the same shape upstream uses, but
reading the *persistent* flag. That is a behaviour change and out of scope
for a port; it is worth a separate issue if we want it.

### Effort

S. Two small text edits plus an optional test port. No behavioural risk.

## #4073 — pass the tool call ID with download and LSP edit permission requests

**Verdict: real bug, present in this fork. Take the fix.**

### The bug

A `PreToolUse` hook that returns `{"decision": "allow"}` is supposed to
pre-approve the permission prompt for that tool call. The fork implements
this in `internal/agent/hooked_tool.go:85`:

```go
if result.Decision == hooks.DecisionAllow {
    ctx = permission.WithHookApproval(ctx, call.ID)
}
```

and `internal/permission/permission.go:233` honours it only when the request
carries the matching tool call ID:

```go
if hookApproved(ctx, opts.ToolCallID) {
```

Three tools in this fork never set `ToolCallID` on their permission request,
so the approval is silently ignored and the user is still prompted — exactly
what the hook said not to do:

| File | Tool | Current state in the fork |
| --- | --- | --- |
| `internal/agent/tools/download.go:96` | `download` | no `ToolCallID` — bug present |
| `internal/agent/tools/lsp_rename.go:68` | `lsp_rename` | no `ToolCallID` — bug present |
| `internal/agent/tools/lsp_replace_symbol.go:141` | `lsp_replace_symbol` | no `ToolCallID` — bug present |

Every other tool that asks for permission already passes `ToolCallID: call.ID`
— `bash`, `edit`, `multiedit`, `write`, `view`, `ls`, `fetch`, `question`, the
MCP tools, and so on. These three are the outliers.

The same empty ID has a second effect: the chat item for the call never
switches to "awaiting permission" / "running", because the UI looks the item
up by tool call ID.

### The fix

One line per tool — add `ToolCallID: call.ID` to the
`permission.CreatePermissionRequest` literal. That is the whole upstream
change to the three tools.

The upstream test file `internal/agent/tools/download_test.go` is also worth
taking: it runs the download tool against the real permission service with a
hook approval on the context for the same call ID, and asserts the file is
downloaded without a prompt. Before the fix the request blocks and the test
fails with `context deadline exceeded`. The LSP tools need a running language
server before they reach the permission request, so upstream covers them with
the same one-line change rather than a separate test.

### Merge or hand-apply?

The upstream branch is `fix/permission-tool-call-id` and touches exactly the
three tool files plus the new test. Our versions of those files are close to
upstream but not identical (the fork has `multiedit`, `question`, the
semantic-search tools, and a different `download` implementation shape), so
expect small context conflicts in `download.go`.

Recommendation: hand-apply the three one-line changes (they are trivial and
unambiguous), then take the upstream test file as-is and run it. If the test
needs a small adaptation to the fork's `download` constructor signature, note
it in the commit message.

### Effort

S. Three one-line changes plus one test file. Low risk, and it closes a
real hole in the fork's own hook feature (`docs/hooks/README.md` promises the
behaviour).

## Order

1. **#4073** first — it is a real bug in a feature the fork ships, it is
   small, and it does not depend on anything else.
2. **#4090** second — documentation only, no code, no risk.
3. **#4105** — nothing to do; it is already in.

The auto-mode PR ([plan-auto.md](plan-auto.md)) is a separate, much larger
piece of work and does not block any of the three above. Note that it
overlaps #4073: auto mode adds its own `PrePermission` / `PermissionDenied`
hook events and touches the same permission-request paths in the same three
tools, so if both land, #4073 should land first and auto mode should be
re-based on top of it.
