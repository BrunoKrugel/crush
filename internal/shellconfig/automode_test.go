package shellconfig

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// autoModeSection returns the auto_mode block from a loaded script.
func autoModeSection(t *testing.T, script string) map[string]any {
	t.Helper()
	result := loadScript(t, script)
	section, ok := result["auto_mode"].(map[string]any)
	require.True(t, ok, "expected an auto_mode section, got %v", result)
	return section
}

func TestAutoModeEnabled(t *testing.T) {
	t.Parallel()

	am := autoModeSection(t, `auto_mode on`)
	require.Equal(t, true, am["enabled"])
}

func TestAutoModeOff(t *testing.T) {
	t.Parallel()

	// Directives are applied in execution order, so an explicit disable
	// written after other settings must survive.
	am := autoModeSection(t, `auto_mode on
auto_mode off`)
	require.Equal(t, false, am["enabled"])
}

func TestAutoModeEnabledBoolKey(t *testing.T) {
	t.Parallel()

	am := autoModeSection(t, `auto_mode enabled
auto_mode fail-open true`)
	require.Equal(t, true, am["enabled"], "a bare boolean key means true")
	require.Equal(t, true, am["fail_open"])
}

// TestAutoModeFullSurface covers every key the builtin accepts, in the
// upstream PR #3874 surface plus this fork's standalone-endpoint keys.
func TestAutoModeFullSurface(t *testing.T) {
	t.Parallel()

	am := autoModeSection(t, `auto_mode on
auto_mode model local-llama/gemma4-e4b
auto_mode max-tokens 2048
auto_mode timeout 90
auto_mode fail-open false
auto_mode environment "Trusted repo: the working repository and its git remotes"
auto_mode prompt stage1 ./prompts/stage1.tmpl
auto_mode transcript-max-chars 12000
auto_mode max-consecutive-denials 3
auto_mode max-total-denials 20
auto_mode base-url http://127.0.0.1:8080/v1
auto_mode api-key test-key-value`)

	require.Equal(t, true, am["enabled"])
	require.Equal(t, float64(2048), am["max_tokens"])
	require.Equal(t, float64(90), am["timeout_seconds"])
	require.Equal(t, false, am["fail_open"])
	require.Equal(t, "./prompts/stage1.tmpl", am["prompt_stage1_file"])
	require.Equal(t, float64(12000), am["transcript_max_chars"])
	require.Equal(t, float64(3), am["max_consecutive_denials"])
	require.Equal(t, float64(20), am["max_total_denials"])

	env, ok := am["environment"].([]any)
	require.True(t, ok, "environment should be an array")
	require.Len(t, env, 1)
	require.Contains(t, env[0], "Trusted repo")

	classifier, ok := am["classifier"].(map[string]any)
	require.True(t, ok, "classifier should be an object")
	require.Equal(t, "local-llama", classifier["provider"])
	require.Equal(t, "gemma4-e4b", classifier["model"])
	require.Equal(t, "http://127.0.0.1:8080/v1", classifier["base_url"])
	require.Equal(t, "test-key-value", classifier["api_key"])
}

func TestAutoModeEnvironmentAppends(t *testing.T) {
	t.Parallel()

	am := autoModeSection(t, `auto_mode environment "Trusted repo: example"
auto_mode environment "Trusted domain: example.com"`)

	env, ok := am["environment"].([]any)
	require.True(t, ok)
	require.Len(t, env, 2)
}

func TestAutoModePromptBothStages(t *testing.T) {
	t.Parallel()

	am := autoModeSection(t, `auto_mode prompt stage1 ./s1.tmpl
auto_mode prompt stage2 ./s2.tmpl`)
	require.Equal(t, "./s1.tmpl", am["prompt_stage1_file"])
	require.Equal(t, "./s2.tmpl", am["prompt_stage2_file"])
}

func TestAutoModeAbsentWithoutDirectives(t *testing.T) {
	t.Parallel()

	// No auto_mode directives means no auto_mode section, which is what
	// keeps the feature off by default.
	result := loadScript(t, `option debug true`)
	require.NotContains(t, result, "auto_mode")
}

func TestAutoModeRejectsBadValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		script string
	}{
		{"unknown key", `auto_mode nonsense value`},
		{"no key", `auto_mode`},
		{"bad model spec", `auto_mode model gemma4-e4b`},
		{"model without value", `auto_mode model`},
		{"bad prompt stage", `auto_mode prompt stage3 ./x`},
		{"prompt without file", `auto_mode prompt stage1`},
		{"bad fail-open", `auto_mode fail-open maybe`},
		{"non-numeric max-tokens", `auto_mode max-tokens lots`},
		{"negative max-tokens", `auto_mode max-tokens -5`},
		{"non-numeric timeout", `auto_mode timeout soon`},
		{"negative timeout", `auto_mode timeout -1`},
		{"negative transcript cap", `auto_mode transcript-max-chars -1`},
		{"non-numeric denials", `auto_mode max-total-denials all`},
		{"int key without value", `auto_mode max-tokens`},
		{"base-url without value", `auto_mode base-url`},
		{"api-key without value", `auto_mode api-key`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Error(t, loadScriptErr(t, tc.script))
		})
	}
}

func TestAutoModeLastWriteWins(t *testing.T) {
	t.Parallel()

	am := autoModeSection(t, `auto_mode model local-llama/gemma
auto_mode model local-llama/qwen`)
	cl := am["classifier"].(map[string]any)
	require.Equal(t, "qwen", cl["model"])
}

func TestAutoModeRegistered(t *testing.T) {
	t.Parallel()

	// The builtin must be registered so a crushrc in a real load path
	// resolves it instead of reporting an unknown command.
	path := filepath.Join(t.TempDir(), "crushrc")
	_, err := LoadShellConfig(t.Context(), path, []byte(`auto_mode on`))
	require.NoError(t, err)
}
