package config

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/bytedance/sonic"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/invopop/jsonschema"
)

const (
	appName              = "crush"
	defaultDataDirectory = ".crush"
	defaultInitializeAs  = "AGENTS.md"
)

var defaultContextPaths = []string{
	".github/copilot-instructions.md",
	".cursorrules",
	".cursor/rules/",
	"CLAUDE.md",
	"CLAUDE.local.md",
	"GEMINI.md",
	"gemini.md",
	"crush.md",
	"crush.local.md",
	"Crush.md",
	"Crush.local.md",
	"CRUSH.md",
	"CRUSH.local.md",
	"AGENTS.md",
	"agents.md",
	"Agents.md",
}

type SelectedModelType string

// String returns the string representation of the [SelectedModelType].
func (s SelectedModelType) String() string {
	return string(s)
}

const (
	SelectedModelTypeLarge SelectedModelType = "large"
	SelectedModelTypeSmall SelectedModelType = "small"
	// SelectedModelTypeAutoMode selects the auto-mode classifier model.
	// It is stored under auto_mode.classifier, not the models map.
	SelectedModelTypeAutoMode SelectedModelType = "auto_mode"
)

const (
	AgentCoder string = "coder"
	AgentPlan  string = "plan"
	AgentTask  string = "task"
)

type SelectedModel struct {
	Temperature      *float64       `json:"temperature,omitempty" jsonschema:"description=Sampling temperature,minimum=0,maximum=1,example=0.7"`
	TopP             *float64       `json:"top_p,omitempty" jsonschema:"description=Top-p (nucleus) sampling parameter,minimum=0,maximum=1,example=0.9"`
	TopK             *int64         `json:"top_k,omitempty" jsonschema:"description=Top-k sampling parameter"`
	FrequencyPenalty *float64       `json:"frequency_penalty,omitempty" jsonschema:"description=Frequency penalty to reduce repetition"`
	PresencePenalty  *float64       `json:"presence_penalty,omitempty" jsonschema:"description=Presence penalty to increase topic diversity"`
	ProviderOptions  map[string]any `json:"provider_options,omitempty" jsonschema:"description=Additional provider-specific options for the model"`
	Model            string         `json:"model" jsonschema:"required,description=The model ID as used by the provider API,example=gpt-4o"`
	Provider         string         `json:"provider" jsonschema:"required,description=The model provider ID that matches a key in the providers config,example=openai"`
	ReasoningEffort  string         `json:"reasoning_effort,omitempty" jsonschema:"description=Reasoning effort level for OpenAI models that support it,enum=low,enum=medium,enum=high"`
	MaxTokens        int64          `json:"max_tokens,omitempty" jsonschema:"description=Maximum number of tokens for model responses,maximum=200000,example=4096"`
	Think            bool           `json:"think,omitempty" jsonschema:"description=Enable thinking mode for Anthropic models that support reasoning"`
}

type ProviderConfig struct {
	OAuthToken         *oauth.Token      `json:"oauth,omitempty" jsonschema:"description=OAuth2 token for authentication with the provider"`
	AutoDiscoverModels *bool             `json:"discover_models,omitempty" jsonschema:"description=Auto-discover models from /v1/models endpoint. When true with existing models they are merged (yours win),default=true"`
	ExtraParams        map[string]string `json:"-"`
	ProviderOptions    map[string]any    `json:"provider_options,omitempty" jsonschema:"description=Additional provider-specific options for this provider"`
	ExtraBody          map[string]any    `json:"extra_body,omitempty" jsonschema:"description=Additional fields to include in request bodies\\, only works with openai-compatible providers"`
	ExtraHeaders       map[string]string `json:"extra_headers,omitempty" jsonschema:"description=Additional HTTP headers to send with requests"`
	APIKey             string            `json:"api_key,omitempty" jsonschema:"description=API key for authentication with the provider,example=$OPENAI_API_KEY"`
	SystemPromptPrefix string            `json:"system_prompt_prefix,omitempty" jsonschema:"description=Custom prefix to add to system prompts for this provider"`
	APIKeyTemplate     string            `json:"-"`
	ID                 string            `json:"id,omitempty" jsonschema:"description=Unique identifier for the provider,example=openai"`
	Type               catwalk.Type      `json:"type,omitempty" jsonschema:"description=Provider type that determines the API format,default=openai"`
	BaseURL            string            `json:"base_url,omitempty" jsonschema:"description=Base URL for the provider's API,format=uri,example=https://api.openai.com/v1"`
	AWSAuthRefresh     string            `json:"aws_auth_refresh,omitempty" jsonschema:"description=Shell command to run when AWS credentials expire (Bedrock only)."`
	Name               string            `json:"name,omitempty" jsonschema:"description=Human-readable name for the provider,example=OpenAI"`
	Models             []catwalk.Model   `json:"models,omitempty" jsonschema:"description=List of models available from this provider"`
	ChatGPTModels      []catwalk.Model   `json:"chatgpt_models,omitempty" jsonschema:"-"`
	GrokModels         []catwalk.Model   `json:"grok_models,omitempty" jsonschema:"-"`
	Disable            bool              `json:"disable,omitempty" jsonschema:"description=Whether this provider is disabled,default=false"`
	FlatRate           bool              `json:"flat_rate,omitempty" jsonschema:"description=Flat-rate mode for this provider"`
}

// ToProvider converts the [ProviderConfig] to a [catwalk.Provider].
func (c *ProviderConfig) ToProvider() catwalk.Provider {
	// Convert config provider to provider.Provider format
	provider := catwalk.Provider{
		Name:   c.Name,
		ID:     catwalk.InferenceProvider(c.ID),
		Models: make([]catwalk.Model, len(c.Models)),
	}

	// Convert models
	for i, model := range c.Models {
		provider.Models[i] = catwalk.Model{
			ID:                     model.ID,
			Name:                   model.Name,
			CostPer1MIn:            model.CostPer1MIn,
			CostPer1MOut:           model.CostPer1MOut,
			CostPer1MInCached:      model.CostPer1MInCached,
			CostPer1MOutCached:     model.CostPer1MOutCached,
			ContextWindow:          model.ContextWindow,
			DefaultMaxTokens:       model.DefaultMaxTokens,
			CanReason:              model.CanReason,
			ReasoningLevels:        model.ReasoningLevels,
			DefaultReasoningEffort: model.DefaultReasoningEffort,
			SupportsImages:         model.SupportsImages,
		}
	}

	return provider
}

func (c *ProviderConfig) SetupGitHubCopilot() {
	maps.Copy(c.ExtraHeaders, copilot.Headers())
}

// HasAPIKey reports whether the provider's api_key resolves to a usable
// credential. The stored value is often an unresolved template like
// $OPENAI_API_KEY, which is not a credential until the variable exists.
func (c *ProviderConfig) HasAPIKey(resolver VariableResolver) bool {
	if c.APIKey == "" {
		return false
	}
	v, err := resolver.ResolveValue(c.APIKey)
	return err == nil && v != ""
}

type MCPType string

const (
	MCPStdio MCPType = "stdio"
	MCPSSE   MCPType = "sse"
	MCPHttp  MCPType = "http"
)

type MCPConfig struct {
	Sessionless           *bool             `json:"sessionless,omitempty" jsonschema:"description=Mark a sessionless MCP server (no Mcp-Session-Id) so Crush skips the subscriptions/listen stream it would otherwise reject. Leave unset to auto-detect known sessionless servers (e.g. GitHub MCP),default=false"`
	OAuthToken            *oauth.Token      `json:"oauth_token,omitempty" jsonschema:"-"`
	Env                   map[string]string `json:"env,omitempty" jsonschema:"description=Environment variables to set for the MCP server"`
	ChannelReply          *MCPChannelReply  `json:"channel_reply,omitempty" jsonschema:"description=Automatically route replies for turns originating from this channel back through one of the server's tools"`
	Headers               map[string]string `json:"headers,omitempty" jsonschema:"description=HTTP headers for HTTP/SSE MCP servers"`
	Lazy                  *bool             `json:"lazy,omitempty" jsonschema:"description=Keep this server's tools out of the model context until the agent loads them with mcp_search,default=true"`
	URL                   string            `json:"url,omitempty" jsonschema:"description=URL for HTTP or SSE MCP servers,format=uri,example=http://localhost:3000/mcp"`
	OAuthClientID         string            `json:"oauth_client_id,omitempty" jsonschema:"description=Pre-registered OAuth client ID for servers without dynamic client registration"`
	Command               string            `json:"command,omitempty" jsonschema:"description=Command to execute for stdio MCP servers,example=npx"`
	OAuthClientIDMetadata string            `json:"oauth_client_id_metadata,omitempty" jsonschema:"description=URL of an OAuth Client ID Metadata Document (SEP-991) sent as the client_id for servers that advertise client_id_metadata_document_supported but have no registration endpoint"`
	OAuthClientSecret     string            `json:"oauth_client_secret,omitempty" jsonschema:"description=Pre-registered OAuth client secret paired with oauth_client_id"`
	Type                  MCPType           `json:"type" jsonschema:"required,description=Type of MCP connection,enum=stdio,enum=sse,enum=http,default=stdio"`
	Args                  []string          `json:"args,omitempty" jsonschema:"description=Arguments to pass to the MCP server command"`
	DisabledTools         []string          `json:"disabled_tools,omitempty" jsonschema:"description=List of tools from this MCP server to disable,example=get-library-doc"`
	EnabledTools          []string          `json:"enabled_tools,omitempty" jsonschema:"description=Allow list of tools from this MCP server,example=get-library-doc"`
	Timeout               int               `json:"timeout,omitempty" jsonschema:"description=Timeout in seconds for MCP server connections,default=10,example=30,example=60,example=120"`
	OAuthCallbackPort     int               `json:"oauth_callback_port,omitempty" jsonschema:"description=Fixed localhost port for the OAuth callback, required by providers that enforce exact-match redirect URIs"`
	OAuth                 bool              `json:"oauth,omitempty" jsonschema:"description=Enable OAuth 2.1 authorization flow for this MCP server (HTTP transport only),default=false"`
	ChannelEnabled        bool              `json:"channel_enabled,omitempty" jsonschema:"description=Enable this MCP server as a channel (equivalent to --channels),default=false"`
	Disabled              bool              `json:"disabled,omitempty" jsonschema:"description=Whether this MCP server is disabled,default=false"`
}

// MCPChannelReply configures deterministic reply routing for an MCP server
// acting as a channel: which of the server's tools deliver a reply for
// direct and group pushes, and how the reply text and target are mapped
// onto tool arguments.
type MCPChannelReply struct {
	// User routes replies to direct (person-to-person) channel pushes.
	User *MCPChannelReplyRoute `json:"user,omitempty" jsonschema:"description=Reply route for direct messages"`
	// Group routes replies to group channel pushes. It is preferred over
	// User when the push carries the group route's meta attribute.
	Group *MCPChannelReplyRoute `json:"group,omitempty" jsonschema:"description=Reply route for group messages"`
	// MessageParam is the tool argument that receives the reply text.
	MessageParam string `json:"message_param,omitempty" jsonschema:"description=Tool argument name that receives the reply text,default=message"`
	// SuppressTools lists additional tool names (beyond the two route
	// tools) that count as the model having already replied on the channel
	// during the turn, e.g. an operator-shortcut send tool.
	SuppressTools []string `json:"suppress_tools,omitempty" jsonschema:"description=Additional tool names that suppress the automatic reply when the model already called one of them during the turn,example=send"`
}

// MCPChannelReplyRoute maps one kind of inbound channel push onto the MCP
// tool call that delivers a reply to it.
type MCPChannelReplyRoute struct {
	// Tool is the MCP tool (bare name, without the mcp_<server>_ prefix)
	// invoked to deliver the reply.
	Tool string `json:"tool" jsonschema:"required,description=MCP tool name that sends the reply,example=send_message_to_user"`
	// TargetParam is the tool argument that receives the reply target
	// (recipient or group ID).
	TargetParam string `json:"target_param" jsonschema:"required,description=Tool argument name that receives the reply target,example=user_id"`
	// TargetMeta is the <channel> meta attribute whose value identifies
	// the reply target. Defaults to "sender" for the user route and
	// "group" for the group route.
	TargetMeta string `json:"target_meta,omitempty" jsonschema:"description=Channel meta attribute carrying the reply target; defaults to sender (user route) or group (group route)"`
}

// isOrphanedToken reports whether this entry is a leftover OAuth token
// with no real server config.
func (m MCPConfig) isOrphanedToken() bool {
	return m.Type == "" && m.Command == "" && m.URL == "" && m.OAuthToken != nil
}

type LSPConfig struct {
	Env         map[string]string `json:"env,omitempty" jsonschema:"description=Environment variables to set to the LSP server command"`
	InitOptions map[string]any    `json:"init_options,omitempty" jsonschema:"description=Initialization options passed to the LSP server during initialize request"`
	Options     map[string]any    `json:"options,omitempty" jsonschema:"description=LSP server-specific settings passed during initialization"`
	Command     string            `json:"command,omitempty" jsonschema:"description=Command to execute for the LSP server,example=gopls"`
	Args        []string          `json:"args,omitempty" jsonschema:"description=Arguments to pass to the LSP server command"`
	FileTypes   []string          `json:"filetypes,omitempty" jsonschema:"description=File types this LSP server handles,example=go,example=mod,example=rs,example=c,example=js,example=ts"`
	RootMarkers []string          `json:"root_markers,omitempty" jsonschema:"description=Files or directories that indicate the project root,example=go.mod,example=package.json,example=Cargo.toml"`
	Timeout     int               `json:"timeout,omitempty" jsonschema:"description=Timeout in seconds for LSP server initialization,default=30,example=60,example=120"`
	Disabled    bool              `json:"disabled,omitempty" jsonschema:"description=Whether this LSP server is disabled,default=false"`
}

type TUIOptions struct {
	Completions      Completions `json:"completions,omitzero" jsonschema:"description=Completions UI options"`
	Transparent      *bool       `json:"transparent,omitempty" jsonschema:"description=Enable transparent background for the TUI interface,default=false"`
	Mouse            *bool       `json:"mouse,omitempty" jsonschema:"description=Enable terminal mouse capture for selection\\, clicks\\, and scrolling in the TUI. Disable to let the terminal emulator or tmux handle text selection and copy/paste,default=true"`
	ScrollSpeed      *float64    `json:"scroll_speed,omitempty" jsonschema:"description=Multiplier applied to mouse wheel scroll distance in the TUI,default=1"`
	NerdFonts        *bool       `json:"nerd_fonts,omitempty" jsonschema:"description=Override Nerd Font glyph detection for the TUI: true forces glyphs on, false forces them off, and unset keeps automatic detection"`
	DiffMode         string      `json:"diff_mode,omitempty" jsonschema:"description=Diff mode for the TUI interface,enum=unified,enum=split"`
	ActiveTheme      string      `json:"active_theme,omitempty" jsonschema:"description=Name of the currently active theme,default=charmtone-panther,example=charmtone-panther,example=gruvbox-dark"`
	Scrollbar        string      `json:"scrollbar,omitempty" jsonschema:"description=Chat scrollbar visibility,enum=default,enum=always,enum=never,default=default"`
	WorkingDirFormat string      `json:"working_dir_format,omitempty" jsonschema:"description=Format for the working directory shown in the header. Supported placeholders: {cwd} for the path\\, {user} for the current user\\, {host} for the hostname.,default={user}@{host}:{cwd},example={cwd},example={host}:{cwd}"`
	ExitBanner       ExitBanner  `json:"exit_banner,omitempty" jsonschema:"description=Exit banner style after quitting Crush,enum=default,enum=compact,enum=none,default=default"`
	CompactMode      bool        `json:"compact_mode,omitempty" jsonschema:"description=Enable compact mode for the TUI interface,default=false"`
}

// IsTransparent reports whether the TUI draws a transparent background. The
// nil receiver and the unset pointer both mean opaque, so callers can ask
// without unwrapping either.
func (t *TUIOptions) IsTransparent() bool {
	return t != nil && t.Transparent != nil && *t.Transparent
}

// UnmarshalJSON tolerates the legacy string form of the "theme" field.
// Older Crush builds stored the selected theme as `"theme":
// "gruvbox-dark"`. When that string is encountered it is promoted to the
// active theme name so existing configs keep loading.
func (t *TUIOptions) UnmarshalJSON(data []byte) error {
	type tuiOptionsAlias TUIOptions
	var loose struct {
		tuiOptionsAlias
		Theme json.RawMessage `json:"theme"`
	}
	if err := sonic.Unmarshal(data, &loose); err != nil {
		return err
	}
	*t = TUIOptions(loose.tuiOptionsAlias)
	if len(loose.Theme) == 0 || string(loose.Theme) == "null" {
		return nil
	}

	var legacyName string
	if err := sonic.Unmarshal(loose.Theme, &legacyName); err != nil {
		// Legacy inline theme maps are ignored now that files are the only
		// palette source, but they must remain loadable during migration.
		var legacyThemes map[string]json.RawMessage
		return sonic.Unmarshal(loose.Theme, &legacyThemes)
	}
	if legacyName != "" && t.ActiveTheme == "" {
		t.ActiveTheme = legacyName
	}
	return nil
}

// Completions defines options for the completions UI.
type Completions struct {
	MaxDepth *int `json:"max_depth,omitempty" jsonschema:"description=Maximum depth for the ls tool,default=0,example=10"`
	MaxItems *int `json:"max_items,omitempty" jsonschema:"description=Maximum number of items to return for the ls tool,default=1000,example=100"`
}

// Limits returns the configured completion limits. Zero means the user has not
// pinned that limit, and callers fall back to their own built-in cap.
func (c Completions) Limits() (depth, items int) {
	return ptrValOr(c.MaxDepth, 0), ptrValOr(c.MaxItems, 0)
}

// Diff mode options.
const (
	DiffModeUnified = "unified" // Inline unified diffs
	DiffModeSplit   = "split"   // Side-by-side diffs
)

// Scrollbar visibility options.
const (
	ScrollbarDefault = "default" // Auto-hide after 2 seconds
	ScrollbarAlways  = "always"  // Always show when content exceeds viewport
	ScrollbarNever   = "never"   // Never show scrollbar
)

// ExitBanner selects what Crush prints after the TUI exits.
type ExitBanner string

const (
	// ExitBannerDefault renders the full ASCII art logo with padding. It is
	// also what the zero value and any unrecognized value fall back to.
	ExitBannerDefault ExitBanner = "default"
	// ExitBannerCompact renders only the session and resume lines, with no
	// logo and no padding. With no active session it renders nothing at all,
	// so Crush exits silently.
	ExitBannerCompact ExitBanner = "compact"
	// ExitBannerNone renders nothing.
	ExitBannerNone ExitBanner = "none"
)

type Permissions struct {
	AllowedTools []string `json:"allowed_tools,omitempty" jsonschema:"description=List of tools that don't require permission prompts,example=bash,example=view"`
}

// AutoModeConfig configures Crush's native auto mode: a safety
// classifier that runs in-process before permission prompts and grants,
// denies, or defers to the prompt based on static rules and an LLM
// classifier (see internal/automode). All fields are optional; unset
// fields use the tested defaults.
//
// User-level only: the loader ignores an auto_mode section in a
// project-level config. A repository must not be able to grant itself
// auto mode.
type AutoModeConfig struct {
	Enabled bool `json:"enabled,omitempty" jsonschema:"description=Enable native auto mode. Intended to be set in user-level config only: a repository should not be able to grant itself auto mode,default=false"`
	// Classifier selects the LLM used for classification. When
	// Provider and Model are set they reference an existing provider
	// and model in this config (e.g. local-llama/gemma4-e4b).
	// Alternatively, an OpenAI-compatible classifier endpoint can be
	// pointed at directly with BaseURL (and optionally APIKey)
	// without a provider entry. Without any classifier selection auto
	// mode runs rules-only: static allow/deny, ambiguities escalate
	// to the prompt.
	Classifier *AutoModeClassifier `json:"classifier,omitempty" jsonschema:"description=Classifier model selection"`
	// MaxTokens caps the classifier completion length. Reasoning models
	// need a generous budget. Default 1024.
	MaxTokens int `json:"max_tokens,omitempty" jsonschema:"description=Max completion tokens for the classifier,default=1024"`
	// TimeoutSeconds bounds each classification (both LLM stages).
	// Default 60.
	TimeoutSeconds int `json:"timeout_seconds,omitempty" jsonschema:"description=Per-evaluation timeout in seconds,default=60"`
	// FailOpen allows on classifier failure instead of escalating to
	// the prompt. Default false.
	FailOpen bool `json:"fail_open,omitempty" jsonschema:"description=Allow on classifier failure instead of escalating,default=false"`
	// MaxConsecutiveDenials / MaxTotalDenials pause classification when
	// exceeded, escalating to the prompt. Defaults 3 / 20.
	MaxConsecutiveDenials int `json:"max_consecutive_denials,omitempty" jsonschema:"description=Consecutive denials before auto mode pauses,default=3"`
	MaxTotalDenials       int `json:"max_total_denials,omitempty" jsonschema:"description=Total denials before auto mode pauses,default=20"`
	// Environment is prose describing trusted repos, domains, buckets,
	// and services, injected into the classifier prompts.
	Environment []string `json:"environment,omitempty" jsonschema:"description=Prose describing the trusted environment, injected into classifier prompts"`
	// PromptStage1File / PromptStage2File override the built-in
	// classifier prompts with template files. Placeholders: {{cwd}},
	// {{tool}}, {{params}}, {{environment}}, {{transcript}}.
	PromptStage1File string `json:"prompt_stage1_file,omitempty" jsonschema:"description=Optional stage 1 (fast filter) prompt template file"`
	PromptStage2File string `json:"prompt_stage2_file,omitempty" jsonschema:"description=Optional stage 2 (chain of thought) prompt template file"`
	// TranscriptMaxChars caps the reasoning-blind transcript excerpt
	// passed to the classifier. Default 24000.
	TranscriptMaxChars int `json:"transcript_max_chars,omitempty" jsonschema:"description=Max characters of transcript passed to the classifier,default=24000"`
}

// Validate checks the auto-mode knobs: 0 keeps a documented default, a
// positive value configures it, and a negative value is a load error
// that names its path. The same rule as the todo enforcement and
// dispatch knobs.
func (a *AutoModeConfig) Validate(path string) error {
	for _, knob := range []struct {
		field string
		value int
	}{
		{"max_tokens", a.MaxTokens},
		{"timeout_seconds", a.TimeoutSeconds},
		{"transcript_max_chars", a.TranscriptMaxChars},
		{"max_consecutive_denials", a.MaxConsecutiveDenials},
		{"max_total_denials", a.MaxTotalDenials},
	} {
		if knob.value < 0 {
			return fmt.Errorf("%s.%s must not be negative (got %d)", path, knob.field, knob.value)
		}
	}
	return nil
}

// ResolvedAutoMode returns the effective auto-mode configuration with
// defaults applied, and whether auto mode is enabled at all. The second
// return value is false when no auto_mode block is configured.
func (c *Config) ResolvedAutoMode() (AutoModeConfig, bool) {
	if c.AutoMode == nil {
		return AutoModeConfig{
			Enabled:               false,
			FailOpen:              false,
			MaxTokens:             1024,
			TimeoutSeconds:        60,
			MaxConsecutiveDenials: 3,
			MaxTotalDenials:       20,
			TranscriptMaxChars:    24000,
		}, false
	}
	e := *c.AutoMode
	if e.MaxTokens <= 0 {
		e.MaxTokens = 1024
	}
	if e.TimeoutSeconds <= 0 {
		e.TimeoutSeconds = 60
	}
	if e.MaxConsecutiveDenials <= 0 {
		e.MaxConsecutiveDenials = 3
	}
	if e.MaxTotalDenials <= 0 {
		e.MaxTotalDenials = 20
	}
	if e.TranscriptMaxChars <= 0 {
		e.TranscriptMaxChars = 24000
	}
	return e, e.Enabled
}

// AutoModeClassifierModel resolves the classifier's model selection: the
// explicit auto_mode.classifier provider/model pair when set, then the
// small slot, then the large slot — the same fallback order upstream PR
// #3874 documents. When it reports false, classification runs
// rules-only. A classifier configured only with a standalone BaseURL
// (no provider entry) is not resolved here; the agent layer builds that
// model directly from the endpoint fields.
func (c *Config) AutoModeClassifierModel() (SelectedModel, bool) {
	if c.AutoMode != nil && c.AutoMode.Classifier != nil {
		cl := *c.AutoMode.Classifier
		if cl.Provider != "" && cl.Model != "" {
			return SelectedModel{Provider: cl.Provider, Model: cl.Model}, true
		}
	}
	if sel, ok := c.Models[SelectedModelTypeSmall]; ok {
		return sel, true
	}
	if sel, ok := c.Models[SelectedModelTypeLarge]; ok {
		return sel, true
	}
	return SelectedModel{}, false
}

// AutoModeClassifier selects the classifier model: either by referencing
// a provider and model id already declared in the providers config, or
// by pointing at a standalone OpenAI-compatible endpoint with BaseURL.
type AutoModeClassifier struct {
	Provider string `json:"provider,omitempty" jsonschema:"description=Provider id from the providers config,example=local-llama"`
	Model    string `json:"model,omitempty" jsonschema:"description=Model id on that provider,example=gemma4-e4b"`
	BaseURL  string `json:"base_url,omitempty" jsonschema:"description=Standalone OpenAI-compatible classifier endpoint, bypassing the providers config"`
	APIKey   string `json:"api_key,omitempty" jsonschema:"description=API key for the standalone classifier endpoint. Supports shell expansion of $VAR and $(command)."`
}

type TrailerStyle string

const (
	TrailerStyleNone         TrailerStyle = "none"
	TrailerStyleCoAuthoredBy TrailerStyle = "co-authored-by"
	TrailerStyleAssistedBy   TrailerStyle = "assisted-by"
)

type Attribution struct {
	CoAuthoredBy  *bool        `json:"co_authored_by,omitempty" jsonschema:"description=Deprecated: use trailer_style instead"`
	TrailerStyle  TrailerStyle `json:"trailer_style,omitempty" jsonschema:"description=Style of attribution trailer to add to commits,enum=none,enum=co-authored-by,enum=assisted-by,default=assisted-by"`
	GeneratedWith bool         `json:"generated_with,omitempty" jsonschema:"description=Add Generated with Crush line to commit messages and issues and PRs,default=true"`
}

// JSONSchemaExtend marks the co_authored_by field as deprecated in the schema.
func (Attribution) JSONSchemaExtend(schema *jsonschema.Schema) {
	if schema.Properties != nil {
		if prop, ok := schema.Properties.Get("co_authored_by"); ok {
			prop.Deprecated = true
		}
	}
}

type Options struct {
	RequestTimeout            *int                   `json:"request_timeout,omitempty" jsonschema:"description=Timeout in seconds for each LLM API request. Streaming responses are aborted only after this much inactivity\\, so slow but active streams are never killed. 0 disables it\\, negative values are invalid.,default=60,example=120,example=300,example=0"`
	Attribution               *Attribution           `json:"attribution,omitempty" jsonschema:"description=Attribution settings for generated content"`
	A2A                       *A2AOptions            `json:"a2a,omitempty" jsonschema:"description=A2A host options: an optional TLS-only TCP listener beside the per-process unix socket"`
	TUI                       *TUIOptions            `json:"tui,omitempty" jsonschema:"description=Terminal user interface options"`
	AutoLSP                   *bool                  `json:"auto_lsp,omitempty" jsonschema:"description=Automatically setup LSPs based on root markers,default=true"`
	Dispatch                  *DispatchOptions       `json:"dispatch,omitempty" jsonschema:"description=Dispatched agent options"`
	TodoEnforcement           *TodoEnforcementConfig `json:"todo_enforcement,omitempty" jsonschema:"description=Proactive todo enforcement: inject a nudge when an agent works without a todo list, and optionally reject mutating tools until one exists"`
	Compaction                *CompactionOptions     `json:"compaction,omitempty" jsonschema:"description=Context compaction settings"`
	MaxBackgroundJobs         *int                   `json:"max_background_jobs,omitempty" jsonschema:"description=How many background shell jobs may run at once. Jobs that have finished never count against it. 0 or negative restores the default.,default=50,example=10,example=200"`
	Progress                  *bool                  `json:"progress,omitempty" jsonschema:"description=Show indeterminate progress updates during long operations,default=true"`
	LazyMCP                   *bool                  `json:"lazy_mcp,omitempty" jsonschema:"description=Keep MCP tools out of the model context until the agent loads them with mcp_search. Individual servers override it with mcp.<name>.lazy,default=true"`
	DataDirectory             string                 `json:"data_directory,omitempty" jsonschema:"description=Directory for storing application data. Relative paths are resolved against the working directory; absolute paths are used as-is.,default=.crush,example=.crush"`
	InitializeAs              string                 `json:"initialize_as,omitempty" jsonschema:"description=Name of the context file to create/update during project initialization,default=AGENTS.md,example=AGENTS.md,example=CRUSH.md,example=CLAUDE.md,example=docs/LLMs.md"`
	Notifications             string                 `json:"notifications,omitempty" jsonschema:"description=Notification style to use. Options: auto (default)\\, native\\, osc\\, bell\\, disabled. Auto selects based on environment: native for local sessions\\, osc for SSH (with automatic OSC 99/777 detection).,enum=auto,enum=native,enum=osc,enum=bell,enum=disabled,default=auto"`
	GlobalContextPaths        []string               `json:"global_context_paths,omitempty" jsonschema:"description=Paths to files containing global context information for the AI,default=~/.config/crush/CRUSH.md,default=~/.config/AGENTS.md"`
	SkillsPaths               []string               `json:"skills_paths,omitempty" jsonschema:"description=Paths to directories containing Agent Skills (folders with SKILL.md files),example=~/.config/crush/skills,example=./skills"`
	DisabledTools             []string               `json:"disabled_tools,omitempty" jsonschema:"description=List of built-in tools to disable and hide from the agent,example=bash,example=sourcegraph"`
	DisabledSkills            []string               `json:"disabled_skills,omitempty" jsonschema:"description=List of skill names to disable and hide from the agent,example=crush-config"`
	ContextPaths              []string               `json:"context_paths,omitempty" jsonschema:"description=Paths to files containing context information for the AI,example=.cursorrules,example=CRUSH.md"`
	AllowedCommands           []string               `json:"allowed_commands,omitempty" jsonschema:"description=List of commands to allow that are normally banned. Only affects the exact-command block list; package-manager argument blocks (e.g. 'apt install') are unaffected. Allowed commands still require permission approval.,example=ssh,example=curl,example=scp"`
	AllowAllCommands          bool                   `json:"allow_all_commands,omitempty" jsonschema:"description=Remove all command restrictions from the bash tool, including package-manager argument blocks (dangerous). Commands still require permission approval unless yolo mode is enabled.,default=false"`
	DisableProviderAutoUpdate bool                   `json:"disable_provider_auto_update,omitempty" jsonschema:"description=Disable providers auto-update,default=false"`
	Debug                     bool                   `json:"debug,omitempty" jsonschema:"description=Enable debug logging,default=false"`
	DisableAutoSummarize      bool                   `json:"disable_auto_summarize,omitempty" jsonschema:"description=Disable automatic conversation summarization,default=false"`
	DebugLSP                  bool                   `json:"debug_lsp,omitempty" jsonschema:"description=Enable debug logging for LSP servers,default=false"`
	DisableDefaultProviders   bool                   `json:"disable_default_providers,omitempty" jsonschema:"description=Ignore all default/embedded providers. When enabled\\, providers must be fully specified in the config file with base_url\\, models\\, and api_key - no merging with defaults occurs,default=false"`
}

// DefaultMaxBackgroundJobs is how many background shell jobs may run at once
// when the user has not set max_background_jobs.
const DefaultMaxBackgroundJobs = 50

// GetMaxBackgroundJobs returns the running-job ceiling, always positive.
func (o *Options) GetMaxBackgroundJobs() int {
	if o == nil || o.MaxBackgroundJobs == nil || *o.MaxBackgroundJobs <= 0 {
		return DefaultMaxBackgroundJobs
	}
	return *o.MaxBackgroundJobs
}

// DefaultDispatchMaxConcurrent bounds how many dispatched agents run at
// once when the user has not configured a cap (#390): each dispatch
// provisions a worktree, a scoped LSP manager, an A2A listener and a
// model session, so an unbounded loop of dispatch calls would exhaust
// disk, memory and API budget.
const DefaultDispatchMaxConcurrent = 4

// DefaultDispatchAgent is the agent definition dispatch_agent runs when
// its call names no agent (#433).
const DefaultDispatchAgent = AgentWorker

// DispatchOptions configures the dispatch_agent tool (#390).
type DispatchOptions struct {
	// MaxConcurrent caps the concurrently running dispatched agents:
	// live dispatches plus setups still in flight. A value below 1 is
	// a load error; nil resolves to DefaultDispatchMaxConcurrent.
	MaxConcurrent *int `json:"max_concurrent,omitempty" jsonschema:"description=Maximum number of concurrently running dispatched agents; a value below 1 is a load error,default=4,example=2"`
	// DefaultAgent is the agent id dispatch_agent runs when its call
	// names no agent (#433). It must name an existing dispatch agent
	// that is not disabled; anything else is a load error. Empty
	// resolves to DefaultDispatchAgent.
	DefaultAgent string `json:"default_agent,omitempty" jsonschema:"description=Agent id dispatch_agent runs when the call names no agent; must be an enabled dispatch agent,default=worker"`
}

// GetDispatchMaxConcurrent returns the resolved dispatch concurrency cap
// (#390): the configured max_concurrent, or DefaultDispatchMaxConcurrent
// when unset. The nil receiver and the unset field both mean the default,
// so callers can ask without unwrapping either.
func (o *Options) GetDispatchMaxConcurrent() int {
	if o == nil || o.Dispatch == nil || o.Dispatch.MaxConcurrent == nil {
		return DefaultDispatchMaxConcurrent
	}
	return *o.Dispatch.MaxConcurrent
}

// GetDispatchDefaultAgent returns the resolved default dispatch agent
// id (#433): the configured default_agent, or DefaultDispatchAgent when
// unset. The nil receiver and the empty field both mean the default, so
// callers can ask without unwrapping either.
func (o *Options) GetDispatchDefaultAgent() string {
	if o == nil || o.Dispatch == nil || o.Dispatch.DefaultAgent == "" {
		return DefaultDispatchAgent
	}
	return o.Dispatch.DefaultAgent
}

// Validate checks the dispatch concurrency cap: a positive value
// configures it, and a value below 1 is a load error, not a setting —
// a cap of 0 would refuse every dispatch. The path names the block in
// the config file, so the error points at the offending key.
func (d *DispatchOptions) Validate(path string) error {
	if d == nil {
		return nil
	}
	if d.MaxConcurrent != nil && *d.MaxConcurrent < 1 {
		return fmt.Errorf("%s.max_concurrent: must be at least 1 (got %d)", path, *d.MaxConcurrent)
	}
	return nil
}

// ValidateDispatchDefaultAgent checks options.dispatch.default_agent
// against the resolved agents (#433): the id must exist and be a dispatch
// agent, so a dispatch that names no agent never fails at dispatch time
// on a bad default. An explicitly named disabled id is an error; the
// implicit default resolving to a disabled worker is not — that is the
// opt-out, and the dispatch tools simply go away with it. An external
// agent that cannot be dispatched is an error too (#560): a default
// every dispatch would refuse is a misconfiguration, not a warning. It
// runs after SetupAgents, at load and reload time, and names the config
// path in every error.
func (c *Config) ValidateDispatchDefaultAgent() error {
	const path = "options.dispatch.default_agent"
	explicit := c.Options != nil && c.Options.Dispatch != nil && c.Options.Dispatch.DefaultAgent != ""
	id := c.Options.GetDispatchDefaultAgent()
	agent, ok := c.Agents[id]
	if !ok {
		return fmt.Errorf("%s: unknown agent %q", path, id)
	}
	if agent.Role != AgentRoleDispatch {
		return fmt.Errorf("%s: agent %q is a %q agent, not a dispatch agent", path, id, agent.Role)
	}
	if agent.Disabled && explicit {
		return fmt.Errorf("%s: agent %q is disabled", path, id)
	}
	if agent.Unusable != "" {
		return fmt.Errorf("%s: agent %q cannot be dispatched: %s", path, id, agent.Unusable)
	}
	return nil
}

// definitionToolset resolves a definition's own tool palette: allow
// expanded (an omitted list inherits every tool), minus deny — before
// the user's disabled_tools. Policy is not a definition error (#433),
// so the load-time sanity check reads this set, not the policy-filtered
// AllowedTools the resolved Agent carries.
func definitionToolset(def AgentDefinition) []string {
	var toolsSpec AgentTools
	if def.Tools != nil {
		toolsSpec = *def.Tools
	}
	allow := allToolNames()
	if toolsSpec.Allow != nil {
		allow = expandToolRefs(toolsSpec.Allow)
	}
	if deny := expandToolRefs(toolsSpec.Deny); len(deny) > 0 {
		allow = slices.DeleteFunc(allow, func(name string) bool {
			return slices.Contains(deny, name)
		})
	}
	return allow
}

// ValidateAgentToolsets runs the load-time sanity checks (#433): an
// enabled coder, plan, or dispatch agent whose definition resolves to no
// tools fails the load instead of quietly producing an agent that can do
// nothing. The user's disabled_tools and permissions deny are policy,
// not definition errors — they keep their runtime behavior, including
// the #376 refusal. A disabled agent always loads: it is the opt-out.
// It runs after SetupAgents, at load and reload time, and names the
// config path in every error.
func (c *Config) ValidateAgentToolsets() error {
	effective := effectiveAgentDefinitions(c.AgentDefinitions)
	for _, id := range slices.Sorted(maps.Keys(effective)) {
		switch definitionRole(id, effective[id]) {
		case AgentRoleMain, AgentRoleDispatch:
		default:
			continue
		}
		if agent, ok := c.Agents[id]; !ok || agent.Disabled {
			continue
		}
		if len(definitionToolset(effective[id])) > 0 {
			continue
		}
		return fmt.Errorf("agents.%s: enabled agent resolves to no tools", id)
	}
	return nil
}

// DefaultRequestTimeout bounds each LLM API request when the user has not
// configured a timeout. Slow or unreachable providers fail after it instead
// of blocking a session forever; streamed responses are only aborted after
// this much inactivity between tokens, and users running slow local models
// can raise or disable it via options.request_timeout.
const DefaultRequestTimeout = 2 * time.Minute

// GetRequestTimeout returns the per-request timeout for LLM API calls (a
// hard deadline for non-streaming requests and an idle timeout for
// streams, with a more generous budget for the first streamed token), or
// zero when disabled. The nil receiver and the unset field both mean
// DefaultRequestTimeout, so callers can ask without unwrapping either.
func (o *Options) GetRequestTimeout() time.Duration {
	if o == nil || o.RequestTimeout == nil {
		return DefaultRequestTimeout
	}
	if *o.RequestTimeout <= 0 {
		return 0
	}
	return time.Duration(*o.RequestTimeout) * time.Second
}

type MCPs map[string]MCPConfig

// EmbeddingsConfig configures the embedding provider that backs the
// semantic_search tool. It points at any OpenAI-compatible
// /v1/embeddings endpoint. A nil config disables semantic search
// entirely — the tool is not registered.
type EmbeddingsConfig struct {
	BaseURL   string `json:"base_url,omitempty" jsonschema:"description=Base URL of an OpenAI-compatible embeddings endpoint,default=https://api.openai.com/v1,example=https://api.openai.com/v1"`
	APIKey    string `json:"api_key,omitempty" jsonschema:"description=API key for the embeddings endpoint. Supports shell expansion of $VAR and $(command)."`
	Model     string `json:"model,omitempty" jsonschema:"description=Embedding model name,default=text-embedding-3-small,example=text-embedding-3-small"`
	Dimension int    `json:"dimension,omitempty" jsonschema:"description=Embedding vector dimension. Baked into the vector table on first creation; changing it requires reindexing.,default=768,example=768"`
}

type MCP struct {
	Name string    `json:"name"`
	MCP  MCPConfig `json:"mcp"`
}

// TodoEnforcementConfig configures the proactive todo enforcement ladder
// (#315): nudges and, opt-in, a hard gate that keeps mutating tools out of
// agents with no todo list. It appears once under options (global defaults
// for every agent) and once per agent definition (per agent type
// overrides); a per-agent field replaces the global one field by field.
//
// Nil everywhere means "defaults on": nudge injection enabled, hard gate
// off.
type TodoEnforcementConfig struct {
	// Enabled toggles nudge injection. When false the coordinator never
	// injects todo nudges; the hard gate is governed by its own field.
	Enabled *bool `json:"enabled,omitempty" jsonschema:"description=Inject a context nudge when an agent works without a todo list,default=true"`
	// NudgeThreshold is how many tool calls without todos activity trip
	// the first nudge. A mutating tool call (a file writer, a
	// non-read-only bash command, an MCP tool without a read-only hint)
	// trips it immediately regardless of count. 0 or "off" disables
	// nudging, the same as enabled: false; a negative value is a load
	// error.
	NudgeThreshold *IntOrOff `json:"nudge_threshold,omitempty" jsonschema:"description=Tool calls without todos activity before a nudge is injected; a mutating tool call trips it immediately. 0 or 'off' disables nudging\\, the same as enabled: false; a negative value is a load error"`
	// HardGate rejects mutating tools (file writers, non-read-only bash
	// commands, MCP tools without a read-only hint) until the session
	// has a todo list. Deterministic but brittle; off by default.
	HardGate *bool `json:"hard_gate,omitempty" jsonschema:"description=Reject mutating tools (file writers\\, non-read-only bash commands\\, MCP tools without a read-only hint) until a todo list exists,default=false"`
	// KillAfterNudges is the wander-kill rung (#316): how many nudges a
	// run may ignore before the coordinator kills it deterministically.
	// The default of 2 kills after both the nudge and the escalating
	// nudge are ignored; 0 or "off" disables the kill, and any positive
	// value is honored, unclamped. It only wires up for dispatched
	// agents (the parent must be able to re-dispatch); the main
	// agent's ladder stays capped at nudging. A negative value is a
	// load error.
	KillAfterNudges *IntOrOff `json:"kill_after_nudges,omitempty" jsonschema:"description=Wander kill: nudges a dispatched agent may ignore before it is killed. 0 or 'off' disables; a negative value is a load error"`
	// StallWindow kills a dispatched run whose todo list has not been
	// updated for this long while it keeps running ("stalled todos").
	// 0 or "off" (the default) disables the stall check. Accepts a
	// number of seconds (legacy form) or a duration string such as
	// "30m"; a negative value is a load error.
	StallWindow *Duration `json:"stall_window,omitempty" jsonschema:"description=Wander kill: time without a todo update that marks a dispatched run as stalled. 0 or 'off' disables; a negative value is a load error"`
	// HardTimeout kills a dispatched run after this long, whatever its
	// progress. 0 or "off" (the default) disables the timeout. Accepts
	// a number of seconds (legacy form) or a duration string such as
	// "30m"; a negative value is a load error.
	HardTimeout *Duration `json:"hard_timeout,omitempty" jsonschema:"description=Wander kill: time after which a dispatched run is killed outright. 0 or 'off' disables; a negative value is a load error"`
	// InactivityTimeout is the A2A-level backstop (#360): a dispatched
	// run served over A2A that yields no events at all for this many
	// seconds is ended by the executor with a Failed status carrying
	// the reason. It guards the runs the in-process watchdogs (#316)
	// miss: stall_window and hard_timeout default to 0, and the ladder
	// needs model steps to fire. 0 (the default) disables the backstop.
	InactivityTimeout *int `json:"inactivity_timeout,omitempty" jsonschema:"description=A2A backstop: seconds without any event that end a silent dispatched run with a Failed status; 0 disables,default=0,example=900"`
}

// JSONSchemaExtend adds the defaults and examples for the knob fields.
// They cannot come from the field tags: the tags reach only the $ref
// wrapper, while the type-specific keyword parsing the tags rely on runs
// on typed properties, and these are references to the knob definitions.
func (TodoEnforcementConfig) JSONSchemaExtend(schema *jsonschema.Schema) {
	if schema.Properties == nil {
		return
	}
	set := func(name string, def any, examples ...any) {
		if prop, ok := schema.Properties.Get(name); ok {
			prop.Default = def
			prop.Examples = examples
		}
	}
	set("nudge_threshold", 4, 8, 1, "off")
	set("kill_after_nudges", 2, 3, 10, "off")
	set("stall_window", 0, 300, "30m", "off")
	set("hard_timeout", 0, 1800, "30m", "off")
}

// Validate checks the ladder's numeric knobs: 0 or "off" disables a knob
// and a positive value configures it, but a negative value is a load
// error, not a setting. The path names the block in the config file, so
// each error points at the offending key.
func (c *TodoEnforcementConfig) Validate(path string) error {
	if c == nil {
		return nil
	}
	if c.NudgeThreshold != nil && *c.NudgeThreshold < 0 {
		return fmt.Errorf("%s.nudge_threshold: must not be negative (got %d)", path, *c.NudgeThreshold)
	}
	if c.KillAfterNudges != nil && *c.KillAfterNudges < 0 {
		return fmt.Errorf("%s.kill_after_nudges: must not be negative (got %d)", path, *c.KillAfterNudges)
	}
	if c.StallWindow != nil && *c.StallWindow < 0 {
		return fmt.Errorf("%s.stall_window: must not be negative (got %s)", path, durationForError(*c.StallWindow))
	}
	if c.HardTimeout != nil && *c.HardTimeout < 0 {
		return fmt.Errorf("%s.hard_timeout: must not be negative (got %s)", path, durationForError(*c.HardTimeout))
	}
	if c.InactivityTimeout != nil && *c.InactivityTimeout < 0 {
		return fmt.Errorf("%s.inactivity_timeout: must not be negative (got %d)", path, *c.InactivityTimeout)
	}
	return nil
}

// TodoEnforcementSettings is the resolved enforcement ladder: the per-agent
// override layered over the global options with defaults applied.
type TodoEnforcementSettings struct {
	NudgeThreshold    int
	KillAfterNudges   int
	StallWindow       time.Duration
	HardTimeout       time.Duration
	InactivityTimeout time.Duration
	Enabled           bool
	HardGate          bool
	// KillAfterNudgesSet reports whether the kill threshold was
	// configured explicitly, as opposed to carrying the default. Goal
	// mode reads it: an explicit threshold says how many ignored
	// continuations to put up with, while the default says nothing and
	// leaves the goal's own turn budget in charge.
	KillAfterNudgesSet bool
}

const (
	defaultTodoNudgeThreshold = 4
	// defaultTodoKillAfterNudges kills a dispatched agent after both the
	// nudge and the escalating nudge have been ignored.
	defaultTodoKillAfterNudges = 2
)

// ResolveTodoEnforcement layers the per-agent config over over the global
// one, field by field, and applies defaults. Nil layers resolve to the
// defaults: nudging on, threshold 4, hard gate off.
func ResolveTodoEnforcement(global, over *TodoEnforcementConfig) TodoEnforcementSettings {
	merged := TodoEnforcementConfig{}
	if global != nil {
		merged = *global
	}
	if over != nil {
		if over.Enabled != nil {
			merged.Enabled = over.Enabled
		}
		if over.NudgeThreshold != nil {
			merged.NudgeThreshold = over.NudgeThreshold
		}
		if over.HardGate != nil {
			merged.HardGate = over.HardGate
		}
		if over.KillAfterNudges != nil {
			merged.KillAfterNudges = over.KillAfterNudges
		}
		if over.StallWindow != nil {
			merged.StallWindow = over.StallWindow
		}
		if over.HardTimeout != nil {
			merged.HardTimeout = over.HardTimeout
		}
		if over.InactivityTimeout != nil {
			merged.InactivityTimeout = over.InactivityTimeout
		}
	}
	settings := TodoEnforcementSettings{
		Enabled:         true,
		NudgeThreshold:  defaultTodoNudgeThreshold,
		KillAfterNudges: defaultTodoKillAfterNudges,
	}
	if merged.Enabled != nil {
		settings.Enabled = *merged.Enabled
	}
	if merged.NudgeThreshold != nil {
		settings.NudgeThreshold = int(*merged.NudgeThreshold)
	}
	if merged.HardGate != nil {
		settings.HardGate = *merged.HardGate
	}
	if merged.KillAfterNudges != nil {
		settings.KillAfterNudges = int(*merged.KillAfterNudges)
		settings.KillAfterNudgesSet = true
	}
	if merged.StallWindow != nil {
		settings.StallWindow = time.Duration(*merged.StallWindow)
	}
	if merged.HardTimeout != nil {
		settings.HardTimeout = time.Duration(*merged.HardTimeout)
	}
	// An explicit 0 (or "off") threshold disables nudging outright, the
	// same as enabled: false; it must win over an explicit enabled: true.
	if merged.NudgeThreshold != nil && *merged.NudgeThreshold == 0 {
		settings.Enabled = false
	}
	if merged.InactivityTimeout != nil && *merged.InactivityTimeout > 0 {
		settings.InactivityTimeout = time.Duration(*merged.InactivityTimeout) * time.Second
	}
	return settings
}

// ResolvedTodoEnforcement returns the effective enforcement settings for
// this agent type, layering its override over the global options.
func (a Agent) ResolvedTodoEnforcement(global *TodoEnforcementConfig) TodoEnforcementSettings {
	return ResolveTodoEnforcement(global, a.TodoEnforcement)
}

func (m MCPs) Sorted() []MCP {
	sorted := make([]MCP, 0, len(m))
	for k, v := range m {
		sorted = append(sorted, MCP{
			Name: k,
			MCP:  v,
		})
	}
	slices.SortFunc(sorted, func(a, b MCP) int {
		return strings.Compare(a.Name, b.Name)
	})
	return sorted
}

type LSPs map[string]LSPConfig

type LSP struct {
	Name string    `json:"name"`
	LSP  LSPConfig `json:"lsp"`
}

func (l LSPs) Sorted() []LSP {
	sorted := make([]LSP, 0, len(l))
	for k, v := range l {
		sorted = append(sorted, LSP{
			Name: k,
			LSP:  v,
		})
	}
	slices.SortFunc(sorted, func(a, b LSP) int {
		return strings.Compare(a.Name, b.Name)
	})
	return sorted
}

// ResolvedEnv returns m.Env with every value expanded through the
// given resolver. The returned slice is of the form "KEY=value" sorted
// by key so callers get deterministic output; the receiver's Env map is
// not mutated. On the first resolution failure it returns nil and an
// error that identifies the offending key; the inner resolver error is
// already sanitized by ResolveValue and is wrapped with %w so
// errors.Is/As continues to work. Callers are expected to surface it
// (for MCP, via StateError on the status card) rather than silently
// spawn the server with an empty credential.
//
// The resolver choice matters: in server mode pass the shell resolver
// so $VAR / $(cmd) expand; in client mode pass IdentityResolver so the
// template is forwarded verbatim and expansion happens on the server.
func (m MCPConfig) ResolvedEnv(r VariableResolver) ([]string, error) {
	return resolveEnvs(m.Env, r)
}

// ResolvedArgs returns m.Args with every element expanded through the
// given resolver. A fresh slice is allocated; m.Args is never mutated.
// On the first resolution failure it returns nil and an error
// identifying the offending positional index; the inner resolver error
// is already sanitized by ResolveValue and is wrapped with %w so
// errors.Is/As continues to work.
//
// See ResolvedEnv for guidance on picking a resolver.
func (m MCPConfig) ResolvedArgs(r VariableResolver) ([]string, error) {
	if len(m.Args) == 0 {
		return nil, nil
	}
	out := make([]string, len(m.Args))
	for i, a := range m.Args {
		v, err := r.ResolveValue(a)
		if err != nil {
			return nil, fmt.Errorf("arg %d: %w", i, err)
		}
		out[i] = v
	}
	return out, nil
}

// ResolvedURL returns m.URL expanded through the given resolver. The
// receiver is not mutated. Errors from the resolver are already
// sanitized by ResolveValue and are wrapped with %w for errors.Is/As.
//
// URLs run through the same shell-expansion pipeline as the other
// fields, so a literal '$' (e.g. OData query strings containing
// $filter/$select) must be escaped as '\$' or '${DOLLAR:-$}' to avoid
// being interpreted as a variable reference. Same constraint already
// applies to command, args, env, and headers.
//
// See ResolvedEnv for guidance on picking a resolver.
func (m MCPConfig) ResolvedURL(r VariableResolver) (string, error) {
	if m.URL == "" {
		return "", nil
	}
	v, err := r.ResolveValue(m.URL)
	if err != nil {
		return "", fmt.Errorf("url: %w", err)
	}
	return v, nil
}

// knownSessionlessMCPs is the set of MCP endpoint URLs (normalized, no
// trailing slash) that are known not to maintain an MCP session — they
// never issue a Mcp-Session-Id and reject the SEP-2575
// "subscriptions/listen" stream. Add an entry when a server is confirmed to
// behave this way.
var knownSessionlessMCPs = map[string]struct{}{
	"https://api.github.com/mcp":        {},
	"https://api.githubcopilot.com/mcp": {},
}

// IsSessionless reports whether the server should be treated as sessionless.
// An explicit Sessionless value wins; when unset, the resolved URL is matched
// against knownSessionlessMCPs (trailing slash ignored). The URL is resolved
// through r so $VAR-expanded endpoints are detected too; on a resolution
// error the explicit value (or false) is used.
func (m MCPConfig) IsSessionless(r VariableResolver) bool {
	if m.Sessionless != nil {
		return *m.Sessionless
	}
	url, err := m.ResolvedURL(r)
	if err != nil {
		return false
	}
	_, ok := knownSessionlessMCPs[strings.TrimSuffix(url, "/")]
	return ok
}

// ResolvedHeaders returns m.Headers with every value expanded through
// the given resolver. A fresh map is allocated; m.Headers is never
// mutated. On the first resolution failure it returns nil and an error
// identifying the offending header name; the inner resolver error is
// already sanitized by ResolveValue and is wrapped with %w so
// errors.Is/As continues to work.
//
// A header whose value resolves to the empty string (unset bare $VAR
// under lenient nounset, $(echo), or literal "") is omitted from the
// returned map — sending "X-Auth:" with an empty value is rejected by
// some providers and the user's intent in "optional, env-gated
// header" is clearly "absent when the var isn't set."
//
// See ResolvedEnv for guidance on picking a resolver.
func (m MCPConfig) ResolvedHeaders(r VariableResolver) (map[string]string, error) {
	if len(m.Headers) == 0 {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(m.Headers))
	// Sort keys so failures are reported deterministically when more
	// than one header would fail.
	keys := make([]string, 0, len(m.Headers))
	for k := range m.Headers {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		v, err := r.ResolveValue(m.Headers[k])
		if err != nil {
			return nil, fmt.Errorf("header %s: %w", k, err)
		}
		if v == "" {
			continue
		}
		out[k] = v
	}
	return out, nil
}

// ResolvedArgs returns l.Args with every element expanded through the
// given resolver. A fresh slice is allocated; l.Args is never mutated.
// On the first resolution failure it returns nil and an error
// identifying the offending positional index; the inner resolver error
// is already sanitized by ResolveValue and is wrapped with %w so
// errors.Is/As continues to work.
//
// Empty resolved values are kept (a deliberate "empty positional arg"
// like --flag "" is sometimes valid), matching MCPConfig.ResolvedArgs.
//
// The resolver choice matters: in server mode pass the shell resolver
// so $VAR / $(cmd) expand; in client mode pass IdentityResolver so the
// template is forwarded verbatim.
func (l LSPConfig) ResolvedArgs(r VariableResolver) ([]string, error) {
	if len(l.Args) == 0 {
		return nil, nil
	}
	out := make([]string, len(l.Args))
	for i, a := range l.Args {
		v, err := r.ResolveValue(a)
		if err != nil {
			return nil, fmt.Errorf("arg %d: %w", i, err)
		}
		out[i] = v
	}
	return out, nil
}

// ResolvedEnv returns l.Env with every value expanded through the
// given resolver. A fresh map is allocated; l.Env is never mutated.
// On the first resolution failure it returns nil and an error that
// identifies the offending key; the inner resolver error is already
// sanitized by ResolveValue and is wrapped with %w so errors.Is/As
// continues to work.
//
// Empty resolved values are kept ("FOO=" is a legitimate request;
// opt out via ${VAR:+...}), matching MCPConfig.ResolvedEnv.
//
// Shape note: this returns map[string]string rather than the []string
// shape MCPConfig.ResolvedEnv uses because the consumer
// (powernap.ClientConfig.Environment in internal/lsp/client.go) takes
// a map directly — returning a []string here would only force a
// round-trip back to a map at the call site.
//
// See ResolvedArgs for guidance on picking a resolver.
func (l LSPConfig) ResolvedEnv(r VariableResolver) (map[string]string, error) {
	if len(l.Env) == 0 {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(l.Env))
	// Sort keys so failures are reported deterministically when more
	// than one value would fail.
	keys := make([]string, 0, len(l.Env))
	for k := range l.Env {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		v, err := r.ResolveValue(l.Env[k])
		if err != nil {
			return nil, fmt.Errorf("env %q: %w", k, err)
		}
		out[k] = v
	}
	return out, nil
}

type Agent struct {
	AllowedMCP      map[string][]string    `json:"allowed_mcp,omitempty"`
	Transport       *AgentTransport        `json:"-"`
	Auth            *AgentAuth             `json:"-"`
	ModelRef        *AgentModelRef         `json:"-"`
	TodoEnforcement *TodoEnforcementConfig `json:"todo_enforcement,omitempty" jsonschema:"description=Todo enforcement overrides for this agent: nudge injection and the opt-in mutating-tool gate"`
	Workspace       string                 `json:"-"`
	PromptAppend    string                 `json:"-"`
	Unusable        string                 `json:"-"`
	Model           SelectedModelType      `json:"model" jsonschema:"required,description=The model type to use for this agent,enum=large,enum=small,default=large"`
	Role            string                 `json:"-"`
	Runtime         string                 `json:"-"`
	Prompt          string                 `json:"-"`
	Name            string                 `json:"name,omitempty"`
	Description     string                 `json:"description,omitempty"`
	Card            string                 `json:"-"`
	ID              string                 `json:"id,omitempty"`
	Skills          []string               `json:"-"`
	AllowedTools    []string               `json:"allowed_tools,omitempty"`
	ContextPaths    []string               `json:"context_paths,omitempty"`
	Disabled        bool                   `json:"disabled,omitempty"`
}

type Tools struct {
	Ls   ToolLs   `json:"ls,omitzero"`
	Grep ToolGrep `json:"grep,omitzero"`
	Glob ToolGlob `json:"glob,omitzero"`
}

type ToolLs struct {
	MaxDepth *int `json:"max_depth,omitempty" jsonschema:"description=Maximum depth for the ls tool,default=0,example=10"`
	MaxItems *int `json:"max_items,omitempty" jsonschema:"description=Maximum number of items to return for the ls tool,default=1000,example=100"`
}

// Limits returns the user-defined max-depth and max-items, or their defaults.
func (t ToolLs) Limits() (depth, items int) {
	return ptrValOr(t.MaxDepth, 0), ptrValOr(t.MaxItems, 0)
}

type ToolGrep struct {
	Timeout *time.Duration `json:"timeout,omitempty" jsonschema:"description=Timeout for the grep tool call,default=5s,example=10s"`
}

// GetTimeout returns the user-defined timeout or the default.
func (t ToolGrep) GetTimeout() time.Duration {
	return ptrValOr(t.Timeout, 5*time.Second)
}

type ToolGlob struct {
	Timeout *time.Duration `json:"timeout,omitempty" jsonschema:"description=Timeout for the glob tool call,default=30s,example=10s"`
}

// GetTimeout returns the user-defined timeout or the default.
func (t ToolGlob) GetTimeout() time.Duration {
	return ptrValOr(t.Timeout, 30*time.Second)
}

// HookConfig defines a user-configured shell command that fires on a hook
// event (e.g. PreToolUse). This is a pure-data struct: matcher compilation
// is owned by hooks.Runner so a JSON round-trip, merge, or reload can't
// silently drop compiled state.
type HookConfig struct {
	// Friendly display name shown in the TUI. Falls back to Command when empty.
	Name string `json:"name,omitempty" jsonschema:"description=Friendly display name shown in the TUI for this hook"`
	// Regex pattern tested against the tool name. Empty means match all.
	Matcher string `json:"matcher,omitempty" jsonschema:"description=Regex pattern tested against the tool name. Empty means match all tools."`
	// Shell command to execute.
	Command string `json:"command" jsonschema:"required,description=Shell command to execute when the hook fires"`
	// Timeout in seconds. Default 30.
	Timeout int `json:"timeout,omitempty" jsonschema:"description=Timeout in seconds for the hook command,default=30"`
}

// DisplayName returns the hook name for display purposes. It returns Name
// when set, otherwise falls back to Command.
func (h *HookConfig) DisplayName() string {
	if h.Name != "" {
		return h.Name
	}
	return h.Command
}

// TimeoutDuration returns the hook timeout as a time.Duration, defaulting
// to 30s.
func (h *HookConfig) TimeoutDuration() time.Duration {
	if h.Timeout <= 0 {
		return 30 * time.Second
	}
	return time.Duration(h.Timeout) * time.Second
}

// ResolvedEmbeddings returns the effective embedding configuration with
// defaults applied, and whether semantic search is enabled at all. The
// second return value is false when no embeddings block is configured.
func (c *Config) ResolvedEmbeddings() (EmbeddingsConfig, bool) {
	if c.Embeddings == nil {
		return EmbeddingsConfig{}, false
	}
	e := *c.Embeddings
	if e.BaseURL == "" {
		e.BaseURL = "https://api.openai.com/v1"
	}
	if e.Model == "" {
		e.Model = "text-embedding-3-small"
	}
	if e.Dimension <= 0 {
		e.Dimension = 768
	}
	return e, true
}

// Config holds the configuration for crush.
type Config struct {
	Tools            Tools                                 `json:"tools,omitzero" jsonschema:"description=Tool configurations"`
	Providers        *csync.Map[string, ProviderConfig]    `json:"providers,omitempty" jsonschema:"description=AI provider configurations"`
	RecentModels     map[SelectedModelType][]SelectedModel `json:"recent_models,omitempty" jsonschema:"-"`
	MCP              MCPs                                  `json:"mcp,omitempty" jsonschema:"description=Model Context Protocol server configurations"`
	LSP              LSPs                                  `json:"lsp,omitempty" jsonschema:"description=Language Server Protocol configurations"`
	Embeddings       *EmbeddingsConfig                     `json:"embeddings,omitempty" jsonschema:"description=Embedding provider configuration for semantic search"`
	Snapshots        *SnapshotsConfig                      `json:"snapshots,omitempty" jsonschema:"description=Git-backed filesystem snapshots captured at each user message"`
	Options          *Options                              `json:"options,omitempty" jsonschema:"description=General application options"`
	Permissions      *Permissions                          `json:"permissions,omitempty" jsonschema:"description=Permission settings for tool usage"`
	Models           map[SelectedModelType]SelectedModel   `json:"models,omitempty" jsonschema:"description=Model configurations for different model types,example={\"large\":{\"model\":\"gpt-4o\",\"provider\":\"openai\"}}"`
	Hooks            map[string][]HookConfig               `json:"hooks,omitempty" jsonschema:"description=User-defined shell commands that fire on hook events (e.g. PreToolUse)"`
	AutoMode         *AutoModeConfig                       `json:"auto_mode,omitempty" jsonschema:"description=Native auto mode: an in-process safety classifier that runs before permission prompts. User-level only: a repository cannot grant itself auto mode"`
	Honcho           *Honcho                               `json:"honcho,omitempty" jsonschema:"description=Honcho memory integration; gives Crush recall across sessions"`
	Env              map[string]string                     `json:"env,omitempty" jsonschema:"description=Environment variables to set on startup"`
	AgentDefinitions map[string]AgentDefinition            `json:"agents,omitempty" jsonschema:"description=Agent definitions: override a built-in agent field by field, or add dispatch agents"`
	Agents           map[string]Agent                      `json:"-"`
	Schema           string                                `json:"$schema,omitempty"`
}

// Honcho configures the Honcho memory integration.
//
// Fields left empty fall back to the shared cross-harness config file
// (~/.honcho/config.json) and then to built-in defaults, so a user who has
// already set up Honcho for another tool needs nothing here but to enable it.
type Honcho struct {
	// Enabled turns the integration on. Supplying an API key or a custom base
	// URL also enables it, since either is an unambiguous statement of intent.
	Enabled bool `json:"enabled,omitempty" jsonschema:"description=Enable the Honcho memory integration"`

	// APIKey authenticates to Honcho. Prefer $HONCHO_API_KEY over writing a
	// credential into a config file.
	APIKey string `json:"api_key,omitempty" jsonschema:"description=Honcho API key; prefer the HONCHO_API_KEY environment variable"`

	// BaseURL points at a self-hosted deployment. Empty means Honcho Cloud.
	BaseURL string `json:"base_url,omitempty" jsonschema:"description=Honcho deployment URL; empty means Honcho Cloud"`

	// Workspace isolates this application's memory.
	Workspace string `json:"workspace,omitempty" jsonschema:"description=Honcho workspace to store memory in"`

	// PeerName identifies the human whose representation Honcho builds.
	PeerName string `json:"peer_name,omitempty" jsonschema:"description=Peer name identifying you in Honcho"`

	// AgentPeer identifies Crush itself.
	AgentPeer string `json:"agent_peer,omitempty" jsonschema:"description=Peer name identifying Crush in Honcho"`

	// RecallMode selects how memory reaches the model: hybrid injects context
	// automatically, tools leaves it to the model to ask.
	RecallMode string `json:"recall_mode,omitempty" jsonschema:"enum=hybrid,enum=context,enum=tools,description=How memory reaches the model"`

	// ObservationMode selects whose conclusion collection is used.
	ObservationMode string `json:"observation_mode,omitempty" jsonschema:"enum=unified,enum=directional,description=Which conclusion collection to use"`

	// SessionStrategy selects what maps onto a Honcho session.
	SessionStrategy string `json:"session_strategy,omitempty" jsonschema:"enum=per-directory,enum=per-repo,enum=git-branch,enum=per-session,enum=global,description=What maps onto a Honcho session"`

	// AgentObserveMe asks Honcho to model Crush itself, not just the user.
	AgentObserveMe bool `json:"agent_observe_me,omitempty" jsonschema:"description=Also build a representation of Crush itself"`

	// CaptureTools records one-line summaries of meaningful tool calls.
	// A pointer so an omitted field ("use the default", which is on)
	// stays distinguishable from an explicit false.
	CaptureTools *bool `json:"capture_tools,omitempty" jsonschema:"description=Record a summary of meaningful tool calls,default=true"`

	// MaxConclusions caps how many conclusions enter an injected block.
	MaxConclusions int `json:"max_conclusions,omitempty" jsonschema:"description=Maximum conclusions to inject per memory block"`

	// ContextTokens caps the size of the per-turn recall block.
	ContextTokens int `json:"context_tokens,omitempty" jsonschema:"description=Token budget for the per-turn memory block"`
}

// cloneForWrite returns a copy of c that the store's typed field mutators
// may modify without racing readers of the currently published Config.
//
// Reads of a published Config take no lock beyond the pointer load, so a
// mutator must never write through the live pointer. Instead it clones,
// mutates the clone, and atomically swaps it in. The clone gives fresh
// copies of every field a typed mutator touches in place — Models,
// RecentModels, MCP, Options (with its nested TUI pointer), and Providers.
// The remaining fields are immutable after load from the mutators'
// standpoint and are shared.
//
// Providers needs its own copy, and not merely for the map itself. Its
// values carry a Models slice, and csync.Map.Get hands back a struct whose
// slice aliases the map's own backing array, so a mutator that writes
// through a value it read out of the map — which is exactly what the
// model-discovery merge does — would write into the live, published map's
// memory while readers are reading it. Sharing the *csync.Map is safe only
// against a map-level data race, not against this one.
func (c *Config) cloneForWrite() *Config {
	nc := *c
	nc.Models = maps.Clone(c.Models)
	nc.RecentModels = maps.Clone(c.RecentModels)
	nc.MCP = maps.Clone(c.MCP)
	if c.AutoMode != nil {
		autoMode := *c.AutoMode
		if c.AutoMode.Classifier != nil {
			classifier := *c.AutoMode.Classifier
			autoMode.Classifier = &classifier
		}
		nc.AutoMode = &autoMode
	}
	if c.Providers != nil {
		providers := csync.NewMap[string, ProviderConfig]()
		for id, pc := range c.Providers.Seq2() {
			pc.Models = slices.Clone(pc.Models)
			pc.ChatGPTModels = slices.Clone(pc.ChatGPTModels)
			// Key by the map key, not pc.ID: providers straight from
			// the config merge (crushrc custom providers especially)
			// have an empty ID until configureProviders fills it in,
			// and keying by it would collapse them all into one "".
			providers.Set(id, pc)
		}
		nc.Providers = providers
	}
	if c.Options != nil {
		opts := *c.Options
		if c.Options.TUI != nil {
			tui := *c.Options.TUI
			opts.TUI = &tui
		}
		nc.Options = &opts
	}
	return &nc
}

// ensureTUI returns c.Options.TUI, allocating Options and TUI as needed so
// callers can assign TUI fields without nil checks.
func (c *Config) ensureTUI() *TUIOptions {
	if c.Options == nil {
		c.Options = &Options{}
	}
	if c.Options.TUI == nil {
		c.Options.TUI = &TUIOptions{}
	}
	return c.Options.TUI
}

func (c *Config) EnabledProviders() []ProviderConfig {
	var enabled []ProviderConfig
	for p := range c.Providers.Seq() {
		if !p.Disable {
			enabled = append(enabled, p)
		}
	}
	return enabled
}

// IsConfigured  return true if at least one provider is configured
func (c *Config) IsConfigured() bool {
	return len(c.EnabledProviders()) > 0
}

func (c *Config) GetModel(provider, model string) *catwalk.Model {
	if providerConfig, ok := c.Providers.Get(provider); ok {
		for _, m := range providerConfig.Models {
			if m.ID == model {
				return &m
			}
		}
		for _, m := range providerConfig.ChatGPTModels {
			if m.ID == model {
				return &m
			}
		}
		for _, m := range providerConfig.GrokModels {
			if m.ID == model {
				return &m
			}
		}
	}
	return nil
}

// ValidateReasoningEffort checks that effort is a reasoning level the
// given provider/model supports. It returns an error listing the accepted
// levels when the model cannot use it.
func (c *Config) ValidateReasoningEffort(provider, modelID, effort string) error {
	model := c.GetModel(provider, modelID)
	if model == nil {
		return fmt.Errorf("model %q not found for provider %q", modelID, provider)
	}
	if len(model.ReasoningLevels) == 0 {
		return fmt.Errorf("model %q does not support reasoning effort", modelID)
	}
	if slices.Contains(model.ReasoningLevels, effort) {
		return nil
	}
	return fmt.Errorf(
		"model %q does not support reasoning effort %q, accepted values: %s",
		modelID, effort, strings.Join(model.ReasoningLevels, ", "),
	)
}

// IsModelAvailable returns true if the provider is enabled and the model
// exists in its catalog. Unlike GetModel, it rejects disabled providers.
func (c *Config) IsModelAvailable(provider, model string) bool {
	providerConfig, ok := c.Providers.Get(provider)
	if !ok || providerConfig.Disable {
		return false
	}
	// A model is available in any of the provider's catalogs: the
	// API-key list, or the ChatGPT/Grok subscription list fetched on
	// login. Subscription models live outside Models, so checking only
	// there would reject a model that is selectable and usable.
	for _, m := range providerConfig.Models {
		if m.ID == model {
			return true
		}
	}
	for _, m := range providerConfig.ChatGPTModels {
		if m.ID == model {
			return true
		}
	}
	for _, m := range providerConfig.GrokModels {
		if m.ID == model {
			return true
		}
	}
	return false
}

func (c *Config) GetProviderForModel(modelType SelectedModelType) *ProviderConfig {
	model, ok := c.Models[modelType]
	if !ok {
		return nil
	}
	if providerConfig, ok := c.Providers.Get(model.Provider); ok {
		return &providerConfig
	}
	return nil
}

func (c *Config) GetModelByType(modelType SelectedModelType) *catwalk.Model {
	model, ok := c.Models[modelType]
	if !ok {
		return nil
	}
	return c.GetModel(model.Provider, model.Model)
}

func (c *Config) LargeModel() *catwalk.Model {
	model, ok := c.Models[SelectedModelTypeLarge]
	if !ok {
		return nil
	}
	return c.GetModel(model.Provider, model.Model)
}

func (c *Config) SmallModel() *catwalk.Model {
	model, ok := c.Models[SelectedModelTypeSmall]
	if !ok {
		return nil
	}
	return c.GetModel(model.Provider, model.Model)
}

const maxRecentModelsPerType = 5

func allToolNames() []string {
	return []string{
		"agent",
		"bash",
		"crush_info",
		"crush_logs",
		"CronCreate",
		"CronList",
		"CronDelete",
		"job_output",
		"job_kill",
		"download",
		"dispatch_agent",
		"message_agent",
		"cancel_dispatch",
		"apply_dispatch",
		"dismiss_dispatch",
		"edit",
		"multiedit",
		"lsp_diagnostics",
		"lsp_references",
		"lsp_restart",
		"lsp_symbols",
		"lsp_definition",
		"lsp_call_hierarchy",
		"lsp_rename",
		"lsp_replace_symbol",
		"fetch",
		"agentic_fetch",
		"glob",
		"grep",
		"ls",
		"question",
		"semantic_search",
		"semantic_index",
		"sourcegraph",
		"todos",
		"view",
		"write",
		"list_mcp_resources",
		"read_mcp_resource",
		"list_mcp_prompts",
		"call_mcp_prompt",
	}
}

func resolveAllowedTools(allTools []string, disabledTools []string) []string {
	if disabledTools == nil {
		return allTools
	}
	// filter out disabled tools (exclude mode)
	return filterSlice(allTools, disabledTools, false)
}

func filterSlice(data []string, mask []string, include bool) []string {
	var filtered []string
	for _, s := range data {
		// if include is true, we include items that ARE in the mask
		// if include is false, we include items that are NOT in the mask
		if include == slices.Contains(mask, s) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// SetupAgents resolves the agent map from the built-in definitions plus
// the user's agents block (#333). It stays error-free: validation runs
// in ValidateAgents and ValidateAgentModelRefs at load and reload time,
// so a bad definition never reaches this point. What it returns are the
// load diagnostics (#560): the problems that do not fail the load but
// change how a definition behaves, in a stable order. The ConfigStore
// keeps them for the command to show once the logger exists.
func (c *Config) SetupAgents() []LoadDiagnostic {
	agents := make(map[string]Agent, len(c.AgentDefinitions)+4)
	var diags []LoadDiagnostic
	for id, def := range effectiveAgentDefinitions(c.AgentDefinitions) {
		agent, agentDiags := c.agentFromDefinition(id, def)
		agents[id] = agent
		diags = append(diags, agentDiags...)
	}
	c.Agents = agents
	sortLoadDiagnostics(diags)
	return diags
}

// agentFromDefinition resolves one definition into the Agent the
// coordinator reads, plus the diagnostics resolving it raised.
// Effective tools are expand(allow) minus expand(deny) minus the user's
// disabled_tools, so a definition can narrow but never widen user
// policy.
func (c *Config) agentFromDefinition(id string, def AgentDefinition) (Agent, []LoadDiagnostic) {
	diags := unhonoredFieldDiagnostics(id, def)

	// The alias's kill knobs are dispatch-only (#402): on any other role
	// they are dropped here and warned about, not a load error. The
	// new-style Kill block takes the hard error instead.
	enforcementDef := def
	if alias := def.TodoEnforcement; alias != nil && definitionRole(id, def) != AgentRoleDispatch {
		trimmed := *alias
		trimmed.KillAfterNudges = nil
		trimmed.StallWindow = nil
		trimmed.HardTimeout = nil
		trimmed.InactivityTimeout = nil
		enforcementDef.TodoEnforcement = &trimmed
	}

	// Only an omitted allow list inherits every tool: an explicit empty
	// allow grants nothing, matching expand(allow) minus expand(deny).
	allow := definitionToolset(def)
	allowedTools := resolveAllowedTools(allow, c.Options.DisabledTools)

	var mcpAllow []string
	if def.MCP != nil {
		mcpAllow = def.MCP.Allow
	}

	var unusable string
	if orString(def.Runtime, AgentRuntimeBuiltin) == AgentRuntimeA2A {
		unusable = externalDefinitionProblem("agents."+id, def)
		if diag, ok := unusableExternalDiagnostic(id, unusable); ok {
			diags = append(diags, diag)
		}
	}

	modelType := SelectedModelTypeLarge
	var modelRef *AgentModelRef
	if def.Model != nil {
		modelType = def.Model.Type
		modelRef = def.Model.Ref
	}

	return Agent{
		ID:              id,
		Name:            orString(def.Name, id),
		Description:     orString(def.Description, ""),
		Disabled:        def.Disabled != nil && *def.Disabled,
		Model:           modelType,
		AllowedTools:    allowedTools,
		AllowedMCP:      expandMCPAllow(mcpAllow),
		ContextPaths:    def.ContextPaths,
		TodoEnforcement: todoEnforcementFromDefinition(enforcementDef),
		Role:            definitionRole(id, def),
		Runtime:         orString(def.Runtime, AgentRuntimeBuiltin),
		Prompt:          orString(def.Prompt, ""),
		PromptAppend:    orString(def.PromptAppend, ""),
		ModelRef:        modelRef,
		Skills:          def.Skills,
		Workspace:       definitionWorkspace(def),
		Card:            orString(def.Card, ""),
		Auth:            def.Auth,
		Transport:       def.Transport,
		Unusable:        unusable,
	}, diags
}

// definitionWorkspace resolves a definition's workspace: a runtime a2a
// agent always runs with none (#434), whatever it set, since an
// external agent never touches the local disk; any other definition
// keeps its own value.
func definitionWorkspace(def AgentDefinition) string {
	if orString(def.Runtime, AgentRuntimeBuiltin) == AgentRuntimeA2A {
		return AgentWorkspaceNone
	}
	return orString(def.Workspace, "")
}

// todoEnforcementFromDefinition maps a definition's todos and kill
// blocks, or the legacy todo_enforcement alias, onto the resolved
// TodoEnforcementConfig; nil when neither set anything. Validation has
// already rejected a definition that sets the alias together with
// either block, so at most one source feeds the mapping (#402).
func todoEnforcementFromDefinition(def AgentDefinition) *TodoEnforcementConfig {
	if def.TodoEnforcement != nil {
		return def.TodoEnforcement
	}
	if def.Todos == nil && def.Kill == nil {
		return nil
	}
	cfg := &TodoEnforcementConfig{}
	if def.Todos != nil {
		cfg.Enabled = def.Todos.Nudge
		cfg.NudgeThreshold = def.Todos.NudgeAfterToolCalls
		cfg.HardGate = def.Todos.HardGate
	}
	if def.Kill != nil {
		cfg.KillAfterNudges = def.Kill.AfterIgnoredNudges
		cfg.StallWindow = def.Kill.Stall
		cfg.HardTimeout = def.Kill.Timeout
	}
	return cfg
}

func (c *ProviderConfig) TestConnection(resolver VariableResolver) error {
	var (
		providerID = catwalk.InferenceProvider(c.ID)
		testURL    = ""
		headers    = make(map[string]string)
		apiKey, _  = resolver.ResolveValue(c.APIKey)
	)

	switch providerID {
	case catwalk.InferenceProviderMiniMax, catwalk.InferenceProviderMiniMaxChina:
		// NOTE: MiniMax has no good endpoint we can use to validate the API key.
		return nil
	case catwalk.InferenceProviderAlibabaSingapore:
		// NOTE: Alibaba has no good endpoint we can use to validate the API key.
		// Let's at least check the pattern.
		if !strings.HasPrefix(apiKey, "sk-") {
			return fmt.Errorf("invalid API key format for provider %s", c.ID)
		}
		return nil
	}

	switch c.Type {
	case catwalk.TypeOpenAI, catwalk.TypeOpenAICompat, catwalk.TypeOpenRouter:
		baseURL, _ := resolver.ResolveValue(c.BaseURL)
		baseURL = cmp.Or(baseURL, "https://api.openai.com/v1")

		switch providerID {
		case catwalk.InferenceProviderOpenRouter:
			testURL = baseURL + "/credits"
		case catwalk.InferenceProviderOpenCodeGo:
			testURL = strings.Replace(baseURL, "/go", "", 1) + "/models"
		default:
			testURL = baseURL + "/models"
		}

		headers["Authorization"] = "Bearer " + apiKey
	case catwalk.TypeAnthropic:
		baseURL, _ := resolver.ResolveValue(c.BaseURL)
		baseURL = cmp.Or(baseURL, "https://api.anthropic.com/v1")

		switch providerID {
		case catwalk.InferenceKimiCoding:
			testURL = baseURL + "/v1/models"
		default:
			testURL = baseURL + "/models"
		}

		headers["x-api-key"] = apiKey
		headers["anthropic-version"] = "2023-06-01"
	case catwalk.TypeGoogle:
		baseURL, _ := resolver.ResolveValue(c.BaseURL)
		baseURL = cmp.Or(baseURL, "https://generativelanguage.googleapis.com")
		testURL = baseURL + "/v1beta/models?key=" + url.QueryEscape(apiKey)
	case catwalk.TypeBedrock:
		// NOTE: Bedrock has a `/foundation-models` endpoint that we could in
		// theory use, but apparently the authorization is region-specific,
		// so it's not so trivial.
		if strings.HasPrefix(apiKey, "ABSK") { // Bedrock API keys
			return nil
		}
		return errors.New("not a valid bedrock api key")
	case catwalk.TypeVercel:
		// NOTE: Vercel does not validate API keys on the `/models` endpoint.
		if strings.HasPrefix(apiKey, "vck_") { // Vercel API keys
			return nil
		}
		return errors.New("not a valid vercel api key")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for provider %s: %w", c.ID, err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for k, v := range c.ExtraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create request for provider %s: %w", c.ID, err)
	}
	defer resp.Body.Close()

	switch providerID {
	case catwalk.InferenceProviderZAI:
		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("failed to connect to provider %s: %s", c.ID, resp.Status)
		}
	default:
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to connect to provider %s: %s", c.ID, resp.Status)
		}
	}
	return nil
}

// resolveEnvs expands every value in envs through the given resolver
// and returns a fresh "KEY=value" slice sorted by key. The input map is
// not mutated. On the first resolution failure it returns nil and an
// error identifying the offending variable; the inner resolver error is
// already sanitized by ResolveValue and is wrapped with %w.
func resolveEnvs(envs map[string]string, r VariableResolver) ([]string, error) {
	if len(envs) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(envs))
	for k := range envs {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	res := make([]string, 0, len(envs))
	for _, k := range keys {
		v, err := r.ResolveValue(envs[k])
		if err != nil {
			return nil, fmt.Errorf("env %s: %w", k, err)
		}
		res = append(res, fmt.Sprintf("%s=%s", k, v))
	}
	return res, nil
}

func ptrValOr[T any](t *T, el T) T {
	if t == nil {
		return el
	}
	return *t
}

// GetLazyMCP reports whether MCP tool schemas are kept out of the model
// context until the agent asks for them. Both the nil receiver and the
// unset field mean enabled, so laziness is on by default for every
// existing config.
func (o *Options) GetLazyMCP() bool {
	return o == nil || o.LazyMCP == nil || *o.LazyMCP
}

// DefaultSnapshotExclusions lists the paths snapshots never capture when
// the user has not configured their own exclusions.
var DefaultSnapshotExclusions = []string{"node_modules", "vendor", "dist", "build"}

// SnapshotsConfig configures the git-backed filesystem snapshots.
type SnapshotsConfig struct {
	// Enabled is nil when unset, which means snapshots are on.
	Enabled *bool `json:"enabled,omitempty" jsonschema:"description=Capture a filesystem snapshot at each user message into the project's private snapshot store,default=true"`
	// Exclude holds glob or prefix patterns skipped during the snapshot walk.
	Exclude []string `json:"exclude,omitempty" jsonschema:"description=Path patterns excluded from snapshots,example=node_modules"`
	// Keep bounds how many snapshots a session retains: the oldest beyond
	// the newest keep are pruned after each capture. Zero keeps everything.
	Keep int `json:"keep,omitempty" jsonschema:"description=Snapshots kept per session; the oldest beyond this are pruned after each capture. Zero keeps everything,default=0"`
}

// SnapshotsEnabled reports whether snapshots are on; they are on by default.
func (c *Config) SnapshotsEnabled() bool {
	return c.Snapshots == nil || c.Snapshots.Enabled == nil || *c.Snapshots.Enabled
}

// SnapshotsKeep returns how many snapshots a session retains; zero keeps
// everything.
func (c *Config) SnapshotsKeep() int {
	if c.Snapshots == nil {
		return 0
	}
	return max(0, c.Snapshots.Keep)
}

// SnapshotExclusions returns the configured exclusions or the defaults.
func (c *Config) SnapshotExclusions() []string {
	if c.Snapshots == nil || len(c.Snapshots.Exclude) == 0 {
		return DefaultSnapshotExclusions
	}
	return c.Snapshots.Exclude
}

type CompactionOptions struct {
	PruneToolResults *bool `json:"prune_tool_results,omitempty" jsonschema:"description=Replace the output of stale tool calls with a one-line skeleton naming what ran; stored history is never modified,default=true"`
	ObserveMemory    *bool `json:"observe_memory,omitempty" jsonschema:"description=Record session memory at the end of each turn; memory is kept verbatim beside every checkpoint, so decisions survive compaction,default=true"`
	TailTokens       int   `json:"tail_tokens,omitempty" jsonschema:"description=Tokens of recent conversation a compaction keeps verbatim. When unset this is 15% of the model's context window,example=8000,example=40000"`
}

// applySelectedModels stores the resolved selections, dropping any type
// that could not be resolved so callers can detect that no model has been
// chosen yet.
func (c *Config) applySelectedModels(resolved resolvedModels) {
	if c.Models == nil {
		c.Models = make(map[SelectedModelType]SelectedModel)
	}
	// A type is only kept when the user actually selected it: resolution
	// fills defaults for everything else, and a default is not a choice.
	explicit := func(t SelectedModelType) bool {
		s, ok := c.Models[t]
		return ok && s.Model != ""
	}
	for modelType, entry := range map[SelectedModelType]struct {
		model SelectedModel
		ok    bool
	}{
		SelectedModelTypeLarge: {resolved.Large, explicit(SelectedModelTypeLarge) && !resolved.LargeFallback},
		// Small follows large when the user did not name it: deriving the
		// small model from the provider's large pick is resolution, not a
		// fallback, so it counts as selected.
		SelectedModelTypeSmall: {resolved.Small, (explicit(SelectedModelTypeSmall) || explicit(SelectedModelTypeLarge)) && !resolved.SmallFallback},
	} {
		if entry.ok {
			c.Models[modelType] = entry.model
		} else {
			delete(c.Models, modelType)
		}
	}
}

// orderedProviders returns the enabled providers that have models, in
// catalog order first and then alphabetically by ID. The stable order keeps
// model lookups reproducible across runs, unlike map iteration.
func (c *Config) orderedProviders(knownProviders []catwalk.Provider) []ProviderConfig {
	ordered := make([]ProviderConfig, 0, c.Providers.Len())
	seen := make(map[string]bool, c.Providers.Len())
	add := func(providerConfig ProviderConfig) {
		if providerConfig.Disable || len(providerConfig.Models) == 0 || seen[providerConfig.ID] {
			return
		}
		seen[providerConfig.ID] = true
		ordered = append(ordered, providerConfig)
	}

	for _, p := range knownProviders {
		if providerConfig, ok := c.Providers.Get(string(p.ID)); ok {
			add(providerConfig)
		}
	}

	remaining := slices.Collect(c.Providers.Seq())
	slices.SortFunc(remaining, func(a, b ProviderConfig) int {
		return strings.Compare(a.ID, b.ID)
	})
	for _, providerConfig := range remaining {
		add(providerConfig)
	}
	return ordered
}

// FirstAvailableModel returns the first model of the first enabled
// provider, preferring catalog order so the result matches the top of the
// model list shown in the TUI. It exists for non-interactive runs, which
// have no way to prompt for a selection; the result is never persisted.
func (c *Config) FirstAvailableModel(knownProviders []catwalk.Provider) (SelectedModel, bool) {
	for _, providerConfig := range c.orderedProviders(knownProviders) {
		model := providerConfig.Models[0]
		return SelectedModel{
			Provider:        providerConfig.ID,
			Model:           model.ID,
			MaxTokens:       model.DefaultMaxTokens,
			ReasoningEffort: model.DefaultReasoningEffort,
		}, true
	}
	return SelectedModel{}, false
}

// providerForModel finds the enabled provider offering the given model ID.
// It lets a config name a model without naming a provider.
func (c *Config) providerForModel(knownProviders []catwalk.Provider, modelID string) (string, bool) {
	for _, providerConfig := range c.orderedProviders(knownProviders) {
		if slices.ContainsFunc(providerConfig.Models, func(m catwalk.Model) bool {
			return m.ID == modelID
		}) {
			return providerConfig.ID, true
		}
	}
	return "", false
}

// HasSelectedModel reports whether a usable model is selected for the
// given type. It is false on a fresh install even when credentials are
// present in the environment, since Crush no longer guesses a provider.
func (c *Config) HasSelectedModel(modelType SelectedModelType) bool {
	selected, ok := c.Models[modelType]
	if !ok || selected.Provider == "" || selected.Model == "" {
		return false
	}
	return c.GetModel(selected.Provider, selected.Model) != nil
}

// DefaultScrollSpeed is the wheel-to-lines multiplier used when the user has
// not configured one.
const DefaultScrollSpeed = 1.0

// ScrollSpeedValue returns the configured scroll speed multiplier. The nil
// receiver and the unset pointer both mean the default. Values are clamped to
// a small positive minimum so scrolling never inverts or stalls.
func (t *TUIOptions) ScrollSpeedValue() float64 {
	if t == nil || t.ScrollSpeed == nil {
		return DefaultScrollSpeed
	}
	return max(*t.ScrollSpeed, 0.1)
}
