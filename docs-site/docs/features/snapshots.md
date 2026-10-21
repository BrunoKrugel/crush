---
id: snapshots
title: Snapshots
sidebar_position: 12
description: Git-backed filesystem checkpoints captured at every user message — restore any point, diff any pair.
---

# Snapshots

:::info[Fork feature]
Snapshots are an addition in this fork. Neither the `snapshots` config block
nor the `internal/snapshot` package exist upstream. See
[What this fork adds](/fork).
:::

At every top-level user message, Crush captures the state of the project
directory into a **private git object store** and ties it to the session and
message that produced it. You can list the checkpoints of a session, diff any
two, and restore the filesystem to any of them.

It is the filesystem counterpart of the conversation
[checkpoints](/features/sessions) used by context compaction: compaction
checkpoints preserve the *conversation*, snapshots preserve the *files*.

## How it works

- The store lives under `<project>/.crush/git` — a real git object database,
  but fully private. Your project's own `.git` is never read or written by
  the snapshot system.
- Every capture walks the working tree, skipping `.git`, `.crush`, the
  project's `.gitignore` rules, and the configured exclusions. Content is
  stored content-addressed, so an unchanged file costs nothing on the next
  snapshot — when a snapshot's tree is identical to the previous one, the
  commit is reused entirely and no new objects are written.
- Each snapshot is pinned with a `refs/snapshots/{session}/{message}` ref,
  so a session's history is addressable per message.
- Capture happens in the background after the user message is persisted;
  a slow or failed snapshot never delays or breaks the turn.
- Snapshots are taken only in git repositories (the home directory is
  explicitly excluded) and only for top-level sessions — sub-agents and
  steering messages never trigger one.

## Restoring

Restoring a snapshot writes its tracked files back into the working tree.
Files that were never part of the snapshot are left untouched, so untracked
work always survives a restore.

## Configuration

```json
{
  "snapshots": {
    "enabled": true,
    "exclude": ["node_modules", "vendor", "dist", "build"]
  }
}
```

| Key | Default | Does |
| --- | --- | --- |
| `enabled` | `true` | Capture a snapshot at each user message. `false` disables the feature entirely. |
| `exclude` | `node_modules`, `vendor`, `dist`, `build` | Glob or prefix patterns skipped during the snapshot walk (on top of `.gitignore` and `.crush`). |

Or from a `crushrc` file:

```bash
option snapshots-enabled true
option snapshots-exclude dist
option snapshots-exclude target
```

## Storage

Snapshots live in the project database (`snapshots` table) alongside the
git object store. Each row chains to the session's previous snapshot, so
`parent_snapshot_id` forms a per-session history. Session deletion removes
the rows; pruning of old git refs is available on the store (`PruneKeep`).
