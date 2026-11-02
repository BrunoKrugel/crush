package discover

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func TestMergeCatalog(t *testing.T) {
	t.Parallel()

	catalog := []catwalk.Model{
		{
			ID:   "glm-5.3",
			Name: "GLM-5.3",
			Pricing: catwalk.Pricing{
				Input:    1,
				Output:   3,
				CacheHit: 0.5,
			},
			ContextWindow:    1_000_000,
			DefaultMaxTokens: 128_000,
			Reasoning: catwalk.Reasoning{
				Thinking:           catwalk.ThinkingToggleable,
				EffortLevels:       catwalk.NewEffortLevels("low", "high", "max"),
				DefaultEffortLevel: "high",
			},
			Capabilities: catwalk.Capabilities{Vision: true},
		},
		{ID: "catalog-only", Name: "Catalog Only"},
	}

	t.Run("fills bare discovered models", func(t *testing.T) {
		t.Parallel()
		models := []catwalk.Model{{ID: "glm-5.3", Name: "glm-5.3"}}

		got := MergeCatalog(models, catalog)
		require.Len(t, got, 1, "catalog-only models must not be added")
		require.Equal(t, catalog[0], got[0])
	})

	t.Run("fills an unnamed model", func(t *testing.T) {
		t.Parallel()
		got := MergeCatalog([]catwalk.Model{{ID: "glm-5.3"}}, catalog)
		require.Equal(t, "GLM-5.3", got[0].Name)
	})

	t.Run("keeps values the user set", func(t *testing.T) {
		t.Parallel()
		models := []catwalk.Model{{
			ID:               "glm-5.3",
			Name:             "My GLM",
			ContextWindow:    200_000,
			DefaultMaxTokens: 32_000,
			Pricing:          catwalk.Pricing{Input: 9},
			Reasoning: catwalk.Reasoning{
				Thinking:           catwalk.ThinkingToggleable,
				EffortLevels:       catwalk.NewEffortLevels("high"),
				DefaultEffortLevel: "high",
			},
		}}

		got := MergeCatalog(models, catalog)
		require.Equal(t, "My GLM", got[0].Name)
		require.Equal(t, int64(200_000), got[0].ContextWindow)
		require.Equal(t, int64(32_000), got[0].DefaultMaxTokens)
		require.InDelta(t, 9, got[0].Pricing.Input, 0)
		require.Equal(t, []catwalk.EffortLevel{{Value: "high", Display: "High"}}, got[0].Reasoning.EffortLevels)
		// Unset fields are still filled around the user's values.
		require.InDelta(t, 3, got[0].Pricing.Output, 0)
		require.Equal(t, catwalk.ThinkingToggleable, got[0].Reasoning.Thinking)
	})

	t.Run("leaves models the catalog does not know", func(t *testing.T) {
		t.Parallel()
		models := []catwalk.Model{{ID: "mystery", Name: "mystery"}}
		got := MergeCatalog(models, catalog)
		require.Equal(t, catwalk.Model{ID: "mystery", Name: "mystery"}, got[0])
	})

	t.Run("does not alias the catalog's reasoning levels", func(t *testing.T) {
		t.Parallel()
		cat := []catwalk.Model{{
			ID: "m",
			Reasoning: catwalk.Reasoning{
				EffortLevels: catwalk.NewEffortLevels("low", "high"),
			},
		}}
		got := MergeCatalog([]catwalk.Model{{ID: "m"}}, cat)
		got[0].Reasoning.EffortLevels[0].Value = "changed"
		require.Equal(t, "low", cat[0].Reasoning.EffortLevels[0].Value)
	})

	t.Run("no catalog is a no-op", func(t *testing.T) {
		t.Parallel()
		models := []catwalk.Model{{ID: "glm-5.3", Name: "glm-5.3"}}
		require.Equal(t, models, MergeCatalog(models, nil))
	})
}

func TestFetchProviderFeed(t *testing.T) {
	t.Parallel()

	t.Run("reads the provider document under the base URL", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/provider", r.URL.Path)
			require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id": "hyper",
				"models": [
					{"id": "glm-5.3", "name": "GLM 5.3", "context_window": 1000000,
					 "reasoning": {"thinking": "toggleable",
					               "effort_levels": [{"value": "low", "display": "Low"},
					                                 {"value": "high", "display": "High"},
					                                 {"value": "max", "display": "Max"}],
					               "default_effort_level": "high"}}
				]
			}`))
		}))
		defer srv.Close()

		cfg := Config{ID: "hyper", BaseURL: srv.URL + "/v1", APIKey: "test-key"}
		p, err := FetchProviderFeed(context.Background(), cfg, &mockResolver{})
		require.NoError(t, err)
		require.Len(t, p.Models, 1)
		require.Equal(t, "GLM 5.3", p.Models[0].Name)
		require.Equal(t, catwalk.NewEffortLevels("low", "high", "max"), p.Models[0].Reasoning.EffortLevels)
	})

	t.Run("reports a non-200 response", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.NotFoundHandler())
		defer srv.Close()

		_, err := FetchProviderFeed(context.Background(), Config{ID: "hyper", BaseURL: srv.URL}, &mockResolver{})
		require.ErrorContains(t, err, "404")
	})

	t.Run("reports an undecodable body", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`not json`))
		}))
		defer srv.Close()

		_, err := FetchProviderFeed(context.Background(), Config{ID: "hyper", BaseURL: srv.URL}, &mockResolver{})
		require.Error(t, err)
	})
}
