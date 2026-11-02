package agent

import (
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildProviderOpenCodeRouting(t *testing.T) {
	t.Parallel()

	for _, providerID := range []string{
		string(catwalk.InferenceProviderOpenCodeZen),
		string(catwalk.InferenceProviderOpenCodeGo),
	} {
		t.Run(providerID, func(t *testing.T) {
			t.Parallel()
			env := testEnv(t)
			providerCfg := config.ProviderConfig{
				ID:      providerID,
				BaseURL: "https://opencode.ai/zen/v1",
				Type:    catwalk.TypeCompletions,
				APIKey:  "$OPENCODE_API_KEY",
			}
			coord := newTestCoordinator(t, env, providerID, providerCfg)

			// The model's wire type now drives routing: messages goes
			// through the Anthropic provider, responses and completions
			// through the openai-compat one. All three must build.
			for _, wireType := range []catwalk.Type{catwalk.TypeMessages, catwalk.TypeResponses, catwalk.TypeCompletions} {
				provider, err := coord.buildProvider(providerCfg, config.SelectedModel{
					Model:    "kimi-k3",
					Provider: providerID,
				}, false, wireType)
				require.NoError(t, err, "%s/%s", providerID, wireType)
				require.NotNil(t, provider, wireType)
			}
		})
	}
}
