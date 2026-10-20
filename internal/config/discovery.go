package config

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net/url"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/catwalk/pkg/embedded"
	"github.com/charmbracelet/crush/internal/discover"
)

// loadModelDiscoveryTimeout bounds each provider's model-discovery probe
// during config load. It is per provider, not a budget shared across the
// pass. Because the probes run concurrently, both designs start the same
// clock and behave identically today: a shared deadline drops every
// endpoint slower than the budget, not only the slowest one, so raising
// the budget is what recovers a slow remote gateway. The per-provider
// split is the structural guarantee for the day probing is staged or
// throttled, when a shared budget would start to starve later probes.
// Providers whose endpoints miss it are recorded on the store and can be
// retried later via ReloadModelDiscovery.
//
// It is sized for a remote gateway on a poor link (LiteLLM and Hyper have
// been measured at 10.5s and 6.7s respectively over hotel/boat wifi), not
// for a healthy LAN. Because the probes are concurrent, the worst case it
// adds to startup is one provider's timeout, not the sum.
// discoveryProbeTimeout bounds the inline probe of a provider that has no
// cache entry: startup waits this long once, then falls back to whatever
// the provider's user-configured models allow.
const discoveryProbeTimeout = 3 * time.Second

const loadModelDiscoveryTimeout = 15 * time.Second

// modelDiscoveryTimeout bounds each provider's probe on an interactive
// model-discovery reload. Like loadModelDiscoveryTimeout it is per
// provider.
// discoveryProbeTimeout bounds the inline probe of a provider that has no
// cache entry: startup waits this long once, then falls back to whatever
// the provider's user-configured models allow.
const modelDiscoveryTimeout = 15 * time.Second

// discoveryJob is one custom provider's model-discovery task: enabled,
// with a base URL, and either opted in through discover_models or lacking
// an explicit models list.
type discoveryJob struct {
	id           string
	cfg          discover.Config
	providerType string
}

// discoveredModels is the on-disk cache entry for one provider. BaseURL is
// stored alongside the models so pointing a provider elsewhere invalidates
// its entry.
type discoveredModels struct {
	BaseURL string          `json:"base_url"`
	Models  []catwalk.Model `json:"models"`
}

// providerWantsDiscovery reports whether a custom provider consents to
// model discovery. userModels is the provider's user-configured model
// list: at load time that is pc.Models itself; on reload it is the list
// the store recorded before load-time discovery merged endpoint models
// into pc.Models.
//
// The consent model: a provider with a curated, non-empty models list is
// never discovered unless it explicitly sets discover_models: true, and
// an empty list implies consent unless discover_models: false opts out.
// An interactive reload re-probes eligible providers but never widens
// this consent.
func providerWantsDiscovery(pc ProviderConfig, userModels []catwalk.Model) bool {
	if pc.AutoDiscoverModels != nil {
		return *pc.AutoDiscoverModels
	}
	return len(userModels) == 0
}

// knownProviderNameSet returns the set of provider IDs treated as known
// (non-custom) for discovery purposes. When disableDefaults is set every
// provider is custom — matching configureProviders, which skips all
// default providers under that option — so the set is empty.
func knownProviderNameSet(known []catwalk.Provider, disableDefaults bool) map[string]bool {
	names := make(map[string]bool, len(known))
	if disableDefaults {
		return names
	}
	for _, p := range known {
		names[string(p.ID)] = true
	}
	return names
}

// embeddedCatalog indexes the catwalk catalog bundled with Crush by
// provider ID. It is parsed once, on first use.
var embeddedCatalog = sync.OnceValue(func() map[string]catwalk.Provider {
	all := embedded.GetAll()
	index := make(map[string]catwalk.Provider, len(all))
	for _, p := range all {
		index[string(p.ID)] = p
	}
	return index
})

// catalogModels returns the catalog models describing a custom provider's
// discovered models, or nil when Crush has no catalog for it. A custom
// provider is matched to a catalog entry by ID, the same key the
// coordinator already uses for provider-specific request handling, and
// only when both speak the same API type: metadata for one wire protocol
// (Gemini's native API, say) does not describe another (its
// OpenAI-compatible endpoint).
//
// Catalogs come from catwalk's embedded snapshot: under
// disable_default_providers Crush never contacts catwalk, and filling in
// metadata is no reason to start. Providers with a live feed of their own
// (Hyper) register an ID-keyed enricher instead, which runs earlier in
// the pipeline and wins because both stages only fill zero fields.
func catalogModels(providerID string, providerType catwalk.Type) []catwalk.Model {
	p, ok := embeddedCatalog()[providerID]
	if !ok || p.Type != providerType {
		return nil
	}
	return p.Models
}

// discoverProviderModels runs model discovery concurrently for the custom
// (non-known) candidates that are eligible for it (see
// providerWantsDiscovery). Each candidate's Models field must hold only
// its user-configured models; those are passed to discovery as the
// existing set, so they always survive and take precedence over endpoint
// models. It returns the discovered model lists and the per-provider
// discovery errors, both keyed by provider ID. A provider that succeeds
// but reports no models gets a results entry with an empty list.
//
// perProviderTimeout bounds each provider's probe independently, so one
// slow endpoint cannot consume another's budget. The caller's ctx still
// cancels the whole pass.
func discoverProviderModels(
	ctx context.Context,
	candidates map[string]ProviderConfig,
	knownProviderNames map[string]bool,
	resolver VariableResolver,
	perProviderTimeout time.Duration,
) (map[string][]catwalk.Model, map[string]error) {
	results := make(map[string][]catwalk.Model)
	errs := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for id, pc := range candidates {
		if knownProviderNames[id] {
			continue
		}
		if pc.Disable || pc.BaseURL == "" {
			continue
		}
		if !providerWantsDiscovery(pc, pc.Models) {
			continue
		}

		providerID := cmp.Or(pc.ID, id)
		providerType := cmp.Or(pc.Type, catwalk.TypeOpenAICompat)
		cfg := discover.Config{
			ID:             providerID,
			BaseURL:        pc.BaseURL,
			APIKey:         pc.APIKey,
			ExtraHeaders:   pc.ExtraHeaders,
			APIType:        string(providerType),
			ExistingModels: pc.Models,
		}
		wg.Go(func() {
			// Each provider gets its own deadline. Enrichment and the
			// catalog lookup share it with the probe: together they are
			// this provider's budget.
			pctx, pcancel := context.WithTimeout(ctx, perProviderTimeout)
			defer pcancel()

			models, err := discover.DiscoverModels(pctx, cfg, resolver)
			if err != nil {
				slog.Warn("Model discovery failed", "provider", id, "error", err)
				mu.Lock()
				errs[id] = err
				mu.Unlock()
				return
			}
			if len(models) > 0 {
				if enricher := discover.GetEnricher(providerID, string(providerType)); enricher != nil {
					models, _ = enricher.EnrichModels(pctx, cfg, resolver, models)
				}
				models = discover.MergeCatalog(models, catalogModels(providerID, providerType))
			}
			mu.Lock()
			results[id] = models
			mu.Unlock()
		})
	}
	wg.Wait()
	return results, errs
}

// ReloadModelDiscovery re-runs model discovery for every custom provider
// eligible for it and merges the results into the in-memory config. It
// returns the number of models added across all providers.
//
// Eligibility matches load exactly (see providerWantsDiscovery): a reload
// never widens consent, it only re-probes providers load would have
// probed — including providers load dropped because discovery failed and
// they had no user-specified models (e.g. Crush started before Ollama),
// which are resurrected here when their endpoint comes back.
//
// Discovered models are intentionally not persisted to disk: like the
// discovery that runs during load, they are recomputed from the live
// provider endpoints each time. This lets a running Crush pick up models
// that appeared after startup (e.g. an `ollama pull`) without a restart.
//
// The network I/O runs without holding writeMu so config mutators (e.g.
// model selection) are never blocked behind slow provider endpoints. The
// merge goes through mutateInMemory and re-reads each provider fresh, so
// concurrent changes to other provider fields (e.g. an OAuth token
// refresh) are preserved.
//
// It returns an error when every eligible provider failed discovery so
// total failure is distinguishable from "no new models".
func (s *ConfigStore) ReloadModelDiscovery(ctx context.Context) (int, error) {
	cfg := s.Config()

	// Snapshot the discovery bookkeeping under a brief lock.
	s.writeMu.Lock()
	userModels := maps.Clone(s.userConfiguredModels)
	failed := maps.Clone(s.failedDiscoveryProviders)
	s.writeMu.Unlock()

	var disableDefaults bool
	if cfg.Options != nil {
		disableDefaults = cfg.Options.DisableDefaultProviders
	}
	known := knownProviderNameSet(s.knownProviders, disableDefaults)

	// Build the candidate set from the live providers plus the ones load
	// dropped after failed discovery. Models is seeded with the recorded
	// user-configured list (not the current, possibly discovery-merged
	// one) so re-discovery prunes endpoint-removed models while keeping
	// user-specified ones.
	candidates := make(map[string]ProviderConfig)
	for id, pc := range cfg.Providers.Seq2() {
		if um, ok := userModels[id]; ok {
			pc.Models = um
		}
		candidates[id] = pc
	}
	for id, pc := range failed {
		if _, ok := candidates[id]; !ok {
			candidates[id] = pc
		}
	}

	results, errs := discoverProviderModels(ctx, candidates, known, s.resolver, modelDiscoveryTimeout)
	if len(errs) > 0 && len(results) == 0 {
		return 0, fmt.Errorf("model discovery failed for all %d eligible providers", len(errs))
	}

	added := 0
	s.mutateInMemory(func(c *Config) {
		for id, models := range results {
			if len(models) == 0 {
				continue
			}
			// Re-read the provider fresh under writeMu and write only
			// the merged Models onto it, so concurrent changes to other
			// fields (OAuth tokens, header edits, …) are not clobbered.
			pc, ok := c.Providers.Get(id)
			if !ok {
				// Resurrect a provider dropped at load. writeMu is
				// held by mutateInMemory, so touching the failed map
				// here is safe.
				fp, wasFailed := s.failedDiscoveryProviders[id]
				if !wasFailed {
					continue
				}
				fp.Models = models
				c.Providers.Set(id, fp)
				delete(s.failedDiscoveryProviders, id)
				added += len(models)
				slog.Info("Recovered provider via model discovery",
					"provider", id, "count", len(models))
				continue
			}
			newCount := 0
			have := make(map[string]bool, len(pc.Models))
			for _, m := range pc.Models {
				have[m.ID] = true
			}
			for _, m := range models {
				if !have[m.ID] {
					newCount++
				}
			}
			pc.Models = models
			c.Providers.Set(id, pc)
			if newCount > 0 {
				added += newCount
				slog.Info("Reloaded models for provider",
					"provider", id, "added", newCount, "count", len(models))
			}
		}
	})
	return added, nil
}

// discoveryJobs returns the custom providers that need model discovery:
// enabled, with a base URL, and either opted in through discover_models or
// lacking an explicit models list.
func discoveryJobs(c *Config, known map[string]bool) []discoveryJob {
	var jobs []discoveryJob
	for id, pc := range c.Providers.Seq2() {
		if known[id] || pc.Disable || pc.BaseURL == "" {
			continue
		}
		wanted := len(pc.Models) == 0
		if pc.AutoDiscoverModels != nil {
			wanted = *pc.AutoDiscoverModels
		}
		if !wanted {
			continue
		}
		jobs = append(jobs, discoveryJob{
			id: id,
			cfg: discover.Config{
				ID:           cmp.Or(pc.ID, id),
				BaseURL:      pc.BaseURL,
				APIKey:       pc.APIKey,
				ExtraHeaders: pc.ExtraHeaders,
			},
			providerType: string(cmp.Or(pc.Type, catwalk.TypeOpenAICompat)),
		})
	}
	return jobs
}

// resolveDiscoveredModels returns the discovered models for each job. Jobs
// with a cache entry use it without touching the network and are recorded
// so RefreshDiscoveredModels can update them later; the rest are probed
// inline, bounded by discoveryProbeTimeout and ctx.
func (s *ConfigStore) resolveDiscoveredModels(ctx context.Context, jobs []discoveryJob, resolver VariableResolver) map[string][]catwalk.Model {
	found := make(map[string][]catwalk.Model, len(jobs))
	var cached, misses []discoveryJob
	for _, job := range jobs {
		if models, ok := s.cachedModels(job); ok {
			found[job.id] = models
			cached = append(cached, job)
			continue
		}
		misses = append(misses, job)
	}
	s.cachedDiscovery = cached

	probeCtx, cancel := context.WithTimeout(ctx, discoveryProbeTimeout)
	defer cancel()
	maps.Copy(found, s.probeAndCache(probeCtx, misses, resolver))
	return found
}

// RefreshDiscoveredModels re-probes every provider whose models came from
// the discovery cache, and reloads the config when any of them changed. It
// reports whether a reload happened. Meant to run in the background after
// startup.
func (s *ConfigStore) RefreshDiscoveredModels(ctx context.Context) (bool, error) {
	s.writeMu.RLock()
	jobs := s.cachedDiscovery
	resolver := s.resolver
	s.writeMu.RUnlock()

	if len(jobs) == 0 {
		return false, nil
	}

	before := make(map[string][]catwalk.Model, len(jobs))
	for _, job := range jobs {
		before[job.id], _ = s.cachedModels(job)
	}
	changed := false
	for id, fresh := range s.probeAndCache(ctx, jobs, resolver) {
		changed = changed || !reflect.DeepEqual(before[id], fresh)
	}
	if !changed {
		return false, nil
	}
	return true, s.ReloadFromDisk(ctx)
}

// probeAndCache discovers models for jobs concurrently and caches every
// non-empty result. Failures are logged and left out of the returned map.
func (s *ConfigStore) probeAndCache(ctx context.Context, jobs []discoveryJob, resolver VariableResolver) map[string][]catwalk.Model {
	// The probe goes through the fork's discoverProviderModels, which adds
	// the per-provider deadline, the ID-keyed enrichers, and the catalog
	// merge that the plain probe lacks.
	byID := make(map[string]discoveryJob, len(jobs))
	candidates := make(map[string]ProviderConfig, len(jobs))
	for _, job := range jobs {
		byID[job.id] = job
		candidates[job.id] = ProviderConfig{
			ID:           job.cfg.ID,
			BaseURL:      job.cfg.BaseURL,
			APIKey:       job.cfg.APIKey,
			ExtraHeaders: job.cfg.ExtraHeaders,
			Type:         catwalk.Type(job.providerType),
		}
	}
	known := map[string]bool{}
	models, _ := discoverProviderModels(ctx, candidates, known, resolver, modelDiscoveryTimeout)
	for id, result := range models {
		if len(result) > 0 {
			slog.Info("Discovered models for provider", "provider", id, "count", len(result))
			s.cacheModels(byID[id], result)
		}
	}
	return models
}

// discoveryCachePath is empty when the store has no data directory, which
// disables the cache.
func (s *ConfigStore) discoveryCachePath(providerID string) string {
	if s.globalDataPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(s.globalDataPath), "discovered_models", url.PathEscape(providerID)+".json")
}

func (s *ConfigStore) cachedModels(job discoveryJob) ([]catwalk.Model, bool) {
	path := s.discoveryCachePath(job.id)
	if path == "" {
		return nil, false
	}
	entry, _, err := newCache[discoveredModels](path).Get()
	if err != nil || entry.BaseURL != job.cfg.BaseURL || len(entry.Models) == 0 {
		return nil, false
	}
	return entry.Models, true
}

func (s *ConfigStore) cacheModels(job discoveryJob, models []catwalk.Model) {
	path := s.discoveryCachePath(job.id)
	if path == "" {
		return
	}
	entry := discoveredModels{BaseURL: job.cfg.BaseURL, Models: models}
	if err := newCache[discoveredModels](path).Store(entry); err != nil {
		slog.Warn("Failed to cache discovered models", "provider", job.id, "error", err)
	}
}

// mergeModels appends the discovered models whose IDs the config does not
// already list; models from the config win.
func mergeModels(configured, discovered []catwalk.Model) []catwalk.Model {
	listed := make(map[string]bool, len(configured))
	for _, m := range configured {
		listed[m.ID] = true
	}
	merged := slices.Clone(configured)
	for _, m := range discovered {
		if !listed[m.ID] {
			merged = append(merged, m)
		}
	}
	return merged
}
