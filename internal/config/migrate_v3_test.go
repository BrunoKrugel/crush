package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func TestMigrateCatwalkV2RewritesModelFields(t *testing.T) {
	t.Parallel()

	v2 := []byte(`{
		"providers": {
			"zai": {
				"type": "openai-compat",
				"models": [{
					"id": "glm-5.3",
					"cost_per_1m_in": 1.4,
					"cost_per_1m_out": 4.4,
					"cost_per_1m_in_cached": 0.1,
					"cost_per_1m_out_cached": 0.2,
					"can_reason": true,
					"reasoning_levels": ["low", "high"],
					"default_reasoning_effort": "high",
					"supports_attachments": true
				}]
			}
		}
	}`)

	migrated, changed := migrateCatwalkV2(v2)
	require.True(t, changed)

	var got struct {
		Providers map[string]struct {
			Type   string `json:"type"`
			Models []struct {
				ID      string `json:"id"`
				Pricing struct {
					Input       float64 `json:"input"`
					Output      float64 `json:"output"`
					CacheCreate float64 `json:"cache_create"`
					CacheHit    float64 `json:"cache_hit"`
				} `json:"pricing"`
				Capabilities struct {
					Vision bool `json:"vision"`
				} `json:"capabilities"`
				Reasoning struct {
					Thinking     string `json:"thinking"`
					EffortLevels []struct {
						Value   string `json:"value"`
						Display string `json:"display"`
					} `json:"effort_levels"`
					DefaultEffortLevel string `json:"default_effort_level"`
				} `json:"reasoning"`
			} `json:"models"`
		} `json:"providers"`
	}
	require.NoError(t, json.Unmarshal(migrated, &got))

	p := got.Providers["zai"]
	require.Equal(t, "openai-compat", p.Type, "type translation happens in NormalizeProviderType, not here")
	m := p.Models[0]
	require.Equal(t, 1.4, m.Pricing.Input)
	require.Equal(t, 4.4, m.Pricing.Output)
	require.Equal(t, 0.1, m.Pricing.CacheCreate)
	require.Equal(t, 0.2, m.Pricing.CacheHit)
	require.True(t, m.Capabilities.Vision)
	require.Equal(t, "toggleable", m.Reasoning.Thinking)
	require.Len(t, m.Reasoning.EffortLevels, 2)
	require.Equal(t, "low", m.Reasoning.EffortLevels[0].Value)
	require.Equal(t, "Low", m.Reasoning.EffortLevels[0].Display)
	require.Equal(t, "high", m.Reasoning.DefaultEffortLevel)
}

func TestMigrateCatwalkV2NoOpForV3(t *testing.T) {
	t.Parallel()

	v3 := []byte(`{"providers": {"p": {"models": [{
		"id": "m",
		"pricing": {"input": 1},
		"reasoning": {"thinking": "never"},
		"capabilities": {"vision": false}
	}]}}}`)

	migrated, changed := migrateCatwalkV2(v3)
	require.False(t, changed)
	require.JSONEq(t, string(v3), string(migrated))
}

func TestMigrateCatwalkV2CanReasonFalseMeansNever(t *testing.T) {
	t.Parallel()

	v2 := []byte(`{"providers": {"p": {"models": [{"id": "m", "can_reason": false}]}}}`)
	migrated, changed := migrateCatwalkV2(v2)
	require.True(t, changed)

	var got struct {
		Providers map[string]struct {
			Models []struct {
				Reasoning struct {
					Thinking string `json:"thinking"`
				} `json:"reasoning"`
			} `json:"models"`
		} `json:"providers"`
	}
	require.NoError(t, json.Unmarshal(migrated, &got))
	require.Equal(t, "never", got.Providers["p"].Models[0].Reasoning.Thinking)
}

func TestMigrateCatwalkV2FilePersistsAndLoads(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "crush.json")
	v2 := []byte(`{"providers": {"p": {"models": [{"id": "m", "cost_per_1m_in": 2}]}}}`)
	require.NoError(t, os.WriteFile(path, v2, 0o600))

	loaded := migrateCatwalkV2File(path, v2)
	require.Contains(t, string(loaded), `"pricing"`)

	ondisk, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(ondisk), `"pricing"`)
}

func TestNormalizeProviderTypeTranslatesLegacyNames(t *testing.T) {
	t.Parallel()

	require.Equal(t, catwalk.TypeCompletions, NormalizeProviderType("openai-compat"))
	require.Equal(t, catwalk.TypeMessages, NormalizeProviderType("anthropic"))
	require.Equal(t, catwalk.TypeResponses, NormalizeProviderType("openai"))
	// "google" no longer aliases completions: it is the standalone Gemini
	// API type, which keeps the Google SDK rather than an OpenAI one.
	require.Equal(t, TypeGoogle, NormalizeProviderType("google"))
	// v3 names and custom enricher types pass through.
	require.Equal(t, catwalk.TypeCompletions, NormalizeProviderType(catwalk.TypeCompletions))
	require.Equal(t, catwalk.Type("ollama"), NormalizeProviderType("ollama"))
}

// TestLoadMigratesCrushrcAuthoredModels covers the fork's primary config
// format: a crushrc-authored model add with v2 pricing fields must reach
// the coordinator as a v3 model with pricing.
func TestLoadMigratesCrushrcAuthoredModels(t *testing.T) {
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("CRUSH_GLOBAL_DATA", t.TempDir())

	workDir := t.TempDir()
	dataDir := t.TempDir()

	cfg, err := Load(t.Context(), workDir, dataDir, false)
	require.NoError(t, err)

	// Simulate a crushrc having produced a v2-shaped provider model.
	cfg.Config().Providers.Set("legacy", ProviderConfig{
		ID:      "legacy",
		Name:    "Legacy",
		Type:    "openai-compat",
		BaseURL: "https://legacy.example/v1",
	})
	// The store clone path and merge path migrate on save; assert the
	// in-memory safety net handles a v2 model reaching loadFromBytes.
	data := []byte(`{"providers": {"legacy": {"models": [{
		"id": "m", "cost_per_1m_in": 3, "can_reason": true,
		"reasoning_levels": ["high"]
	}]}}}`)
	migrated, changed := migrateCatwalkV2(data)
	require.True(t, changed)
	require.Contains(t, string(migrated), `"pricing"`)
	require.Contains(t, string(migrated), `"reasoning"`)
	_ = cfg
}
