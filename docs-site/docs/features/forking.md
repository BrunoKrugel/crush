---
id: forking
title: Forking
sidebar_position: 14
description: Branch a conversation from any point into a new session, with the filesystem state that went with it.
---

# Forking

:::info[Fork feature]
Forking is an addition in this fork, reimplemented from the
[`taigrr/crush`](https://github.com/taigrr/crush) design (their
`docs/specs/WORKTREES_AND_SNAPSHOTS.md` §7). Nothing about it exists
upstream. See [What this fork adds](/fork).
:::

Sometimes a conversation goes one way and you want to try another without
losing the first. Forking branches it: the new session continues the old one
up to the point you pick, and the original session stays exactly as it was.

Forking is the conversation counterpart of
[snapshots](/features/snapshots). Snapshots preserve the *files* at a point
in a session; a fork preserves the *conversation* at that same point, and can
bring the files along with it.

## How it works

- **The fork point is a message.** The new session keeps the history up to
  and including that message, and nothing after it.
- **Hidden and steer messages do not travel.** They are harness
  scaffolding — generated continuations and steering injections — not
  something you said, so a fork leaves them behind.
- **The original session is untouched.** A fork is a new session that points
  back at its source; the source keeps its own messages, its own snapshots,
  and its own title.
- **The lineage is recorded.** The forked session carries the session, the
  message, and the snapshot it was taken from, so the UI can show where it
  came from.

## Files come along when snapshots are on

With [snapshots](/features/snapshots) enabled, the fork point usually has a
snapshot: the working tree as it was when that message was sent. The fork
can restore it, so the new session starts from the same code the old one did.

- **In place** — the files are written back into the current project
  directory.
- **In a fresh worktree** — a detached git worktree is created under
  `.crush/forks/` and the snapshot is materialized there, so the original
  directory stays as it is. The worktrees directory is kept out of git.

With snapshots off, a fork is a conversation fork only: the history is
branched, the files are not.

## How to use it

1. Open the command palette (<kbd>ctrl+p</kbd>) and pick **Fork
   Conversation**. The fork point defaults to your last prompt.
2. Give the fork a name, or leave it empty to derive one from the source
   session.
3. Press <kbd>tab</kbd> to ask for a fresh worktree instead of restoring
   the files in place.
4. Press <kbd>enter</kbd>. Crush creates the forked session and switches to
   it.

## Limits

- Forking runs in the process that owns the session database. A client
  attached to a remote server cannot fork; run the fork from the server's
  own TUI.
- A fork needs a message to fork from. In a session with no prompt yet,
  there is nothing to branch.
