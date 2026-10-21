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
| 2 | Conversation forking (depends on #1) | M | Not started |
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
