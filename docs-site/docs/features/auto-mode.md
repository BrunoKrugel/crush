---
id: auto-mode
title: Auto mode
sidebar_position: 16
description: An opt-in safety classifier that grants, denies, or escalates tool permissions before the prompt.
---

# Auto mode

:::info[Fork feature]
Auto mode is a port of
[`gurnben/crush`](https://github.com/gurnben/crush) (upstream PR #3874) onto
this fork. See [What this fork adds](/fork).
:::

Yolo mode skips every prompt. The normal flow asks for every one. Auto mode
sits between them: a classifier looks at a tool call that would otherwise
prompt, and grants it, denies it with a reason, or escalates it back to you.

It is off unless you turn it on, and a repository cannot turn it on for you.
The loader ignores an `auto_mode` section in a project config. Put it in your
user `crushrc`.

## What it decides

Auto mode runs only when a request is about to prompt. A hard deny, an
allow-list hit, and a `PreToolUse` decision still win. The classifier then
returns one of three verdicts:

- **Allow.** The tool runs. You are not asked.
- **Deny.** The tool does not run. The model sees why, so it can try something
  else instead of repeating the call.
- **Escalate.** You get the normal permission prompt.

A classifier error escalates. It does not allow. `fail-open true` is the
escape hatch, and it defaults to false.

Static rules run before the model. With no classifier selected, auto mode is
rules-only: unambiguous calls are granted or denied, and everything else
escalates. That is a valid configuration, not a degraded one.

## Quotas

Denials are counted per session. After 3 consecutive denials, or 20 denials
in total, auto mode pauses and later calls escalate until you grant one.
A grant resets the consecutive counter. Both limits are configurable.

## Cost

Each ambiguous call can spend one classifier completion, and a second if the
fast filter is unsure. Pick a small local model. The classifier is not your
coding model, and it does not see the coding model's reasoning.

## Configure it

```bash
auto_mode on
auto_mode model local-llama/gemma4-e4b
auto_mode max-tokens 1024
auto_mode timeout 60
auto_mode fail-open false
auto_mode environment "Trusted repo: this git remote. Trusted domain: example.com"
auto_mode max-consecutive-denials 3
auto_mode max-total-denials 20
auto_mode transcript-max-chars 24000
```

`auto_mode model` takes `<provider>/<id>` of a provider already in your
config. To point at an OpenAI-compatible endpoint directly:

```bash
auto_mode base-url http://127.0.0.1:8080/v1
auto_mode api-key "$AUTOMODE_API_KEY"
```

Prompt overrides are Go templates. Placeholders are `{{cwd}}`, `{{tool}}`,
`{{params}}`, `{{environment}}`, and `{{transcript}}`.

```bash
auto_mode prompt stage1 ./prompts/stage1.tmpl
auto_mode prompt stage2 ./prompts/stage2.tmpl
```

`auto_mode off` disables it. Omitting the value of a boolean key sets it to
true.

The JSON form is the same object, under `auto_mode`, and only in a user-level
config:

```json
{
  "auto_mode": {
    "enabled": true,
    "classifier": { "provider": "local-llama", "model": "gemma4-e4b" },
    "max_tokens": 1024,
    "timeout_seconds": 60,
    "fail_open": false
  }
}
```

## Not yolo

Auto mode never skips the prompt for a call it cannot classify. Yolo does.
Turning one on turns the other off. A dispatched agent follows the live
setting the same way it follows yolo.

Shell hooks can still veto. `PrePermission` runs in the same slot and can
allow or deny before the classifier's verdict is the last word. See
[Hooks](/features/hooks#prepermission).
