package config

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"

	"charm.land/catwalk/v3/pkg/catwalk"
)

// legacyModelPricingFields maps catwalk v2 model cost fields onto their
// v3 pricing counterparts.
var legacyModelPricingFields = map[string]string{
	"cost_per_1m_in":         "input",
	"cost_per_1m_out":        "output",
	"cost_per_1m_in_cached":  "cache_create",
	"cost_per_1m_out_cached": "cache_hit",
}

// migrateCatwalkV2 rewrites a merged user config from the catwalk v2
// schema onto the v3 schema so pre-migration configs decode correctly.
// In v3, flat model fields moved into nested structures: the cost
// fields became pricing, supports_attachments became
// capabilities.vision, and the can_reason/reasoning_levels/
// default_reasoning_effort trio became the reasoning object. The
// rewrite is pure and is a no-op for configs already written with v3
// fields; use migrateCatwalkV2File to persist the result.
func migrateCatwalkV2(data []byte) ([]byte, bool) {
	var root map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&root); err != nil {
		return data, false
	}
	providers, ok := root["providers"].(map[string]any)
	if !ok {
		return data, false
	}

	changed := false
	for _, raw := range providers {
		provider, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"models", "chatgpt_models"} {
			models, ok := provider[key].([]any)
			if !ok {
				continue
			}
			for i, rawModel := range models {
				model, ok := rawModel.(map[string]any)
				if !ok {
					continue
				}
				if migrateCatwalkV2Model(model) {
					models[i] = model
					changed = true
				}
			}
		}
	}
	if !changed {
		return data, false
	}

	out, err := json.Marshal(root)
	if err != nil {
		return data, false
	}
	return out, true
}

// migrateCatwalkV2File persists the migrated form of data at path and
// returns the bytes to load: the migrated form when the rewrite
// changed anything, data otherwise. The returned bytes are always the
// migrated form, so loading continues even when the write fails.
func migrateCatwalkV2File(path string, data []byte) []byte {
	migrated, changed := migrateCatwalkV2(data)
	if !changed {
		return data
	}
	perm := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}
	if err := os.WriteFile(path, migrated, perm); err != nil {
		slog.Warn("Failed to write migrated config", "path", path, "error", err)
	} else {
		slog.Info("Migrated config file to the catwalk v3 model schema", "path", path)
	}
	return migrated
}

// migrateCatwalkV2Model rewrites one model entry from the v2 flat
// fields onto the v3 nested schema, in place. Reports whether anything
// changed.
func migrateCatwalkV2Model(model map[string]any) bool {
	changed := false

	if _, ok := model["pricing"]; !ok {
		pricing := map[string]any{}
		for legacy, field := range legacyModelPricingFields {
			value, ok := model[legacy]
			if !ok {
				continue
			}
			pricing[field] = value
			delete(model, legacy)
			changed = true
		}
		if len(pricing) > 0 {
			model["pricing"] = pricing
		}
	} else {
		for legacy := range legacyModelPricingFields {
			if _, ok := model[legacy]; ok {
				delete(model, legacy)
				changed = true
			}
		}
	}

	if value, ok := model["supports_attachments"]; ok {
		delete(model, "supports_attachments")
		capabilities, ok := model["capabilities"].(map[string]any)
		if !ok {
			capabilities = map[string]any{}
			model["capabilities"] = capabilities
		}
		if _, ok := capabilities["vision"]; !ok {
			capabilities["vision"] = value
		}
		changed = true
	}

	canReason, hasCanReason := model["can_reason"]
	levels, hasLevels := model["reasoning_levels"]
	defaultEffort, hasDefaultEffort := model["default_reasoning_effort"]
	for _, legacy := range []string{"can_reason", "reasoning_levels", "default_reasoning_effort"} {
		if _, ok := model[legacy]; ok {
			delete(model, legacy)
			changed = true
		}
	}
	if _, ok := model["reasoning"]; !ok && (hasCanReason || hasLevels || hasDefaultEffort) {
		reasoning := map[string]any{}
		if can, ok := canReason.(bool); ok && !can {
			reasoning["thinking"] = catwalk.ThinkingNever
		} else {
			reasoning["thinking"] = catwalk.ThinkingToggleable
			if raw, ok := levels.([]any); ok && len(raw) > 0 {
				values := make([]string, 0, len(raw))
				for _, level := range raw {
					if value, ok := level.(string); ok {
						values = append(values, value)
					}
				}
				if len(values) > 0 {
					effortLevels := make([]any, 0, len(values))
					for _, level := range catwalk.NewEffortLevels(values...) {
						effortLevels = append(effortLevels, map[string]any{
							"value":   level.Value,
							"display": level.Display,
						})
					}
					reasoning["effort_levels"] = effortLevels
					if effort, ok := defaultEffort.(string); ok && effort != "" {
						reasoning["default_effort_level"] = effort
					}
				}
			}
		}
		model["reasoning"] = reasoning
	}

	return changed
}
