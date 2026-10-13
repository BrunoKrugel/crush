---
id: fork
title: What this fork adds
sidebar_label: What this fork adds
sidebar_position: 2
description: Every feature, improvement, and fix that brunokrugel/crush carries on top of charmbracelet/crush — via merged upstream pull requests and fork branches.
---

# What this fork adds

These docs cover
**[`brunokrugel/crush`](https://github.com/brunokrugel/crush)**, a fork that
tracks [`charmbracelet/crush`](https://github.com/charmbracelet/crush) upstream
and carries a curated set of changes on top: upstream pull requests that are
still open there, plus feature branches from other forks. This page is the
inventory of what it adds beyond upstream `main`.

:::info[How this fork is built]
None of the additions below are bespoke. They are merges of upstream pull
requests (numbered as such) and of published feature branches, each pullable
into the `fork` branch with two commands. When upstream eventually merges its
own copy of these changes, the duplicate history drops out cleanly.
:::

## Major features

| Feature | What it is | Source |
| --- | --- | --- |
| **[Scheduled tasks](/features/scheduled-tasks)** | `CronCreate`, `CronList`, and `CronDelete` tools that re-run a prompt on a cron schedule, with a scheduler, one-shot support, and a command-palette entry showing what is queued. | upstream PR #3466 |
| **[Context compaction with checkpoints](/features/sessions)** | Automatic compaction ledgers for long-running sessions, with preview/accept/discard checkpoints, an undo path, and inline checkpoint handoff in the TUI; recorded session memory rides beside every summary instead of replacing it, and the checkpoint accepts an emphasis instruction. | `gurnben/crush:feat/context-compaction`, upstream PR #4059 (28 commits) |
| **[Multi-agent dispatch](/agents/overview)** | A `dispatch_agent` tool that starts independent agents in the background, each in its own git worktree on a fresh `crush-dispatch-*` branch, while the main agent keeps working. Steer a running agent with a leading `@handle` (the model uses `message_agent`), read its live transcript in inspect mode, cancel it with <kbd>ctrl+x</kbd> or `cancel_dispatch`, and settle its work with `apply_dispatch` (merge, squash or cherry-pick) or `dismiss_dispatch` — Crush never merges on its own. Every agent is an agent definition: override `coder`, `plan`, `task` and `worker` or add your own with the `agent` builtin in `crushrc`. `crush dispatch list` and `prune` manage workspaces from the shell. On by default inside a git repository. | `brunokrugel/crush` |
| **[Semantic search](/features/semantic-search)** | A local vector index over the repo — chunked by symbol, embedded through any OpenAI-compatible endpoint, stored in the project's own SQLite via `sqlite-vec`. Adds the `semantic_search` and `semantic_index` tools and a symbol-extraction package. | `brunokrugel/crush` |
| **[Channel reply routing](/features/channels)** | The channel *push* mechanism is upstream. This fork adds the `channel_enabled` config key, a `channel_reply` tool, server-side routing that delivers each event exactly once per workspace, auto-creation of a session for an inbound event, and replies that go back only to the channel they came from. | `brunokrugel/crush` |
| **[Herdr compat](/features/notifications)** | Overhauled agent state reporting and pane integration for the herdr multiplexer, including compaction state reporting. | upstream PR #3541 |
| **[Lazy MCP loading](/features/mcp)** | MCP tool schemas stay out of the model's context until explicitly pulled with `mcp_search`; per-server tri-state settings in the MCP dialog, plus `list_mcp_prompts` and `call_mcp_prompt` tools and prompts offered in the inline `/` completions. | upstream PR #3983, `brunokrugel/crush` |
| **[Skill scoping and caching](/features/skills)** | Deduplicated symlinked skills, skill discovery cached across prompt builds, repo-scoped enable/disable toggles over the global config. | upstream PRs #3830, #3813, plus the skill-toggle upstream work |

## Quality-of-life improvements

| Improvement | Detail | Source |
| --- | --- | --- |
| Provider selection rework | Providers are no longer picked just because their API key sits in the environment; a usable model must be explicitly selected, with stable catalog-order resolution. | upstream PR #3487 |
| Pretty `crush run` output | `--show-events` renders tool calls and results during non-interactive runs. | upstream PR #3440 |
| Nerd Font detection | Terminal nerd-font autodetection (PR #4006), and LSP severity indicators — error, warning, information, hint — render as glyphs with letter fallbacks for plain terminals. | upstream PR #4006 |
| Plan-mode fixes | Raw-markdown copy on selection and table support in plan output. | merged PR #3745, `fix/sort-configured` |
| MCP sidebar seeding | A periodic re-seed of MCP client states in the TUI, so a server whose only state event fired before the TUI subscribed no longer sticks on "None". | merged PR #3289 |
| Subagent permissions | Non-interactive permission propagation applies to subagents too. | merged PR #3365 |
| Sentry known-sessionless | `https://mcp.sentry.dev/mcp` ships pre-registered in `knownSessionlessMCPs`; known sessionless servers skip the subscriptions stream that stalls them. | merged PR #4038 |
| Skill toggles | Repo-scoped skill enable/disable dialogs, on top of the upstream toggle work this fork already carried. | merged PR #3994 |
| Scroll speed | Configurable TUI scroll speed (`tui.scroll_speed`). | merged PR #3946 |
| Text copy performance | Faster text copy path in the message renderer. | merged PR #3945 |
| Tool-call elapsed time | Each running tool call shows its own elapsed timer instead of an aggregate one. | merged PR #3600 |
| `@handle` completions and routing | `@` completes live dispatched agents — role, status, current todo — alongside files. A prompt that *starts* with `@handle` steers that agent instead of going to the main agent; a mid-sentence `@handle` attaches the agent's status card. | `brunokrugel/crush` |
| Inspect mode | <kbd>ctrl+]</kbd> on an agent block opens that sub-agent's full transcript in the chat window, following it live; <kbd>ctrl+]</kbd> again cycles live agents and <kbd>esc</kbd> or <kbd>ctrl+[</kbd> returns with the scroll position restored. The parent stays the active session. | `brunokrugel/crush` |
| Sub-agent session tree | The session picker shows a `▸N` count on sessions that started sub-agents; <kbd>ctrl+]</kbd> lists them, and each opens read-only in inspect mode. | `brunokrugel/crush` |
| Click-to-copy | Finished assistant *and* user messages carry a right-aligned `⎘` icon that copies the message. Shown on focus, pinned to the item's right edge. | `brunokrugel/crush` |
| Text-file attachments | Attach JSON, YAML, Markdown, and source files, not just images, with chips that survive sending and a clickable remove button. | `brunokrugel/crush` |
| `user@host:cwd` header | On by default, mirroring the familiar shell prompt — useful when you have sessions open on several machines. | `brunokrugel/crush` |
| `allowed_commands` | Re-allow specific commands the blocklist normally denies, without disabling the blocklist. Still subject to permission approval. | `brunokrugel/crush` |
| Current time in the prompt | The system prompt carries the current time, which is what makes relative scheduling ("every weekday at 9") work. | `brunokrugel/crush` |
| Channels dialog and sidebar column | Palette entry **Channels** to see and manage active channels, and a sidebar column showing them. | `brunokrugel/crush` |
| In-process run dispatch serialized | Two turns could run concurrently in one session. | `brunokrugel/crush` |
| Bounded MCP init waits | A wedged MCP server could blank the whole app at startup. Tool lists wait for init, with a bound. | `brunokrugel/crush` |
| Per-server MCP lifecycle serialized, stdio process groups reaped, MCP OAuth persistence | Concurrent connect/disconnect races, orphaned stdio servers, and OAuth tokens that did not survive a restart. | `brunokrugel/crush` |

## Proposed but not yet merged

| PR | What it would add |
| --- | --- |
| upstream PR #2584 | Let the user configure the agent's model size (large/small flexing per agent definition). Open and conflicted against upstream; not carried here yet. |

## What this fork does NOT have

Anything not in the table above is plain upstream Crush or one of the merged
branches named above. In particular there is **no A2UI rendering**: the A2UI
compile-time plumbing from `brunokrugel/crush` is present only to keep the
tree building, with no feature wiring — no `a2tea` integration, no cards,
lists, buttons, forms, or dashboards drawn inline, and interactive surfaces do
not round-trip to the model. There is also no prompt-side wizardry. If a page
in these docs describes one of those, it does not apply to this fork.
