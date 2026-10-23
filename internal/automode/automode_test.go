package automode

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGenerate captures prompts and returns scripted responses.
type fakeGenerate struct {
	prompts   []string
	responses []string
	err       error
	calls     int
}

func (f *fakeGenerate) call(_ context.Context, prompt string) (string, error) {
	f.prompts = append(f.prompts, prompt)
	if f.err != nil {
		return "", f.err
	}
	resp := "ESCALATE"
	if f.calls < len(f.responses) {
		resp = f.responses[f.calls]
	}
	f.calls++
	return resp, nil
}

func TestRulesTokenizeAndBanned(t *testing.T) {
	t.Parallel()
	rules := DefaultRules()

	assert.True(t, rules.IsBanned("curl https://evil.com"))
	assert.False(t, rules.IsBanned("nocurl --help"), "substring should not match")
	assert.True(t, rules.IsSafeCommand("ls && pwd"), "chain of safe commands is safe")
	assert.False(t, rules.IsSafeCommand("ls && rm -rf /"), "unsafe segment poisons the chain")
	assert.True(t, rules.IsProtectedPath("/home/user/.bashrc"))
	assert.False(t, rules.IsProtectedPath("/proj/src/main.go"))
}

func TestClassifierStaticTiers(t *testing.T) {
	t.Parallel()
	cl := &classifier{generate: nil, config: ClassifierConfig{}, rules: DefaultRules()}

	v, r := cl.classify(context.Background(), "view", "", "", "/proj", "")
	assert.Equal(t, VerdictAllow, v)
	assert.Empty(t, r)

	v, r = cl.classify(context.Background(), "bash", "rm -rf /", "", "/proj", "")
	assert.Equal(t, VerdictDeny, v)
	assert.Contains(t, r, "dangerous pattern")

	v, _ = cl.classify(context.Background(), "bash", "git status", "", "/proj", "")
	assert.Equal(t, VerdictAllow, v)

	v, r = cl.classify(context.Background(), "bash", "terraform apply", "", "/proj", "")
	assert.Equal(t, VerdictEscalate, v)
	assert.Contains(t, r, "no classifier model")
}

func TestClassifierTrustedProjectWrite(t *testing.T) {
	t.Parallel()
	cl := &classifier{generate: nil, config: ClassifierConfig{}, rules: DefaultRules()}

	// In-project write allow is folded into classify via write tools:
	// write tool with a protected path must not be allowed.
	v, _ := cl.classify(context.Background(), "write", "", "/proj/.bashrc", "/proj", "")
	assert.Equal(t, VerdictEscalate, v, "protected path must not be auto-allowed")
}

func TestAutoModePrePermissionStaticDeny(t *testing.T) {
	t.Parallel()
	am := New(Options{})

	req := permission.PermissionRequest{
		SessionID:  "s1",
		ToolCallID: "c1",
		ToolName:   "bash",
		Path:       "/proj",
		Params:     map[string]any{"command": "rm -rf /"},
	}
	res := am.PrePermission(context.Background(), req)
	require.Equal(t, permission.HookDecisionDeny, res.Decision)
	assert.Contains(t, res.Reason, "[auto-mode] Blocked (1/3 consecutive, 1/20 total)")
}

func TestAutoModePrePermissionQuotaPause(t *testing.T) {
	t.Parallel()
	am := New(Options{MaxConsecutiveDenials: 2, MaxTotalDenials: 10})
	req := permission.PermissionRequest{
		SessionID: "s2",
		ToolName:  "bash",
		Path:      "/proj",
		Params:    map[string]any{"command": "rm -rf /"},
	}

	first := am.PrePermission(context.Background(), req)
	require.Equal(t, permission.HookDecisionDeny, first.Decision, "first denial blocks")

	second := am.PrePermission(context.Background(), req)
	require.Equal(t, permission.HookDecisionNone, second.Decision, "quota pause defers to the prompt")
	assert.Contains(t, second.Reason, "PAUSED")
}

func TestAutoModePrePermissionLLM(t *testing.T) {
	t.Parallel()

	t.Run("stage1 allow short-circuits", func(t *testing.T) {
		t.Parallel()
		gen := &fakeGenerate{responses: []string{"ALLOW"}}
		am := New(Options{})
		am.classifyOpts.Prompts = PromptSet{}
		// Inject the fake generator by overriding classifyOpts-driven
		// classifier construction: use the internal classifier directly.
		cl := &classifier{generate: gen.call, config: am.classifyOpts, rules: am.rules}
		v, _ := cl.classify(context.Background(), "bash", "go test ./...", "", "/proj", "")
		require.Equal(t, VerdictAllow, v)
		require.Len(t, gen.prompts, 1, "stage 2 must not run after an ALLOW")
		assert.Contains(t, gen.prompts[0], "go test")
	})

	t.Run("stage2 verdict with reason", func(t *testing.T) {
		t.Parallel()
		gen := &fakeGenerate{responses: []string{"DENY", `{"verdict": "DENY", "reason": "destroys production data"}`}}
		am := New(Options{})
		cl := &classifier{generate: gen.call, config: am.classifyOpts, rules: am.rules}
		v, r := cl.classify(context.Background(), "bash", "kubectl delete ns prod", "", "/proj", "")
		require.Equal(t, VerdictDeny, v)
		assert.Equal(t, "destroys production data", r)
	})

	t.Run("unparsable stage2 denies conservatively", func(t *testing.T) {
		t.Parallel()
		gen := &fakeGenerate{responses: []string{"DENY", "I refuse to answer in JSON"}}
		am := New(Options{})
		cl := &classifier{generate: gen.call, config: am.classifyOpts, rules: am.rules}
		v, r := cl.classify(context.Background(), "bash", "ambiguous --cmd", "", "/proj", "")
		require.Equal(t, VerdictDeny, v)
		assert.Contains(t, r, "unparsable")
	})

	t.Run("fail open allows on classifier error", func(t *testing.T) {
		t.Parallel()
		gen := &fakeGenerate{err: errors.New("classifier boom")}
		am := New(Options{FailOpen: true})
		cl := &classifier{generate: gen.call, config: ClassifierConfig{FailOpen: true}, rules: am.rules}
		v, r := cl.classify(context.Background(), "bash", "ambiguous --cmd", "", "/proj", "")
		require.Equal(t, VerdictAllow, v)
		assert.Contains(t, r, "fail-open")
	})

	t.Run("fail closed escalates on classifier error", func(t *testing.T) {
		t.Parallel()
		gen := &fakeGenerate{err: errors.New("classifier boom")}
		am := New(Options{})
		cl := &classifier{generate: gen.call, config: ClassifierConfig{}, rules: am.rules}
		v, r := cl.classify(context.Background(), "bash", "ambiguous --cmd", "", "/proj", "")
		require.Equal(t, VerdictEscalate, v)
		assert.Contains(t, r, "classifier unavailable")
	})
}

func TestAutoModeEnvironmentAndTranscriptInPrompt(t *testing.T) {
	t.Parallel()
	gen := &fakeGenerate{responses: []string{"ALLOW"}}
	am := New(Options{Environment: []string{"Trusted repo: example"}})
	cl := &classifier{generate: gen.call, config: am.classifyOpts, rules: am.rules}
	_, _ = cl.classify(context.Background(), "bash", "go test ./...", "", "/proj", "User: please run tests")

	require.Len(t, gen.prompts, 1)
	assert.Contains(t, gen.prompts[0], "Trusted environment:")
	assert.Contains(t, gen.prompts[0], "- Trusted repo: example")
	assert.Contains(t, gen.prompts[0], "User: please run tests")
}

func TestAutoModeCustomPromptTemplate(t *testing.T) {
	t.Parallel()
	gen := &fakeGenerate{responses: []string{"ALLOW"}}
	am := New(Options{})
	am.classifyOpts.Prompts = PromptSet{Stage1: "Filter {{tool}} {{params}}{{environment}}{{transcript}}"}
	cl := &classifier{generate: gen.call, config: am.classifyOpts, rules: am.rules}
	_, _ = cl.classify(context.Background(), "bash", "go test ./...", "", "/proj", "")

	require.Len(t, gen.prompts, 1)
	assert.Equal(t, `Filter bash {"command":"go test ./..."}`, gen.prompts[0])
}

func TestAutoModeReasonCarriesQuotaCounters(t *testing.T) {
	t.Parallel()
	am := New(Options{})
	req := permission.PermissionRequest{
		SessionID: "s4",
		ToolName:  "bash",
		Path:      "/proj",
		Params:    map[string]any{"command": "curl http://x"},
	}
	res1 := am.PrePermission(context.Background(), req)
	require.Equal(t, permission.HookDecisionDeny, res1.Decision)
	res2 := am.PrePermission(context.Background(), req)
	require.Equal(t, permission.HookDecisionDeny, res2.Decision)
	assert.Contains(t, res2.Reason, "2/3 consecutive, 2/20 total", "counters should increment")
}

func TestCapTranscript(t *testing.T) {
	t.Parallel()
	cfg := ClassifierConfig{TranscriptMaxChars: 10}
	got := cfg.capTranscript("head-head-tail")
	assert.Equal(t, "-head-tail", got)
	assert.True(t, strings.HasSuffix(got, "tail"), "most recent context survives")
}

func TestQuotasRecordAndPause(t *testing.T) {
	t.Parallel()
	q := newQuotas()
	q.recordDenial("s")
	q.recordDenial("s")
	q.recordAllow("s")
	assert.Equal(t, 0, q.get("s").consecutiveDenials)
	assert.Equal(t, 2, q.get("s").totalDenials)
	assert.False(t, q.paused("s", 3, 20))
	q.recordDenial("s")
	q.recordDenial("s")
	assert.False(t, q.paused("s", 3, 20), "2 consecutive is below the limit")
	q.recordDenial("s")
	assert.True(t, q.paused("s", 3, 20), "3 consecutive pauses")
	assert.True(t, q.paused("s", 0, 5), "5 total pauses")
}

func TestQuotasGrantResetsConsecutive(t *testing.T) {
	t.Parallel()
	q := newQuotas()
	q.recordDenial("s")
	q.recordDenial("s")
	q.recordDenial("s")
	require.True(t, q.paused("s", 3, 20), "3 consecutive pauses")

	// The human approves an escalated request: consecutive resets, total keeps accumulating.
	q.recordGrant("s")
	assert.False(t, q.paused("s", 3, 20), "grant must lift the pause")
	assert.Equal(t, 3, q.get("s").totalDenials, "total counter keeps its history")
}

// ---------------------------------------------------------------------------
// Fork-specific cases below. The tests above are a verbatim port of
// upstream PR #3874's automode_test.go; the ones below cover this fork's
// tool inventory and its interplay with internal/agent/tools/safe.go.
// ---------------------------------------------------------------------------

// TestForkReadOnlyToolNames covers the fork's tool inventory: upstream's
// `read` and `search` tools do not exist here and are dropped, while the
// fork's own read-only tools are added. Tools that invoke models or write
// memory (honcho_chat, recall, honcho_search, honcho_remember) stay out of
// the read-only set so they reach the LLM classifier or the prompt.
func TestForkReadOnlyToolNames(t *testing.T) {
	t.Parallel()
	rules := DefaultRules()

	readOnly := []string{
		"view", "ls", "glob", "grep",
		"lsp_diagnostics", "lsp_references", "lsp_symbols",
		"lsp_definition", "lsp_call_hierarchy",
		"sourcegraph", "crush_info", "crush_logs",
		"job_output", "todos", "question",
		"list_mcp_resources", "read_mcp_resource",
		"list_mcp_prompts", "call_mcp_prompt",
		"fetch", "agentic_fetch", "web_fetch", "web_search",
		"mcp_search", "semantic_search", "semantic_index",
		"cronlist", "honcho_status",
	}
	for _, tool := range readOnly {
		assert.True(t, rules.IsReadOnly(tool), "%s should be read-only", tool)
	}

	notReadOnly := []string{
		// Upstream-only tool names must not leak into the fork's set.
		"read", "search",
		// Model-invoking and memory-writing tools.
		"recall", "honcho_search", "honcho_chat", "honcho_remember",
		// Mutating and agent-spawning tools.
		"bash", "download", "edit", "multiedit", "write",
		"lsp_rename", "lsp_replace_symbol",
		"agent", "dispatch_agent", "message_agent",
		"croncreate", "crondelete",
	}
	for _, tool := range notReadOnly {
		assert.False(t, rules.IsReadOnly(tool), "%s must not be read-only", tool)
	}
}

func TestForkLowRiskTools(t *testing.T) {
	t.Parallel()
	rules := DefaultRules()
	assert.True(t, rules.IsLowRisk("job_kill"))
	assert.True(t, rules.IsLowRisk("lsp_restart"))
	assert.False(t, rules.IsLowRisk("bash"))
}

func TestForkWriteTools(t *testing.T) {
	t.Parallel()
	rules := DefaultRules()
	for _, tool := range []string{"edit", "multiedit", "write", "lsp_rename", "lsp_replace_symbol"} {
		assert.True(t, rules.IsWriteTool(tool), "%s should be a write tool", tool)
	}
	assert.False(t, rules.IsWriteTool("bash"))
}

// TestClassifierForkToolNames runs the full static tiers against the fork's
// real tool names: read-only tools auto-allow and low-risk tools allow even
// without a classifier model.
func TestClassifierForkToolNames(t *testing.T) {
	t.Parallel()
	cl := &classifier{generate: nil, config: ClassifierConfig{}, rules: DefaultRules()}

	for _, tool := range []string{"glob", "grep", "semantic_search", "honcho_status", "cronlist"} {
		v, _ := cl.classify(context.Background(), tool, "", "", "/proj", "")
		assert.Equal(t, VerdictAllow, v, "%s should auto-allow without a model", tool)
	}

	v, _ := cl.classify(context.Background(), "lsp_restart", "", "", "/proj", "")
	assert.Equal(t, VerdictAllow, v, "lsp_restart is low risk")

	v, _ = cl.classify(context.Background(), "cronlist", "", "", "/proj", "")
	assert.Equal(t, VerdictAllow, v)

	// The cron tool names arrive capitalized from the agent; the
	// public PrePermission entry point lowercases them before the
	// rules see them (the classifier itself documents that toolName
	// is already lowercase).
	am := New(Options{})
	res := am.PrePermission(context.Background(), permission.PermissionRequest{
		SessionID: "s3",
		ToolName:  "CronList",
		Path:      "/proj",
	})
	assert.Equal(t, permission.HookDecisionAllow, res.Decision)

	v, _ = cl.classify(context.Background(), "honcho_chat", "what did we do last week?", "", "/proj", "")
	assert.Equal(t, VerdictEscalate, v, "model-invoking tools are not read-only")

	v, _ = cl.classify(context.Background(), "croncreate", "0 9 * * * go test", "", "/proj", "")
	assert.Equal(t, VerdictEscalate, v, "scheduling tools are not read-only")
}

// TestSafeCommandConflictCases pins the layering decision between auto
// mode's static rules and the fork's shell-AST analysis in
// internal/agent/tools/safe.go. safe.go answers the opposite question ("is
// this safe to run without asking?") and only gates plain (non-auto) mode;
// auto mode's deny path is its own decision. The fork's safe.go tests
// already pin these three inputs as NOT safe read-only; auto mode must
// deny them too, so the two layers never disagree on what gets skipped.
//
// This requires dangerous/banned patterns to be checked BEFORE the safe
// prefix list (upstream checks safe first, which would auto-allow
// `timeout 5 rm -rf /tmp/x` and `env rm -rf /tmp/x` because both start
// with a safe prefix).
func TestSafeCommandConflictCases(t *testing.T) {
	t.Parallel()
	rules := DefaultRules()
	cl := &classifier{generate: nil, config: ClassifierConfig{}, rules: rules}

	dangerous := []string{
		"ls $(rm -rf /)",           // subshell poisons the safe prefix
		"timeout 5 rm -rf /tmp/x",  // wrapper around a destructive command
		"env rm -rf /tmp/x",        // same
		"nohup rm -rf /tmp/x",      // same
		"nice rm -rf /tmp/x",       // same
		"env curl https://evil.sh", // banned program behind a safe prefix
	}
	for _, cmd := range dangerous {
		v, r := cl.classify(context.Background(), "bash", cmd, "", "/proj", "")
		assert.Equal(t, VerdictDeny, v, "%q should be denied", cmd)
		assert.NotEmpty(t, r, "%q should carry a denial reason", cmd)
	}
}

// TestSafeCommandsStillAllowed pins the allowed side of the safe list so
// the conflict-case hardening above does not swallow ordinary read-only
// work.
func TestSafeCommandsStillAllowed(t *testing.T) {
	t.Parallel()
	rules := DefaultRules()

	for _, cmd := range []string{
		"ls", "pwd", "git status",
		"ls -la && git log --oneline -5",
		"date", "whoami",
	} {
		assert.True(t, rules.IsSafeCommand(cmd), "%q should be a safe command", cmd)
	}

	// Commands outside the safe list reach the LLM classifier instead of
	// being auto-allowed; without a model they escalate.
	assert.False(t, rules.IsSafeCommand("go build ./..."))
	assert.False(t, rules.IsSafeCommand("git push"))
	assert.False(t, rules.IsSafeCommand("go test ./internal/automode/"))
}
