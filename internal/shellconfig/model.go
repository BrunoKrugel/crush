package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
)

// handleModel implements the `model` builtin.
//
// Usage:
//
//	model add <provider>/<id> [--name NAME] [--context-window N]
//	    [--default-max-tokens N] [--can-reason true|false]
//	    [--thinking always|never|toggleable]
//	    [--supports-images true|false] [--price-input F]
//	    [--price-output F] [--price-cache-create F]
//	    [--price-cache-hit F] [--reasoning-level L]
//	    [--reasoning-effort low|medium|high]
//	    [--max-attachments N]
//
// The pricing, reasoning, and capability flags write the catwalk v3
// nested JSON (pricing.input, capabilities.vision, reasoning.*), and the
// v2 spellings are accepted and translated (see modelV2ToV3Fields).
//
//	model remove <provider>/<id>   (alias: rm)
//	model large [<provider>/<id>] [--think] [--reasoning-effort L]
//	    [--max-tokens N] [--temperature F] [--top-p F] [--top-k N]
//	    [--frequency-penalty F] [--presence-penalty F]
//	    [--provider-options JSON]
//	model small [<provider>/<id>] [...]
//
// "add" registers a model on an existing provider (the provider must have
// been declared with `provider add` first). "remove" removes it. "large" and
// "small" set the selected model for that slot, or print the current
// selection as <provider>/<id> when given no argument.
func handleModel(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: model add|remove <provider>/<id> | model large|small [<provider>/<id>]")
	}

	switch args[1] {
	case "add":
		return modelAdd(b, args, stderr)
	case "remove", "rm":
		return modelRemove(b, args, stderr)
	case "large", "small":
		return modelSelect(b, args, stdout, stderr)
	default:
		return usage(stderr, fmt.Sprintf("model: unknown subcommand %q (expected add, remove, large, or small)", args[1]))
	}
}

// splitProviderModel splits "provider/id" on the first slash. Model ids may
// themselves contain slashes, so only the first separates provider from id.
func splitProviderModel(s string) (provider, id string, ok bool) {
	provider, id, found := strings.Cut(s, "/")
	if !found || provider == "" || id == "" {
		return "", "", false
	}
	return provider, id, true
}

// modelAddFlags is the declarative flag surface for `model add`.
var modelAddFlags = []flagSpec{
	{name: "--name", jsonKey: "name", kind: flagString, op: opSet},
	{name: "--context-window", jsonKey: "context_window", kind: flagInt, op: opSet},
	{name: "--default-max-tokens", jsonKey: "default_max_tokens", kind: flagInt, op: opSet},
	{name: "--thinking", jsonKey: "thinking", child: "reasoning", kind: flagString, op: opSetChild, validate: func(v any) error {
		thinking, ok := v.(string)
		if !ok || !slices.Contains([]string{"always", "never", "toggleable"}, thinking) {
			return fmt.Errorf("--thinking expects always, never, or toggleable, got %v", v)
		}
		return nil
	}},
	{name: "--can-reason", jsonKey: "can_reason", kind: flagBool, op: opSet},
	{name: "--supports-images", jsonKey: "supports_attachments", kind: flagBool, op: opSet},
	{name: "--price-input", jsonKey: "cost_per_1m_in", kind: flagFloat, op: opSet},
	{name: "--price-output", jsonKey: "cost_per_1m_out", kind: flagFloat, op: opSet},
	{name: "--price-cache-create", jsonKey: "cost_per_1m_in_cached", kind: flagFloat, op: opSet},
	{name: "--price-cache-hit", jsonKey: "cost_per_1m_out_cached", kind: flagFloat, op: opSet},
	{name: "--reasoning-effort", jsonKey: "default_reasoning_effort", kind: flagString, op: opSet},
	{name: "--max-attachments", jsonKey: "max_attachments", kind: flagInt, op: opSet},
	// --reasoning-level lists the selectable effort tiers behind one model
	// entry, which is how a family that sells one model per tier collapses
	// into a single picker row.
	{name: "--reasoning-level", jsonKey: "effort_levels", child: "reasoning", kind: flagString, op: opAppendChild},
}

// modelV2ToV3Fields rewrites the v2 model keys the flags write onto the
// catwalk v3 nested schema the config decodes: the cost fields became
// pricing, supports_attachments became capabilities.vision, and the
// can_reason/reasoning_levels/default_reasoning_effort trio became the
// reasoning object.
var modelV2ToV3Fields = map[string]string{
	"cost_per_1m_in":           "pricing.input",
	"cost_per_1m_out":          "pricing.output",
	"cost_per_1m_in_cached":    "pricing.cache_create",
	"cost_per_1m_out_cached":   "pricing.cache_hit",
	"supports_attachments":     "capabilities.vision",
	"can_reason":               "reasoning.thinking",
	"reasoning_levels":         "reasoning.effort_levels",
	"default_reasoning_effort": "reasoning.default_effort_level",
}

func modelAdd(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: model add <provider>/<id> [--name NAME] [--context-window N] [--default-max-tokens N] [--can-reason true|false] [--thinking always|never|toggleable] [--supports-images true|false] [--price-input F] [--price-output F] [--price-cache-create F] [--price-cache-hit F] [--reasoning-level L] [--reasoning-effort low|medium|high] [--max-attachments N]")
	}
	provider, id, ok := splitProviderModel(args[2])
	if !ok {
		return usage(stderr, fmt.Sprintf("model add: expected <provider>/<id>, got %q", args[2]))
	}

	providers := b.section("providers")
	if _, exists := providers[provider]; !exists {
		return usage(stderr, fmt.Sprintf("model add: provider %q does not exist (declare it with `provider add %s` first)", provider, provider))
	}

	model := map[string]any{"id": id}
	if err := applyFlags(modelAddFlags, args, 3, model, "model add", stderr); err != nil {
		return err
	}
	// The v2 keys the flags write are translated onto the catwalk v3
	// nested schema the config decodes. can_reason:true becomes
	// reasoning.thinking "toggleable" unless an explicit thinking mode
	// was set; supports_attachments becomes capabilities.vision.
	for v2Key, v3Key := range modelV2ToV3Fields {
		if v, ok := model[v2Key]; ok {
			delete(model, v2Key)
			if v2Key == "can_reason" {
				// An explicit --thinking wins over the legacy boolean.
				if r, hasReasoning := model["reasoning"].(map[string]any); hasReasoning && r["thinking"] != nil {
					continue
				}
				if b, isBool := v.(bool); isBool && b {
					v = "toggleable"
				} else if isBool {
					v = "never"
				}
			}
			if err := setNested(model, v3Key, v); err != nil {
				return err
			}
		}
	}

	p := childMap(providers, provider)
	// Re-adding a model id replaces the existing entry, matching the
	// update-in-place behavior of `provider add` and `lsp add`.
	modelsArr, _ := p["models"].([]any)
	kept := make([]any, 0, len(modelsArr)+1)
	for _, item := range modelsArr {
		if m, ok := item.(map[string]any); ok && m["id"] == id {
			continue
		}
		kept = append(kept, item)
	}
	p["models"] = append(kept, model)

	slog.Info("Model added in shell config", "provider", provider, "model", id)
	return nil
}

func modelRemove(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: model remove <provider>/<id>")
	}
	provider, id, ok := splitProviderModel(args[2])
	if !ok {
		return usage(stderr, fmt.Sprintf("model remove: expected <provider>/<id>, got %q", args[2]))
	}

	providers := b.section("providers")
	p, exists := providers[provider].(map[string]any)
	if !exists {
		return nil
	}
	modelsArr, _ := p["models"].([]any)
	kept := make([]any, 0, len(modelsArr))
	for _, item := range modelsArr {
		m, ok := item.(map[string]any)
		if ok && m["id"] == id {
			continue
		}
		kept = append(kept, item)
	}
	p["models"] = kept

	slog.Info("Model removed in shell config", "provider", provider, "model", id)
	return nil
}

// modelSelectFlags is the declarative flag surface for `model large`/`small`.
var modelSelectFlags = []flagSpec{
	{name: "--think", jsonKey: "think", kind: flagBoolTrue, op: opSet},
	{name: "--reasoning-effort", jsonKey: "reasoning_effort", kind: flagString, op: opSet},
	{name: "--max-tokens", jsonKey: "max_tokens", kind: flagInt, op: opSet},
	{name: "--temperature", jsonKey: "temperature", kind: flagFloat, op: opSet},
	{name: "--top-p", jsonKey: "top_p", kind: flagFloat, op: opSet, validate: func(v any) error {
		f := v.(float64)
		if f < 0 || f > 1 {
			return fmt.Errorf("--top-p expects a value between 0 and 1, got %v", f)
		}
		return nil
	}},
	{name: "--top-k", jsonKey: "top_k", kind: flagInt, op: opSet},
	{name: "--frequency-penalty", jsonKey: "frequency_penalty", kind: flagFloat, op: opSet},
	{name: "--presence-penalty", jsonKey: "presence_penalty", kind: flagFloat, op: opSet},
	{name: "--provider-options", child: "provider_options", kind: flagJSONObject, op: opMergeChild},
}

func modelSelect(b *ConfigBuilder, args []string, stdout, stderr io.Writer) error {
	slot := args[1]

	// No argument: print the current selection as <provider>/<id>.
	if len(args) == 2 {
		if models, ok := b.root["models"].(map[string]any); ok {
			if sel, ok := models[slot].(map[string]any); ok {
				provider, _ := sel["provider"].(string)
				id, _ := sel["model"].(string)
				if provider != "" && id != "" {
					fmt.Fprintln(stdout, provider+"/"+id)
				}
			}
		}
		return nil
	}

	provider, id, ok := splitProviderModel(args[2])
	if !ok {
		return usage(stderr, fmt.Sprintf("model %s: expected <provider>/<id>, got %q", slot, args[2]))
	}

	sel := childMap(b.section("models"), slot)
	sel["provider"] = provider
	sel["model"] = id

	if err := applyFlags(modelSelectFlags, args, 3, sel, "model "+slot, stderr); err != nil {
		return err
	}

	slog.Info("Model selected in shell config", "slot", slot, "provider", provider, "model", id)
	return nil
}

// setNested sets value at the dotted path within obj, creating nested
// maps as needed.
func setNested(obj map[string]any, path string, value any) error {
	parts := strings.Split(path, ".")
	cur := obj
	for _, part := range parts[:len(parts)-1] {
		next, ok := cur[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[part] = next
		}
		cur = next
	}
	cur[parts[len(parts)-1]] = value
	return nil
}
