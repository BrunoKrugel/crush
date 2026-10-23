package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/require"
)

// TestAutoModeDefaults pins the tested defaults for the auto mode
// section: disabled, fail-closed, 3 consecutive / 20 total denial
// quotas. Unset knobs must resolve to these, not to zero values that
// would change behavior.
func TestAutoModeDefaults(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	resolved, enabled := cfg.ResolvedAutoMode()
	require.False(t, enabled, "auto mode must default to off")
	require.False(t, resolved.Enabled)
	require.False(t, resolved.FailOpen)
	require.Equal(t, 3, resolved.MaxConsecutiveDenials)
	require.Equal(t, 20, resolved.MaxTotalDenials)
	require.Equal(t, 1024, resolved.MaxTokens)
	require.Equal(t, 60, resolved.TimeoutSeconds)
	require.Equal(t, 24000, resolved.TranscriptMaxChars)
}

func TestAutoModeEnabledResolves(t *testing.T) {
	t.Parallel()

	cfg := &Config{AutoMode: &AutoModeConfig{Enabled: true}}
	_, enabled := cfg.ResolvedAutoMode()
	require.True(t, enabled)
}

// TestAutoModeValidation pins the load-time rejection of negative knobs,
// matching the TodoEnforcement and Dispatch validation style: 0 keeps the
// default, a positive value configures it, and a negative value is a load
// error that names its path.
func TestAutoModeValidation(t *testing.T) {
	t.Parallel()

	valid := &AutoModeConfig{Enabled: true, MaxConsecutiveDenials: 5, MaxTotalDenials: 50}
	require.NoError(t, valid.Validate("auto_mode"))

	tests := []struct {
		name   string
		config AutoModeConfig
		field  string
	}{
		{"negative max tokens", AutoModeConfig{MaxTokens: -1}, "max_tokens"},
		{"negative timeout", AutoModeConfig{TimeoutSeconds: -5}, "timeout_seconds"},
		{"negative transcript cap", AutoModeConfig{TranscriptMaxChars: -1}, "transcript_max_chars"},
		{"negative consecutive denials", AutoModeConfig{MaxConsecutiveDenials: -1}, "max_consecutive_denials"},
		{"negative total denials", AutoModeConfig{MaxTotalDenials: -1}, "max_total_denials"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.config.Validate("auto_mode")
			require.Error(t, err)
			require.Contains(t, err.Error(), "auto_mode."+tc.field)
		})
	}
}

// TestCloneForWrite_AutoModeIsolation verifies the auto-mode pointer and
// its nested classifier pointer are deep-copied: a config reload must
// never mutate the live published config through a shared struct.
func TestCloneForWrite_AutoModeIsolation(t *testing.T) {
	t.Parallel()

	orig := &Config{
		Providers: csync.NewMap[string, ProviderConfig](),
		AutoMode: &AutoModeConfig{
			Enabled:    true,
			Classifier: &AutoModeClassifier{Provider: "local-llama", Model: "gemma4-e4b"},
		},
	}

	clone := orig.cloneForWrite()
	clone.AutoMode.Enabled = false
	clone.AutoMode.Classifier.Model = "other"

	require.True(t, orig.AutoMode.Enabled, "AutoMode.Enabled leaked")
	require.Equal(t, "gemma4-e4b", orig.AutoMode.Classifier.Model, "AutoMode.Classifier leaked")
}

// TestAutoModeClassifierFallbackOrder pins the classifier model
// resolution: an explicit auto_mode.classifier selection wins, then the
// small slot, then the large slot. This matches upstream PR #3874's
// documented order (explicit selection → small → large) and the fork's
// resolveSelectedModels, which always fills both slots.
func TestAutoModeClassifierFallbackOrder(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Models: map[SelectedModelType]SelectedModel{
			SelectedModelTypeLarge: {Provider: "anthropic", Model: "claude-large"},
			SelectedModelTypeSmall: {Provider: "openai", Model: "gpt-small"},
		},
		AutoMode: &AutoModeConfig{
			Classifier: &AutoModeClassifier{Provider: "local-llama", Model: "gemma4-e4b"},
		},
	}
	sel, ok := cfg.AutoModeClassifierModel()
	require.True(t, ok)
	require.Equal(t, "local-llama", sel.Provider)
	require.Equal(t, "gemma4-e4b", sel.Model)

	// Without an explicit selection the small slot is preferred.
	cfg.AutoMode.Classifier = nil
	sel, ok = cfg.AutoModeClassifierModel()
	require.True(t, ok)
	require.Equal(t, "openai", sel.Provider)
	require.Equal(t, "gpt-small", sel.Model)

	// Without a small slot the large slot is used.
	delete(cfg.Models, SelectedModelTypeSmall)
	sel, ok = cfg.AutoModeClassifierModel()
	require.True(t, ok)
	require.Equal(t, "anthropic", sel.Provider)
	require.Equal(t, "claude-large", sel.Model)

	// With no model at all classification runs rules-only.
	delete(cfg.Models, SelectedModelTypeLarge)
	_, ok = cfg.AutoModeClassifierModel()
	require.False(t, ok, "no slots means rules-only classification")
}

// TestAutoModeUserLevelOnly is the security test for the "a repository
// must not be able to grant itself auto mode" property: an auto_mode
// section in a project-level config file is ignored (with a warning),
// while the same section in a user-level config is honored.
func TestAutoModeUserLevelOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tmp := t.TempDir()
	project := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(project, 0o755))

	projectCfg := filepath.Join(project, "crush.json")
	require.NoError(t, os.WriteFile(projectCfg, []byte(`{"auto_mode": {"enabled": true}}`), 0o644))

	cfg, _, err := loadFromConfigPaths(ctx, []string{projectCfg}, 0)
	require.NoError(t, err)
	_, enabled := cfg.ResolvedAutoMode()
	require.False(t, enabled, "a repository must not be able to grant itself auto mode")

	// The same section at a user-level path is honored.
	userCfg := filepath.Join(tmp, "user", "crush.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(userCfg), 0o755))
	require.NoError(t, os.WriteFile(userCfg, []byte(`{"auto_mode": {"enabled": true}}`), 0o644))
	cfg, _, err = loadFromConfigPaths(ctx, []string{projectCfg, userCfg}, 1)
	require.NoError(t, err)
	_, enabled = cfg.ResolvedAutoMode()
	require.True(t, enabled, "user-level config may enable auto mode")
}
