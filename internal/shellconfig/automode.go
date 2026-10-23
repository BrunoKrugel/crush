package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
)

// handleAutoMode implements the `auto_mode` builtin.
//
// Usage: auto_mode <key> [value]
//
// Configures the native auto mode, an in-process safety classifier that
// runs before permission prompts and grants, denies, or defers to the
// prompt (see internal/automode and config.AutoModeConfig). Auto mode is
// user-level only: a repository must not be able to grant itself auto
// mode, so this builtin belongs in the user's crushrc, not in a project
// one — and the loader ignores a project-level auto_mode section anyway.
//
// Examples:
//
//	auto_mode on
//	auto_mode model local-llama/gemma4-e4b
//	auto_mode base-url http://127.0.0.1:8080/v1
//	auto_mode api-key "$AUTOMODE_API_KEY"
//	auto_mode max-tokens 2048
//	auto_mode timeout 90
//	auto_mode fail-open false
//	auto_mode environment "Trusted repo: my git remotes"
//	auto_mode prompt stage1 ./prompts/stage1.tmpl
//	auto_mode transcript-max-chars 12000
//	auto_mode max-consecutive-denials 3
//	auto_mode max-total-denials 20
//
// Boolean shortcut: for boolean keys, omitting the value sets it to true.
// The on/off shortcut mirrors the upstream builtin.
func handleAutoMode(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: auto_mode on|off | auto_mode model <provider>/<id> | auto_mode <setting> [value]")
	}

	key := args[1]
	am := b.section("auto_mode")

	var val string
	if len(args) >= 3 {
		val = args[2]
	}

	// on/off are the upstream shortcut spellings for enabled true/false.
	if key == "on" {
		key, val = "enabled", "true"
	}
	if key == "off" {
		key, val = "enabled", "false"
	}

	switch key {
	case "model":
		if len(args) < 3 {
			return usage(stderr, "usage: auto_mode model <provider>/<id>")
		}
		provider, id, ok := splitProviderModel(args[2])
		if !ok {
			return usage(stderr, fmt.Sprintf("auto_mode model: expected <provider>/<id>, got %q", args[2]))
		}
		classifier := childMap(am, "classifier")
		classifier["provider"] = provider
		classifier["model"] = id
		slog.Info("Auto mode classifier selected in shell config", "provider", provider, "model", id)
		return nil
	case "prompt":
		if len(args) < 4 {
			return usage(stderr, "usage: auto_mode prompt stage1|stage2 <file>")
		}
		stage := args[2]
		if stage != "stage1" && stage != "stage2" {
			return usage(stderr, fmt.Sprintf("auto_mode prompt: expected stage1 or stage2, got %q", stage))
		}
		am["prompt_"+stage+"_file"] = args[3]
		slog.Info("Auto mode prompt override set in shell config", "stage", stage, "file", args[3])
		return nil
	case "environment":
		if len(args) < 3 {
			return usage(stderr, `usage: auto_mode environment "prose describing trusted repos/domains"`)
		}
		am["environment"] = appendArr(am, "environment", strings.Join(args[2:], " "))
		slog.Info("Auto mode environment entry added in shell config")
		return nil
	}

	spec, ok := autoModeSpecs[key]
	if !ok {
		return usage(stderr, fmt.Sprintf("auto_mode: unknown key %q", key))
	}

	switch spec.kind {
	case optBool:
		bv := true
		if val != "" {
			parsed, err := parseBool(val)
			if err != nil {
				return usage(stderr, fmt.Sprintf("auto_mode: %s expects true/false, got %q", key, val))
			}
			bv = parsed
		}
		am[spec.jsonKey] = bv

	case optInt:
		if val == "" {
			return usage(stderr, fmt.Sprintf("auto_mode: %s requires a value", key))
		}
		n, err := strconv.Atoi(val)
		if err != nil || n <= 0 {
			return usage(stderr, fmt.Sprintf("auto_mode: %s expects a positive number, got %q", key, val))
		}
		am[spec.jsonKey] = n

	default: // optString
		if val == "" {
			return usage(stderr, fmt.Sprintf("auto_mode: %s requires a value", key))
		}
		target, jsonKey := am, spec.jsonKey
		if spec.child != "" {
			target = childMap(am, spec.child)
			jsonKey = spec.field
		}
		target[jsonKey] = val
	}

	// Never log the value of a credential.
	if spec.secret {
		slog.Info("Auto mode option set in shell config", "key", key)
	} else {
		slog.Info("Auto mode option set in shell config", "key", key, "value", am[spec.jsonKey])
	}
	return nil
}

// autoModeSpec describes one user-facing auto_mode key: the JSON field
// it writes, its value type, the nested classifier object it may target,
// and whether it holds a credential that must stay out of logs.
type autoModeSpec struct {
	jsonKey string
	kind    optionKind
	child   string
	field   string
	secret  bool
}

// autoModeSpecs maps user-facing kebab-case keys onto the AutoModeConfig
// fields. Keys mirror the JSON field names so the two config formats stay
// legible as the same thing written two ways.
var autoModeSpecs = map[string]autoModeSpec{
	"enabled":                 {jsonKey: "enabled", kind: optBool},
	"fail-open":               {jsonKey: "fail_open", kind: optBool},
	"base-url":                {kind: optString, child: "classifier", field: "base_url"},
	"api-key":                 {kind: optString, child: "classifier", field: "api_key", secret: true},
	"max-tokens":              {jsonKey: "max_tokens", kind: optInt},
	"timeout":                 {jsonKey: "timeout_seconds", kind: optInt},
	"transcript-max-chars":    {jsonKey: "transcript_max_chars", kind: optInt},
	"max-consecutive-denials": {jsonKey: "max_consecutive_denials", kind: optInt},
	"max-total-denials":       {jsonKey: "max_total_denials", kind: optInt},
}
