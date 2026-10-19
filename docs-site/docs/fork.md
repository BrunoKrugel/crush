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
| **[Multi-agent dispatch](/agents/overview)** | A `dispatch_agent` tool that starts independent agents in the background, each in its own git worktree on a fresh `crush-dispatch-*` branch, while the main agent keeps working. Steer a running agent with a leading `@handle` (the model uses `message_agent`), read its live transcript in inspect mode, cancel it with <kbd>ctrl+x</kbd> or `cancel_dispatch`, and settle its work with `apply_dispatch` (merge, squash or cherry-pick) or `dismiss_dispatch` — Crush never merges on its own. Every agent is an agent definition: override `coder`, `plan`, `task` and `worker` or add your own with the `agent` builtin in `crushrc`. `crush dispatch list` and `prune` manage workspaces from the shell. On by default inside a git repository. | `joestump-agent/crush` |
| **[Semantic search](/features/semantic-search)** | A local vector index over the repo — chunked by symbol, embedded through any OpenAI-compatible endpoint, stored in the project's own SQLite via `sqlite-vec`. Adds the `semantic_search` and `semantic_index` tools and a symbol-extraction package. | `joestump-agent/crush` |
| **[Channel reply routing](/features/channels)** | The channel *push* mechanism is upstream. This fork adds the `channel_enabled` config key, a `channel_reply` tool, server-side routing that delivers each event exactly once per workspace, auto-creation of a session for an inbound event, and replies that go back only to the channel they came from. | `joestump-agent/crush` |
| **[Herdr compat](/features/notifications)** | Overhauled agent state reporting and pane integration for the herdr multiplexer, including compaction state reporting. | upstream PR #3541 |
| **[Lazy MCP loading](/features/mcp)** | MCP tool schemas stay out of the model's context until explicitly pulled with `mcp_search`; per-server tri-state settings in the MCP dialog, plus `list_mcp_prompts` and `call_mcp_prompt` tools and prompts offered in the inline `/` completions. | upstream PR #3983, `brunokrugel/crush` |
| **[Skill scoping and caching](/features/skills)** | Deduplicated symlinked skills, skill discovery cached across prompt builds, repo-scoped enable/disable toggles over the global config. | upstream PRs #3830, #3813, plus the skill-toggle upstream work |
| **Session-scoped prompts** | Permission and question prompts are scoped to the session the client is actually viewing: clients report their current session, pending prompts re-publish when a client switches to the session that raised them, and sub-agent prompts surface on the top-level session. | upstream PRs #4044, `brunokrugel/crush` |
| **Turn and paste UX** | The turn spinner runs from the moment a prompt is sent (not when the first assistant message lands), a setup failure still publishes RunComplete, copied file paths paste as text rather than attaching, and <kbd>ctrl+shift+v</kbd> pastes the clipboard as text on every model. | upstream PRs #4046, #4070, #4071, `brunokrugel/crush` |

## Quality-of-life improvements

Smaller things you notice in the terminal rather than in a config file.

### Chat

| Improvement | Detail | Source |
| --- | --- | --- |
| Click-to-copy | Finished assistant *and* user messages carry a right-aligned `⎘` icon that copies the message. Shown on focus, pinned to the item's right edge. | `joestump-agent/crush` |
| Clickable hyperlinks | A plain click on a link in the transcript opens it in your browser. | `joestump-agent/crush` |
| Channel message styling | Channel-originated messages render with a `sender / via / at` metadata line below the body, so you can tell a webhook from a human. | `joestump-agent/crush` |
| Text copy performance | Faster text copy path in the message renderer. | merged PR #3945 |
| Tool-call elapsed time | Each running tool call shows its own elapsed timer instead of an aggregate one. | merged PR #3600 |

### Composer

| Improvement | Detail | Source |
| --- | --- | --- |
| Syntax-highlighted composer | A vendored textarea with a highlighter, so `@file`, `/skill`, and MCP-prompt tokens are visibly distinct as you type. | `joestump-agent/crush` |
| Atomic token editing | Backspace over a completed token deletes the whole token — and drops its attachment, if it had one — instead of eating one character at a time. | `joestump-agent/crush` |
| Text-file attachments | Attach JSON, YAML, Markdown, and source files, not just images. One shared text predicate everywhere, with binary sniffing to reject the rest. | `joestump-agent/crush` |
| Attachment chips | A clickable remove button on each chip, correct hit zones, and chips that survive sending. | `joestump-agent/crush` |
| `/skill` completions | Skills complete inline from `/`, alongside commands and MCP prompts. | `joestump-agent/crush` |
| Paste as text | A copied file path pastes as text; auto-attach applies only to image paths, and <kbd>ctrl+shift+v</kbd> always pastes the clipboard as text. | `brunokrugel/crush` |

### Dialogs and palette

| Improvement | Detail | Source |
| --- | --- | --- |
| **Toggle Skills dialog** | Palette entry **Toggle Skills** — every discovered skill with its error state, toggled per repository or globally. | merged PR #3994, `brunokrugel/crush` |
| **Toggle MCPs dialog** | Palette entry **Toggle MCPs** — per-server tri-state (enabled / lazy / disabled) for this repository or the global config. | upstream PR #3983, `brunokrugel/crush` |
| **Channels dialog** | Palette entry **Channels** — see and manage active channels. | `joestump-agent/crush` |
| Hierarchical sub-menus | The command palette nests related commands instead of one flat list, with `back`/`backspace` navigation. | `joestump-agent/crush` |
| Model discovery reload | Re-run provider/model discovery from the `/models` dialog without restarting. | `joestump-agent/crush` |

### Sidebar and header

| Improvement | Detail | Source |
| --- | --- | --- |
| Focusable sidebar | Three-state <kbd>tab</kbd> focus, mouse scrolling, a scrollbar instead of a focus border, and irrelevant keybindings hidden while it is focused. | `joestump-agent/crush` |
| Channels column | Active channels shown on the landing page and in the sidebar. | `joestump-agent/crush` |
| MCP sidebar seeding | A periodic re-seed of MCP client states in the TUI, so a server whose only state event fired before the TUI subscribed no longer sticks on "None". | merged PR #3289 |
| `user@host:cwd` header | On by default, mirroring the familiar shell prompt — useful when you have sessions open on several machines. | `joestump-agent/crush` |
| Pills panel | <kbd>ctrl+t</kbd> expands *every* section, and scheduled tasks get their own pill with a stopwatch glyph that distinguishes recurring from one-shot. | `joestump-agent/crush` |

### Configuration

| Improvement | Detail | Source |
| --- | --- | --- |
| `allowed_commands` | Re-allow specific commands the [blocklist](/configuration/permissions#blocked-commands) normally denies, without disabling the blocklist. Still subject to permission approval. | `joestump-agent/crush` |
| `channel_enabled` | Turn an MCP server into a channel from `crush.json`, rather than only via the `--channels` flag. | `joestump-agent/crush` |
| `oauth_client_id_metadata` | OAuth Client ID Metadata Documents (SEP-991): the URL is sent as the `client_id` for servers that advertise `client_id_metadata_document_supported` but have no registration endpoint (Home Assistant). Crush's own document is hosted on this docs site. | `brunokrugel/crush` |
| `embeddings` block | Configure the semantic-search embedding provider from `crush.json` or `crushrc`. | `joestump-agent/crush` |
| Config reload on reconnect | Reconnecting an MCP server re-reads config from disk, so an edit takes effect without a restart. | `joestump-agent/crush` |
| Current time in the prompt | The system prompt carries the current time, which is what makes relative scheduling ("every weekday at 9") work. | `joestump-agent/crush` |

### Performance

| Improvement | Detail | Source |
| --- | --- | --- |
| Sonic JSON codec | Both halves of the JSON wire path run through [`bytedance/sonic`](https://github.com/bytedance/sonic)'s JIT-compiled codec: every `Unmarshal`/`Marshal` call in the tree, plus the HTTP handlers (server, A2A, proto), the CLI's session output, and the streaming message deltas and session-row decodes. `encoding/json` remains only where sonic has no equivalent (`RawMessage`, the `Marshaler`/`Unmarshaler` interfaces sonic dispatches through, and streaming decode). | `brunokrugel/crush` |

## Fixes this fork carries

Fixes to **upstream** behaviour that live here and are not upstream. Fixes to
the fork's own features are part of those features and are not listed
separately.

| Fix | Symptom it removes |
| --- | --- |
| Bounded MCP init waits | A wedged MCP server could blank the whole app at startup. Tool lists now wait for init, with a bound. |
| Per-server MCP lifecycle serialized | Concurrent connect/disconnect on one server raced; an error on one session tore down others. |
| stdio process groups reaped | Killed stdio MCP servers left orphan processes behind. |
| MCP OAuth persistence | Tokens did not actually survive a restart; the store now does atomic writes and validates state. |
| In-process run dispatch serialized | Two turns could run concurrently in one session. |
| Provider base URL resolved before validation | A custom provider with an expanded `base_url` failed validation that should have passed. |
| Empty paste routes to the clipboard image | An empty bracketed paste dropped the image instead of attaching it — and now warns instead of silently failing on a model that cannot take images. |
| Command filter matches shortcuts | Typing a command's shortcut in the `/` menu filtered it out. |
| Tied project timestamps ordered by recency | Projects registered in the same instant sorted arbitrarily. |
| Backspace in the commands filter | Backspace was swallowed in the top-level commands filter. |
| Sub-agent permissions | Non-interactive permission propagation applied only to the main agent, so a subagent's runs prompted from scratch. | (upstream PR #3365 is merged upstream; the fork's port predates it) |
| Sentry known-sessionless | `https://mcp.sentry.dev/mcp` ships pre-registered in `knownSessionlessMCPs`; known sessionless servers skip the subscriptions stream that stalls them. | (merged PR #4038) |

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
