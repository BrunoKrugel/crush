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
| **[Context compaction with checkpoints](/features/sessions)** | Automatic compaction ledgers for long-running sessions, with preview/confirm/discard checkpoints, a restore path, and a recall tool for querying past work. | `gurnben/crush:feat/context-compaction` |
| **[Herdr compat](/features/notifications)** | Overhauled agent state reporting and pane integration for the herdr multiplexer, including compaction state reporting. | upstream PR #3541 |
| **[Lazy MCP loading](/features/mcp)** | MCP tool schemas stay out of the model's context until explicitly pulled with `mcp_search`; per-server `Enabled`/`Disabled`/`Lazy` tri-state settings in the MCP dialog. | upstream PR #3983 |
| **[Skill scoping and caching](/features/skills)** | Deduplicated symlinked skills, skill discovery cached across prompt builds, repo-scoped enable/disable toggles over the global config. | upstream PRs #3830, #3813, plus the skill-toggle upstream work |

## Quality-of-life improvements

| Improvement | Detail | Source |
| --- | --- | --- |
| Provider selection rework | Providers are no longer picked just because their API key sits in the environment; a usable model must be explicitly selected, with stable catalog-order resolution. | upstream PR #3487 |
| Pretty `crush run` output | `--show-events` renders tool calls and results during non-interactive runs. | upstream PR #3440 |
| Nerd Font detection | Terminal nerd-font autodetection (PR #4006), and LSP severity indicators — error, warning, information, hint — render as glyphs with letter fallbacks for plain terminals. | upstream PR #4006 |
| Plan-mode fixes | Raw-markdown copy on selection and table support in plan output. | merged PR #3745, `fix/sort-configured` |

## What this fork does NOT have

Anything not in the table above is plain upstream Crush. In particular there is
no A2UI rendering, no semantic search, no A2A multi-agent dispatch, and no
prompt-side wizardry: if a page describes one of those, it does not apply to
this fork.
