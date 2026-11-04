package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// budgetStreamModel rejects oversized requests, like a strict provider.
type budgetStreamModel struct {
	finishStreamModel
	calls    []fantasy.Call
	toolSent bool
	window   int64
}

func (m *budgetStreamModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.calls = append(m.calls, call)
	if tokens := estimateMessageTokens(call.Prompt); tokens > m.window {
		return nil, fmt.Errorf("prompt exceeds max length: %d > %d", tokens, m.window)
	}
	if len(call.Tools) > 0 && !m.toolSent {
		m.toolSent = true
		return func(yield func(fantasy.StreamPart) bool) {
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: "big-call", ToolCallName: "large_result", ToolCallInput: "{}"}) {
				return
			}
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls, Usage: fantasy.Usage{InputTokens: 100, OutputTokens: 10, TotalTokens: 110}})
		}, nil
	}
	return m.finishStreamModel.Stream(ctx, call)
}

// budgetSessionAgent is testSessionAgent with a chosen context window.
func budgetSessionAgent(env fakeEnv, large, small fantasy.LanguageModel, window int64, tools ...fantasy.AgentTool) SessionAgent {
	agent := NewSessionAgent(SessionAgentOptions{
		LargeModel: Model{
			Model: large,
			CatwalkCfg: catwalk.Model{
				ContextWindow:    window,
				DefaultMaxTokens: 10000,
			},
		},
		SmallModel: Model{
			Model: small,
			CatwalkCfg: catwalk.Model{
				ContextWindow:    window,
				DefaultMaxTokens: 10000,
			},
		},
		SystemPrompt: "system",
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     env.messages,
		Tools:        tools,
	})
	env.joinTitles(agent)
	return agent
}

func TestContextBudget_PendingToolResultsTriggerCompaction(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	model := &budgetStreamModel{finishStreamModel: finishStreamModel{text: "summary or answer"}, window: 20000}
	fullOutput := strings.Repeat("tool data ", 12000)
	tool := &fakeTool{name: "large_result", resp: fantasy.NewTextResponse(fullOutput)}
	sa := budgetSessionAgent(env, model, model, 20000, tool).(*sessionAgent)
	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "budget test")
	require.NoError(t, err)
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "Read the tool output"}},
	})
	require.NoError(t, err)

	_, err = sa.Run(ctx, SessionAgentCall{SessionID: sess.ID, Prompt: "Read it"})
	require.NoError(t, err, "pending tool results must trigger compaction instead of an oversized request")
	require.True(t, model.toolSent)
	require.GreaterOrEqual(t, len(model.calls), 2, "tool request and summary request")
	require.Empty(t, model.calls[1].Tools, "compact before the next normal request")
	require.Contains(t, fmt.Sprint(model.calls[1].Prompt), summaryOmission,
		"the summary request must carry the bounded tool output")

	updated, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.NotEmpty(t, updated.SummaryMessageID, "compaction must adopt a checkpoint")

	msgs, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)
	var originalFound bool
	for _, msg := range msgs {
		for _, result := range msg.ToolResults() {
			if result.ToolCallID == "big-call" {
				require.Equal(t, fullOutput, result.Content)
				originalFound = true
			}
		}
	}
	require.True(t, originalFound, "keep the full tool output in session history")
}

func TestContextBudget_SummaryKeepsToolPairsAndUTF8(t *testing.T) {
	t.Parallel()
	text := strings.Repeat("日本語🦀", 1000)
	output := &fantasy.ToolResultOutputContentText{Text: text}
	result := &fantasy.ToolResultPart{ToolCallID: "call-1", Output: output}
	messages := []fantasy.Message{
		fantasy.NewUserMessage("Keep this original request"),
		{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{fantasy.ToolCallPart{ToolCallID: "call-1", ToolName: "read", Input: "{}"}}},
		{Role: fantasy.MessageRoleTool, Content: []fantasy.MessagePart{result}},
	}
	const budget = 250
	bounded, err := fitSummaryToolResults(messages, budget)
	require.NoError(t, err)
	require.LessOrEqual(t, estimateMessageTokens(bounded), int64(budget))
	require.Equal(t, messages[0], bounded[0])
	require.Equal(t, messages[1], bounded[1])
	shortened := bounded[2].Content[0].(fantasy.ToolResultPart)
	require.Equal(t, "call-1", shortened.ToolCallID)
	shortText := shortened.Output.(fantasy.ToolResultOutputContentText).Text
	require.True(t, utf8.ValidString(shortText))
	require.Contains(t, shortText, summaryOmission)
	require.Equal(t, text, output.Text, "do not mutate shared output pointers")
	require.Same(t, result, messages[2].Content[0])
}

func TestContextBudget_SummaryRefusesToDropUserMessages(t *testing.T) {
	t.Parallel()
	messages := []fantasy.Message{fantasy.NewUserMessage(strings.Repeat("user data ", 1000))}
	_, err := fitSummaryToolResults(messages, 100)
	require.ErrorContains(t, err, "user messages and tool calls were kept intact")
}

func TestContextBudget_SummaryLeavesSmallPromptUntouched(t *testing.T) {
	t.Parallel()
	messages := []fantasy.Message{fantasy.NewUserMessage("small")}
	bounded, err := fitSummaryToolResults(messages, 100)
	require.NoError(t, err)
	require.Equal(t, messages, bounded)
}

func TestContextBudget_Reserve(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		window, reserve int64
	}{{0, 0}, {100000, 20000}, {200000, 40000}, {300000, 30000}, {1000000, 100000}} {
		require.Equal(t, tc.reserve, contextReserve(tc.window))
	}
}

func TestContextBudget_OnlyPendingClientResults(t *testing.T) {
	t.Parallel()
	result := fantasy.ToolResultContent{ToolCallID: "client", ToolName: "read", Result: fantasy.ToolResultOutputContentText{Text: "client output"}}
	providerResult := result
	providerResult.ProviderExecuted = true
	providerResult.ToolCallID = "provider"
	steps := []fantasy.StepResult{
		{Content: fantasy.ResponseContent{result}},
		{Content: fantasy.ResponseContent{result, providerResult}},
	}
	require.Equal(t, estimateToolResultContentTokens(result.ToolCallID, result.ToolName, result.ClientMetadata, result.Result), pendingToolResultTokens(steps))
	require.Zero(t, pendingToolResultTokens(nil))
}
