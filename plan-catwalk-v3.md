# Porting plan: upstream PR #3829 — Catwalk v3

Plan for pulling
[`charmbracelet/crush#3829`](https://github.com/charmbracelet/crush/pull/3829)
("feat: Catwalk v3", merged upstream 2026-10-29, 47 files, +1,705/−684)
into [`brunokrugel/crush`](https://github.com/brunokrugel/crush).

The largest port yet by blast radius: it changes what a provider *type*
means, moves per-model fields into nested structures, and reaches into
the fork's most-owned surface — the `crushrc` provider/model builtins,
the user's own `provider add zai --type openai-compat`, and the
discovery enrichers.

## What Catwalk v3 changes

1. **Provider `type` changes meaning.** In v2 it named the SDK/company
   (`openai`, `anthropic`, `bedrock`, `google`, `vercel`, `openrouter`,
   …). In v3 it names the wire protocol:
   - `completions` — OpenAI chat completions
   - `responses` — OpenAI responses API
   - `messages` — Anthropic messages API

2. **Per-model type overrides.** A model can carry its own `type`,
   overriding the provider's. This deletes the hardcoded model-ID
   routing: the copilot `gpt-5.x` responses list and the
   `claude-*`/`minimax-*` messages guessing go away, replaced by
   `model.EffectiveType(providerType)`.

3. **Model JSON reshaped.**
   - `cost_per_1m_in/out` (+ two cached variants) → nested pricing:
     `input`, `output`, `cache_create`, `cache_hit`.
   - `can_reason` + `reasoning_levels` + `default_reasoning_effort` →
     nested `reasoning` object (`thinking` mode: always/never/toggleable,
     effort levels with value + display name, default effort).
   - `supports_attachments` → `capabilities.vision`; plus
     `capabilities.audio`.

4. **New field `max_attachments`** — per-model attachment cap (Anthropic
   100–600, OpenAI 1500, Gemini 10). Audio attachments become a thing;
   the UI enforces the cap.

5. **Config back-compat is in the port**: `legacyProviderTypes` maps old
   type names onto v3 endpoint types at load (`openai-compat` →
   `completions`, `anthropic` → `messages`, …), and `migrateCatwalkV2`
   rewrites v2-shaped model JSON in a merged config to the v3 schema,
   no-op for already-v3 configs. Custom provider types with enrichers
   (ollama, litellm, lmstudio, …) pass through unchanged.

6. **Dependency**: `charm.land/catwalk` pinned to the **v3-preview
   branch** (`v0.52.87-0.20261028124101-d20cc5a55a69`) and pointed at a
   personal `catwalk.andrinoff.com/v3/providers` endpoint in the PR —
   the merged main pins the dependency but serves the released catalog.

## Fork divergence in the touched files

| Fork surface | Divergence | Collision |
| --- | --- | --- |
| `internal/config/load.go` | crushrc layer, the user-level split, `stripProjectAutoMode`, `migrateCatwalkV2` must run over **crushrc-produced JSON too** | **High** — the migration hook lands mid-fork-code |
| `internal/config/config.go` | fork owns `ToProvider` conversion, `cloneForWrite`, its own sections | **High** |
| `internal/shellconfig/model.go` | `model add` writes **v2 JSON keys** (`cost_per_1m_in`, `can_reason`, `supports_attachments`) | **High** — the builtin must translate or grow v3 flags |
| `internal/agent/coordinator.go` | fork's `copilotResponsesModels`, `isOpenCodeMessagesModel`/`ResponsesModel`, post-hooks-port structure (HostContext, notes, HookRunner) | **Medium-high** — the routing tables this PR deletes are fork-carried too |
| `internal/ui/model/ui.go` | fork's text-attachment work sits where upstream adds audio attachments + the cap | **Medium** |
| `internal/message/attachment.go`, `internal/ui/common/common.go` | fork's shared text predicate (`IsTextMIME`) is the attachment choke point audio must join | **Medium** |
| `internal/config/discovery.go`, enrichers | fork carries the host-context-era enrichers plus #4137's llama.cpp strip | **Low** |
| `schema.json` | fork's schema is generated from the fork's config | **Low** — regenerate |

## What the fork must decide (and why)

1. **Type translation is load-bearing for this user.** The user's
   `crushrc` says `provider add zai --type openai-compat`. The
   `legacyProviderTypes` map keeps that working; a test must pin
   `openai-compat` → `completions` translation through the crushrc
   path, not just the JSON path.
2. **The v3-preview pin is a moving target.** The PR pins a preview
   commit. Decide: pin the same preview commit (upstream parity, may
   shift under us), or wait for catwalk to release v3 properly.
   Recommend: pin the same commit as upstream's post-merge go.mod so
   the next sync stays clean, and revisit when catwalk cuts a release.
3. **The crushrc `model add` flags.** Upstream kept the v2 flag
   spellings working. Keep `--price-input`/`--can-reason`/
   `--supports-images` (translated at write time to v3 JSON) and add
   v3-native spellings (`--pricing-input`, `--capabilities-vision`,
   …) only if a flag is genuinely missing. Document in crushrc docs.
4. **Deleted routing tables.** `copilotResponsesModels` and the
   opencode model-ID lists die with per-model types. The fork carries
   copies; deleting them is the point of the migration — the catalog
   now says what each model speaks. Verify against the fork's
   `opencode_routing_test.go`, which upstream rewrote.
5. **Audio attachments.** Upstream adds audio (`capabilities.audio`,
   `AllowedAudioTypes`, `IsAudioPath`). The fork's attachment choke
   point is `IsTextMIME` + the attachments renderer; audio joins as a
   third kind beside image and text.

## The plan

Work in this order, one commit per step, each green.

### Step 1 — dependency + data model

Bump `charm.land/catwalk` to upstream's post-merge pin. Adopt the v3
`catwalk.Model`/`catwalk.Provider` shapes: pricing, `reasoning`
object, `capabilities`, `max_attachments`, per-model `type`, and the
`EffectiveType(providerType)` helper. Expect compile errors across
config/agent/UI — fix mechanically per the steps below. Port
`internal/config/hooks.go`-era discipline: no fork behavior changes in
this commit, only the new data model compiling.

### Step 2 — the v2→v3 translation layer

Port `legacyProviderTypes`, `NormalizeProviderType`,
`legacyModelPricingFields`, `migrateCatwalkV2`, and the persist hook
from upstream's `internal/config/load.go` — but place the call where
**the fork** merges configs (`loadFromConfigPaths`), so crushrc JSON
migrates too. Port upstream's `load_test.go` migration tests and add:
a crushrc-authored provider (`--type openai-compat`, `--price-input`)
round-trips to a v3 config; a project-level config with v2 models
migrates; the user-level/project-level split still applies after
migration.

### Step 3 — config, store, ToProvider

Reconcile `internal/config/config.go` (provider test-URL switch keyed
on the new types; the Gemini endpoint branch) and `ToProvider` with
upstream's version. Port `provider.go`'s one-line change and
`store.go`'s. Reconcile with `cloneForWrite` (the fork's fix that keeps
crushrc providers through copy-on-write).

### Step 4 — crushrc builtins

Port `internal/shellconfig/model.go` and `flags.go`: keep the v2 flag
spellings accepted, writing v3 JSON; add the new flags upstream added.
Port `model_test.go` and `shellconfig_provider_test.go` updates, and
update `docs-site/docs/configuration/crushrc.md`.

### Step 5 — routing tables die

Delete the fork's `copilotResponsesModels`,
`isOpenCodeMessagesModel`/`isOpenCodeResponsesModel`, and the caller
plumbing in `coordinator.go`; adopt upstream's `servingProvider` and
the `Reasoning`-object reads. Port `opencode_routing_test.go` (rewritten
upstream) and `usage_fallback_test.go`. The fork's coordinator carries
HostContext/notes/HookRunner around every touch point — hand-merge.

### Step 6 — discovery enrichers

Mechanical: `litellm.go`, `lmstudio.go`, `discover_test.go`,
`lmstudio_test.go`, `litellm_test.go`, plus the fork's #4137 llama.cpp
strip verified against the new shapes.

### Step 7 — attachments: audio + the cap

Port `message/attachment.go` (max-attachments field), `ui/common`
(`AllowedAudioTypes`, `IsAudioPath`), and the `ui.go` attachment-cap
warning + `Capabilities.Vision`/`Audio` reads — reconciled with the
fork's text-attachment predicate and renderer so audio is a third kind,
not a replacement. Port `ui_test.go` additions.

### Step 8 — UI odds and ends

`ui/dialog/models_item.go` (pricing display), `reasoning.go` (effort
levels with display names), `model/sidebar.go`,
`ui/dialog/commands.go`, and `skills/builtin/crush-config/SKILL.md`
(flag documentation). Regenerate `schema.json`. Update the user's
crushrc if anything migrated.

## Sequencing

Steps 1–3 are one unit (the data model plus its translation) — the
build is red between them, so they may land as a single squashed
commit if review prefers. Steps 4–8 are independent after that.

## Tests

- Upstream's `load_test.go` migration tests (199 added lines) — port
  verbatim.
- crushrc-path migration test (fork-specific, see step 2).
- `zai` regression: the user's actual provider loads with
  `completions` type and the same base URL/key after migration.
- Routing: opencode routing tests rewritten upstream; copilot responses
  models come from the catalog, not the table.
- Attachments: audio acceptance, max-attachments warning, and the
  existing text-attachment suite unchanged.
- The full suites touched before: config, shellconfig, discover,
  agent, ui/model, ui/dialog.

## Open questions

1. **Pin vs release.** Track upstream's v3-preview pin now, or wait for
   catwalk's v3 release? (Recommend: match upstream's pin; revisit on
   the next sync.)
2. **Catalog endpoint.** The PR points at a personal endpoint in the
   description; confirm merged main serves the standard catalog URL so
   the fork needs no endpoint override.
3. **Hyper's provider.json.** Upstream rewrote `hyper/provider.json`
   (573 lines) — carry it verbatim; the fork does not diverge there.
4. **Fork-added models.** The user's crushrc models (`zai/*`,
   `GenPlat/*`) are authored in v2 flags — after step 4 they keep
   loading via translation, but consider rewriting the user's crushrc
   to v3 spellings once stable. (Not this port.)

## Effort

Overall: **L**, but mostly mechanical — the risk concentrates in step 2
(config migration over the crushrc layer) and step 5 (routing tables in
the fork's heavily-extended coordinator). Upstream's test additions
(+199 load_test, +140 ui_test) carry most of the verification.
