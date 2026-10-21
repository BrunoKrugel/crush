package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/config"
)

// contextFilesTemplate renders only the project context files' contents.
const contextFilesTemplate = `{{range .ContextFiles}}{{.Content}};{{end}}`

func TestWithContextPaths(t *testing.T) {
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "crush"))

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("global"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "NOTES.md"), []byte("agent"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "crush.json"),
		[]byte(`{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}}
}`), 0o644))
	store, err := config.Init(t.Context(), dir, "", false)
	require.NoError(t, err)

	render := func(opts ...Option) string {
		t.Helper()
		p, err := NewPrompt("context", contextFilesTemplate, append([]Option{WithWorkingDir(dir)}, opts...)...)
		require.NoError(t, err)
		out, err := p.Build(t.Context(), "provider", "model", store)
		require.NoError(t, err)
		return out
	}

	// The default reads options.context_paths, which includes AGENTS.md.
	require.Equal(t, "global;", render())
	// An agent's context paths (#432) replace the global ones.
	require.Equal(t, "agent;", render(WithContextPaths([]string{"NOTES.md"})))
	// An empty list keeps the global paths.
	require.Equal(t, "global;", render(WithContextPaths(nil)))
}

// Context files resolve against the store's working directory by
// default, and WithContextRoot (#561) moves that root without moving
// anything else: a dispatched agent reads its notes from the parent's
// checkout while its store still points at the worktree.
func TestWithContextRoot(t *testing.T) {
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "crush"))

	// The store's working directory stands in for a dispatch worktree,
	// the other directory for the parent's checkout. Each carries its
	// own AGENTS.md and a NOTES.md for an agent definition's
	// context_paths (#432).
	storeDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(storeDir, "AGENTS.md"), []byte("worktree"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(storeDir, "NOTES.md"), []byte("worktree-notes"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(storeDir, "crush.json"),
		[]byte(`{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}}
}`), 0o644))
	parentDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "AGENTS.md"), []byte("parent"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "NOTES.md"), []byte("parent-notes"), 0o644))

	store, err := config.Init(t.Context(), storeDir, "", false)
	require.NoError(t, err)
	// A relative global context path resolves against the same root.
	store.Config().Options.GlobalContextPaths = []string{"NOTES.md"}

	const bothTemplate = `{{range .ContextFiles}}{{.Content}};{{end}}|{{range .GlobalContextFiles}}{{.Content}};{{end}}`
	render := func(opts ...Option) string {
		t.Helper()
		p, err := NewPrompt("context", bothTemplate, append([]Option{WithWorkingDir(storeDir)}, opts...)...)
		require.NoError(t, err)
		out, err := p.Build(t.Context(), "provider", "model", store)
		require.NoError(t, err)
		return out
	}

	// The main prompt path is unchanged: no root means the store's
	// working directory.
	require.Equal(t, "worktree;|worktree-notes;", render())
	require.Equal(t, "worktree;|worktree-notes;", render(WithContextRoot("")))
	// A root moves every relative context path, the agent definition's
	// included, while the store itself stays where it was.
	require.Equal(t, "parent;|parent-notes;", render(WithContextRoot(parentDir)))
	require.Equal(t, "parent-notes;|parent-notes;", render(WithContextRoot(parentDir), WithContextPaths([]string{"NOTES.md"})))
	require.Equal(t, storeDir, store.WorkingDir())
	// An absolute context path is read as given, whatever the root.
	absolute := filepath.Join(storeDir, "NOTES.md")
	require.Equal(t, "worktree-notes;|parent-notes;", render(WithContextRoot(parentDir), WithContextPaths([]string{absolute})))
}

// gatedSkill is the builtin skill that declares `requires: honcho`, and
// so is the one that must appear or vanish with the feature.
const gatedSkill = "honcho-memory"

// buildSkillXML renders a prompt whose whole body is the skill block, so
// the assertions below read the exact text the model would receive.
func buildSkillXML(t *testing.T, opts ...Option) string {
	t.Helper()

	store, err := config.Init(t.Context(), t.TempDir(), "", false)
	require.NoError(t, err)

	p, err := NewPrompt("test", "{{.AvailSkillXML}}", opts...)
	require.NoError(t, err)

	out, err := p.Build(t.Context(), "", "", store)
	require.NoError(t, err)
	return out
}

func TestSkillGatingHidesSkillWithoutFeature(t *testing.T) {
	t.Parallel()

	// An unset features func means no integrations are live, which is
	// the default every one-off prompt gets.
	xml := buildSkillXML(t)

	require.NotContains(t, xml, gatedSkill,
		"a skill requiring an unavailable integration must stay out of the prompt")
	// Guard against the assertion passing because skill discovery
	// produced nothing at all.
	require.Contains(t, xml, "crush-config",
		"ungated skills must still be present")
}

func TestSkillGatingShowsSkillWithFeature(t *testing.T) {
	t.Parallel()

	xml := buildSkillXML(t, WithFeatures(func() []string { return []string{"honcho"} }))

	require.Contains(t, xml, gatedSkill,
		"a skill whose required integration is live must be offered")
}

// TestSkillGatingIsReEvaluatedPerBuild is the regression guard for
// connecting memory mid-session. The features func is deliberately not a
// slice captured at construction: a backend connected after the prompt
// was built must show up on the next build without a restart.
func TestSkillGatingIsReEvaluatedPerBuild(t *testing.T) {
	t.Parallel()

	store, err := config.Init(t.Context(), t.TempDir(), "", false)
	require.NoError(t, err)

	// live stands in for the memory backend being connected partway
	// through the session.
	live := false
	p, err := NewPrompt("test", "{{.AvailSkillXML}}", WithFeatures(func() []string {
		if !live {
			return nil
		}
		return []string{"honcho"}
	}))
	require.NoError(t, err)

	before, err := p.Build(t.Context(), "", "", store)
	require.NoError(t, err)
	require.NotContains(t, before, gatedSkill, "skill must be hidden before connecting")

	live = true

	after, err := p.Build(t.Context(), "", "", store)
	require.NoError(t, err)
	require.Contains(t, after, gatedSkill, "skill must appear once connected, without a rebuild")
}

// TestSkillGatingDoesNotConsultConfig pins the behaviour that motivated
// the change: gating follows the live integration, not what the user
// asked for. An integration can be enabled in config and still fail to
// start, and a prompt that trusted config would promise the model tools
// it was never given.
func TestSkillGatingDoesNotConsultConfig(t *testing.T) {
	t.Parallel()

	store, err := config.Init(t.Context(), t.TempDir(), "", false)
	require.NoError(t, err)

	// Enable memory in config while leaving the features func unset,
	// which is what "configured but failed to start" looks like here.
	store.Config().Honcho = &config.Honcho{Enabled: true}

	p, err := NewPrompt("test", "{{.AvailSkillXML}}")
	require.NoError(t, err)

	xml, err := p.Build(t.Context(), "", "", store)
	require.NoError(t, err)

	require.NotContains(t, xml, gatedSkill,
		"config alone must not unlock a skill when the integration is not actually live")
}

// TestSkillGatingLeavesDisabledSkillsAlone checks the two filters
// compose, since both run over the same slice in sequence.
func TestSkillGatingLeavesDisabledSkillsAlone(t *testing.T) {
	t.Parallel()

	store, err := config.Init(t.Context(), t.TempDir(), "", false)
	require.NoError(t, err)
	store.Config().Options.DisabledSkills = []string{"crush-config"}

	p, err := NewPrompt("test", "{{.AvailSkillXML}}",
		WithFeatures(func() []string { return []string{"honcho"} }))
	require.NoError(t, err)

	xml, err := p.Build(t.Context(), "", "", store)
	require.NoError(t, err)

	require.NotContains(t, xml, "crush-config", "explicitly disabled skills stay out")
	require.Contains(t, xml, gatedSkill, "gating must not disturb the disabled-skill filter")
	require.True(t, strings.Contains(xml, "jq"), "unrelated skills survive both filters")
}
