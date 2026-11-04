package discover

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/oauth"
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

// anthropicCatalog is the shape a Claude plan publishes: a listing whose
// capability tree says which effort levels a model takes, and how much context
// it holds. Nothing here is a field name Crush knows.
const anthropicCatalog = `{
  "data": [
    {"id": "claude-opus-5-5", "display_name": "Claude Opus 5.5",
     "max_input_tokens": 1000000, "max_tokens": 128000,
     "capabilities": {
       "effort": {"supported": true, "low": {"supported": true}, "medium": {"supported": true},
                  "high": {"supported": true}, "xhigh": {"supported": true}, "max": {"supported": true}},
       "thinking": {"supported": true}, "image_input": {"supported": true}}},
    {"id": "claude-opus-4-6", "display_name": "Claude Opus 4.6",
     "max_input_tokens": 1000000, "max_tokens": 128000,
     "capabilities": {
       "effort": {"supported": true, "low": {"supported": true}, "medium": {"supported": true},
                  "high": {"supported": true}, "xhigh": {"supported": false}, "max": {"supported": true}},
       "thinking": {"supported": true}, "image_input": {"supported": true}}},
    {"id": "claude-haiku-4-5", "display_name": "Claude Haiku 4.5",
     "max_input_tokens": 200000, "max_tokens": 64000,
     "capabilities": {
       "effort": {"supported": false, "low": {"supported": false}, "medium": {"supported": false},
                  "high": {"supported": false}, "xhigh": {"supported": false}, "max": {"supported": false}},
       "thinking": {"supported": true}, "image_input": {"supported": false}}}
  ]
}`

// catalogProgram is the mapping the plugin declares: pick the supported rungs
// of the effort ladder, in the ladder's order rather than the response's.
const catalogProgram = `def ladder: ["low", "medium", "high", "xhigh", "max"];
def supported($effort): ladder | map(select(($effort[.] // {})["supported"] == true));
def preferred($levels):
  if ($levels | index("medium")) then "medium"
  elif ($levels | index("high")) then "high"
  elif ($levels | index("low")) then "low"
  else "" end;
.data | map(
  (.capabilities // {}) as $caps
  | supported($caps.effort // {}) as $levels
  | {id, name: (.display_name // .id), context_window: (.max_input_tokens // 0),
     default_max_tokens: (.max_tokens // 0),
     reasoning: {
       thinking: (if (($caps.thinking // {})["supported"] == true) then "toggleable" else "never" end),
       effort_levels: ($levels | map({value: ., display: (. | ascii_upcase)})),
       default_effort_level: preferred($levels)},
     capabilities: {vision: (($caps.image_input // {})["supported"] == true), audio: false}})`

func TestCatalogReadsAProvidersOwnListing(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "Bearer at-123", r.Header.Get("Authorization"))
		// A listing can be gated behind a header the inference endpoint does
		// not want; it rides on the catalog call alone.
		require.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
		_, _ = w.Write([]byte(anthropicCatalog))
	}))
	defer server.Close()

	models, err := Catalog(context.Background(), CatalogConfig{
		ID:      "plan",
		BaseURL: server.URL,
		Spec: &oauth.CatalogSpec{
			URL:     "/v1/models",
			Program: catalogProgram,
			Headers: map[string]string{"anthropic-version": "2023-06-01"},
		},
		Bearer: "at-123",
	}, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 3)

	opus := models[0]
	require.Equal(t, "claude-opus-5-5", opus.ID)
	require.Equal(t, "Claude Opus 5.5", opus.Name)
	require.Equal(t, int64(1000000), opus.ContextWindow)
	require.Equal(t, int64(128000), opus.DefaultMaxTokens)
	require.True(t, opus.CanReason())
	require.True(t, opus.Capabilities.Vision)
	require.Equal(t, []string{"low", "medium", "high", "xhigh", "max"}, opus.ReasoningEffortLevels())
	require.Equal(t, "medium", opus.Reasoning.DefaultEffortLevel)

	// A rung the listing marks unsupported is left out, and the levels stay
	// in ladder order rather than the order the response listed them.
	require.Equal(t, []string{"low", "medium", "high", "max"}, models[1].ReasoningEffortLevels())

	// A model that takes no effort at all gets no default rather than a
	// made-up one, and reports the smaller window it actually has.
	haiku := models[2]
	require.Empty(t, haiku.ReasoningEffortLevels())
	require.Empty(t, haiku.Reasoning.DefaultEffortLevel)
	require.Equal(t, int64(200000), haiku.ContextWindow)
	require.False(t, haiku.Capabilities.Vision)
}

// A catalog fills in what a declared model leaves out and leaves alone what it
// sets, so a plugin can pin one field without restating the rest.
func TestCatalogKeepsDeclaredFields(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(anthropicCatalog))
	}))
	defer server.Close()

	models, err := Catalog(context.Background(), CatalogConfig{
		ID:      "plan",
		BaseURL: server.URL,
		Spec:    &oauth.CatalogSpec{URL: "/v1/models", Program: catalogProgram},
		Existing: []catwalk.Model{{
			ID:        "claude-opus-5-5",
			Name:      "Pinned Name",
			Reasoning: catwalk.Reasoning{DefaultEffortLevel: "high"},
		}},
	}, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 3, "declared models are not duplicated by the catalog")

	first := models[0]
	require.Equal(t, "Pinned Name", first.Name, "a declared name wins")
	require.Equal(t, "high", first.Reasoning.DefaultEffortLevel, "a declared effort wins")
	require.Equal(t, int64(1000000), first.ContextWindow, "the catalog fills what was left unset")
	require.Equal(t, []string{"low", "medium", "high", "xhigh", "max"}, first.ReasoningEffortLevels())
	require.True(t, first.Capabilities.Vision)

	// A declared model the catalog never lists survives as it was.
	models, err = Catalog(context.Background(), CatalogConfig{
		ID:       "plan",
		BaseURL:  server.URL,
		Spec:     &oauth.CatalogSpec{URL: "/v1/models", Program: catalogProgram},
		Existing: []catwalk.Model{{ID: "claude-from-the-future", Name: "Unreleased"}},
	}, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 4)
	require.Equal(t, "claude-from-the-future", models[0].ID)
	require.Equal(t, "Unreleased", models[0].Name)
}

// A program that maps one entry rather than the listing returns an object, and
// an entry with no id cannot be selected, so it is dropped.
func TestCatalogAcceptsSingleEntryAndDropsNameless(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"model": {"id": "only", "display_name": "Only"}}`))
	}))
	defer server.Close()

	models, err := Catalog(context.Background(), CatalogConfig{
		ID:      "plan",
		BaseURL: server.URL,
		Spec:    &oauth.CatalogSpec{URL: "/v1/models", Program: `.model | {id, name: .display_name}`},
	}, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "only", models[0].ID)

	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data": [{"display_name": "no id"}, {"id": "kept"}]}`))
	})
	models, err = Catalog(context.Background(), CatalogConfig{
		ID:      "plan",
		BaseURL: server.URL,
		Spec:    &oauth.CatalogSpec{URL: "/v1/models", Program: `.data | map({id, name: .display_name})`},
	}, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "kept", models[0].ID)
}

// Some gateways serve their catalog the way they serve their quota: over
// POST, answering any client but only to an empty JSON body.
func TestCatalogPostsWhenTheGatewayDemandsIt(t *testing.T) {
	t.Parallel()

	var sawMethod, sawBody, sawContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawMethod = r.Method
		sawContentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		sawBody = string(raw)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, `{"data": [{"id": "posted", "display_name": "Posted"}]}`)
	}))
	defer server.Close()

	models, err := Catalog(context.Background(), CatalogConfig{
		ID:      "plan",
		BaseURL: server.URL,
		Spec: &oauth.CatalogSpec{
			URL:     "/v1internal:fetchAvailableModels",
			Method:  "POST",
			Program: `.data | map({id, name: .display_name})`,
		},
	}, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "posted", models[0].ID)
	require.Equal(t, http.MethodPost, sawMethod)
	require.Equal(t, "application/json", sawContentType)
	require.JSONEq(t, "{}", sawBody, "a POST catalog defaults to an empty body")
}

func TestCatalogErrors(t *testing.T) {
	t.Parallel()

	_, err := Catalog(context.Background(), CatalogConfig{
		ID: "plan", Spec: &oauth.CatalogSpec{URL: "/v1/models"},
	}, &mockResolver{})
	require.ErrorContains(t, err, "names no url or program")

	_, err = Catalog(context.Background(), CatalogConfig{
		ID:   "plan",
		Spec: &oauth.CatalogSpec{URL: "/v1/models", Program: `if`},
	}, &mockResolver{})
	require.ErrorContains(t, err, "catalog program for provider plan")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "expired"}`))
	}))
	defer server.Close()

	_, err = Catalog(context.Background(), CatalogConfig{
		ID:      "plan",
		BaseURL: server.URL,
		Spec:    &oauth.CatalogSpec{URL: "/v1/models", Program: `.data`},
	}, &mockResolver{})
	require.ErrorContains(t, err, "401")
}
