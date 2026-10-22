# Porting plan: taigrr/crush features into this fork

Tracks what we take from [`taigrr/crush`](https://github.com/taigrr/crush)
(see their `FEATURES.md` / `OUR_CHANGES.md`) into
[`brunokrugel/crush`](https://github.com/brunokrugel/crush).

Approach: their fork renamed modules and gutted provider/telemetry code, so
commits do not cherry-pick cleanly. We **reimplement** on our base, using
their `docs/specs/` as design references. Their self-contained packages
(`checkpoint`, `worktree`, `fork`) can be ported with import fixes;
everything touching `workspace`/`server`/TUI is a rewrite.

## Status

| # | Feature | Effort | Status |
| --- | --- | --- | --- |
| 1 | Git-backed filesystem snapshots | L | **Done** (2026-10-21) |
| 2 | Conversation forking (depends on #1) | M | **Done** (2026-10-21) |
| 3 | Swarm — cross-session messaging | M | Not started |
| 4 | Slash commands + `/export` | S | Not started |
| 5 | Goal mode (`/goal` evaluator loop) | M | Not started |
| 6 | New tools: `multi_view`, LSP tools, `context7` | S–M | Not started |
| 7 | Procedures (then milestones) | S–M | Not started |
| 8 | Dynamic 1M context modes | S | Not started |
| 9 | User-facing worktrees (switch/merge, per-session cwd) | L | Backlog — after #1; reconcile with dispatch worktrees |
| 10 | Session archive + navigator sidebar | M–L | Backlog |
| 11 | Session history search (embeddings) | M | Backlog — reuse our `embeddings` config + vec0 |
| 12 | Lua theming, KGP images, sysadmin mode, review flow, neovim bridge | — | Backlog / opportunistic |

Skipped deliberately: telemetry removal, embedded-only providers, de-charm,
their client-server DB sync protocol (research project), their multi-client
plumbing (our server/workspaces work diverged), Bedrock forks (we track
upstream catwalk).

## Done: #1 git-backed filesystem snapshots

Implemented 2026-10-21, reimplemented from the taigrr design (their
`internal/checkpoint` + specs), not merged.

- **`internal/snapshot/`** — engine: private git object store at
  `.crush/git` (never touches the user's `.git`), content-addressed blob
  reuse (unchanged tree reuses the previous commit object), git-canonical
  tree sort, `refs/snapshots/{session}/{message}` refs, restore of tracked
  files only, unified diff, `PruneKeep`, `.gitignore` + exclusion support,
  `IsSupported` guard (home dir / non-git are skipped).
- **`internal/db`** — `snapshots` table migration (FK to sessions, index),
  queries, hand-written sqlc-style code (run `task sqlc` to verify).
- **Config** — top-level `snapshots` block in `crush.json`
  (`enabled`, default true; `exclude`, default
  `node_modules/vendor/dist/build`) and `option snapshots-enabled` /
  `option snapshots-exclude` in `crushrc`.
- **Wiring** — app builds the service when enabled and supported;
  coordinator passes it to session agents; each top-level user message
  triggers a background capture (steers, hidden messages, and sub-agents
  excluded; failures warn-only).
- **Tests** — `internal/snapshot` (9 tests: round-trip, restore,
  exclusions, unchanged-tree reuse, diff, prune, service, DB store),
  `internal/db` CRUD, config parsing, crushrc keys.
- **Docs** — `docs-site/docs/features/snapshots.md`, fork.md entry,
  crushrc.md configuration section.

Follow-ups: TUI/CLI surface (list/restore/diff dialog or commands), ref
pruning policy, snapshot-on-restore-before-restore safety capture.

## Planned: #2 conversation forking

Yes, we can do it: the dependency (#1 snapshots) is done, and our snapshot
engine makes the taigrr flow simpler here. Design, reconciled with our
fork's own surface:

**What forking does.** Fork a conversation from any message: a new session
is created that shares the history up to that point, with a filesystem
state restored from the snapshot captured at (or created for) that message,
optionally in a fresh worktree. The original session is untouched.

**Differences from taigrr's design that fall out of our base.**

- Their design keys the fork on a snapshot id and a `worktrees` table.
  We do not need their worktrees table: the multi-agent dispatch
  infrastructure already owns managed worktrees, and item #9 will
  reconcile user-facing worktrees with it. For #2 the worktree toggle
  creates a plain `git worktree add` at the current HEAD, then runs our
  snapshot restore into it, so the fork's file state matches the fork
  point without importing their schema.
- Our snapshot store is private (`.crush/git`, never touches the user's
  `.git`), so a worktree cannot check out our snapshot commit directly.
  The restore-into-fresh-worktree approach above is the workaround.
- Compaction checkpoints and steer/hidden messages: fork copies only
  user+assistant messages up to the fork point and marks `steer` and
  hidden messages out, so the new session never replays harness
  scaffolding.

**Steps.**

1. **DB** — migration adding `forked_from_snapshot_id` and
   `forked_from_message_id` (both nullable, FK to snapshots/messages)
   to `sessions`; new queries (`Fork` reads: source session with
   messages up to a message id, create target with copied messages).
2. **`internal/fork/`** — small package: `Fork(ctx, source sessionID,
   messageID, opts)` doing: ensure a snapshot exists for the message
   (capture on demand through `internal/snapshot`), create the session
   via `session.Service` with the fork pointers, copy messages, and
   optionally restore files (current dir or a fresh worktree). Returns
   the new session.
3. **Session service** — expose `CopyMessagesUpTo` (or accept it inside
   `internal/fork`) reusing existing message CRUD.
4. **UI** — fork action in the chat view's per-message menu (user
   messages), a `fork.go` dialog (new session name + worktree toggle),
   switch to the new session after forking, and a fork indicator in the
   header. Reuse the compaction-checkpoint dialog styling.
5. **Server/API (optional phase)** — `POST /sessions/{sid}/fork` for
   remote clients, mirroring the A2A session surface.
6. **Tests** — fork round-trip (messages copied, none lost, order kept),
   snapshot on demand, restore behavior, steer/hidden exclusion, DB
   migration. Follow repo TDD rules (RED first under `internal/`).
7. **Docs** — `docs-site/docs/features/forking.md`, fork.md row,
   sidebars entry.

**Estimated effort**: M, roughly 1–1.5 days with dispatched agents
(DB+package in one worktree, UI in another; docs parallel). Reuses
#1's snapshot capture and restore paths so no new git plumbing is
needed.

## Done: #2 conversation forking

Implemented 2026-10-21, reimplemented from the taigrr design (their
`docs/specs/WORKTREES_AND_SNAPSHOTS.md` §7).

- **DB** — migration adding `forked_from_session_id`,
  `forked_from_snapshot_id` and `forked_from_message_id` to `sessions`;
  `internal/db/sql/fork.sql` with the message copy and the origin
  update. A copied message gets a fresh id: `messages.id` is the table's
  primary key, and the source session keeps its own copy.
- **`internal/fork/`** — the service: resolve the fork point, copy the
  history up to it, record the origin, and (with snapshots on) capture
  the fork point on demand and restore it — in place, or into a fresh
  detached worktree under `.crush/forks`.
- **`internal/snapshot`** — `RestoreTo` writes a snapshot into a target
  root, which is how a fork materializes its files in a worktree.
- **Session model** — the fork lineage is carried on the session.
- **UI** — a **Fork Conversation** palette entry, a dialog to name the
  fork and ask for a worktree, and a switch to the new session when the
  fork succeeds. The fork point defaults to the last prompt.
- **Wiring** — the snapshot service is now built once and shared by the
  agent and the fork service; `Workspace.ForkSession` serves the local
  workspace and reports honestly that a remote client cannot fork.
- **Tests** — `internal/fork` (copy boundary, message from another
  session, hidden/steer exclusion, origin, missing snapshot service,
  derived title) and `internal/ui/model` (fork point selection, fork
  request arguments).
- **Docs** — `docs-site/docs/features/forking.md`, fork.md entry,
  sidebar entry.

Follow-ups: a per-message fork action in the chat view (the palette
entry forks at the last prompt for now), a server endpoint so a remote
client can fork, and the worktree reconcile that item #9 will settle
together with the dispatch worktrees.

## Planned: #5 goal mode (the /goal evaluator loop)

An autonomous, turn-budgeted loop: after each turn a cheap small-model
evaluator asks whether the stated goal is met; while it is unmet and the
turn budget (default 25, hard-capped) is not exhausted, a fresh
continuation directive is injected and another turn runs. Design
reconciled with our nudge ladder (the fork's own enforcement machinery):

- **One ladder, one new rung.** The watchdogs (hard_timeout,
  inactivity_timeout, stall_window) stay authoritative and goal-agnostic.
  Goal mode adds a *progress signal*, not a second enforcement layer.
- **One injection channel.** The goal continuation supersedes the todo
  nudge for the same turn, folding the nudge content in ("and refresh
  your todo list"). A single per-turn decision: goal continuation, else
  nudge, else nothing.
- **Goal state rides on the session**: goal text, turn budget, evaluator
  model hint (small), and state (active / met / exhausted) — the same
  way the ladder already reads session state.
- **kill_after_nudges maps directly**: a run that ends turns ignoring N
  continuations (claiming done while the evaluator disagrees) trips the
  same kill rung; for dispatched agents that is the existing re-dispatch
  contract with the evaluator's verdict as the reason.
- **Hook points are the ladder's own**: the fantasy loop's streaming
  callbacks (OnToolCall, PrepareStep). No parallel loop in Run.
- **Deliberate alternative**: goal-seeded dispatches (a /goal that starts
  a worker) inherit A2A records, cards, inspect mode and every watchdog
  for free, but move the work into a child session. Reconsidered when
  #3 (swarm) lands; not this pass.
