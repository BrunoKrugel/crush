package cmd

import (
	"image/color"
	"os"
	"path/filepath"

	fang "charm.land/fang/v2"
	"charm.land/lipgloss/v2"
	"encoding/json"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// helpThemeName best-effort reads the configured theme name from the JSON
// config layers (data config, then the user config). The full config load is
// too heavy for the help path — it validates agents and wires services — so
// a partial parse of the options.tui.active_theme value is all this needs. An
// empty result falls back to the built-in default theme.
func helpThemeName() string {
	type partialConfig struct {
		Options struct {
			TUI struct {
				ActiveTheme string `json:"active_theme"`
			} `json:"tui"`
		} `json:"options"`
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	paths := []string{
		filepath.Join(home, ".local", "share", "crush", "crush.json"),
		filepath.Join(home, ".config", "crush", "crush.json"),
	}
	if env := os.Getenv("CRUSH_GLOBAL_DATA"); env != "" {
		paths[0] = filepath.Join(env, "crush.json")
	}
	if env := os.Getenv("CRUSH_GLOBAL_CONFIG"); env != "" {
		paths[1] = filepath.Join(env, "crush.json")
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg partialConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		if name := cfg.Options.TUI.ActiveTheme; name != "" {
			return name
		}
	}
	return ""
}

// helpColorScheme maps the active crush theme's palette onto the fang help
// renderer's colorscheme, so `crush -h` follows the theme config instead of
// fang's built-in charmtones. Best-effort: an unknown or missing theme name
// resolves to the built-in default.
func helpColorScheme() fang.ColorScheme {
	palette, err := styles.MergePalette(helpThemeName(), styles.Palette{})
	if err != nil {
		palette, _ = styles.MergePalette("", styles.Palette{})
	}
	c := func(hex string) color.Color {
		if hex == "" {
			return nil
		}
		return lipgloss.Color(hex)
	}
	var cs fang.ColorScheme
	cs.Base = c(palette.FgBase)
	cs.Title = c(palette.Primary)
	cs.Description = c(palette.FgSubtle)
	cs.Codeblock = c(palette.FgBase)
	cs.Program = c(palette.Secondary)
	cs.DimmedArgument = c(palette.FgMoreSubtle)
	cs.Comment = c(palette.FgMostSubtle)
	cs.Flag = c(palette.Success)
	cs.FlagDefault = c(palette.FgSubtle)
	cs.Command = c(palette.Accent)
	cs.QuotedString = c(palette.Warning)
	cs.Argument = c(palette.FgBase)
	cs.Help = c(palette.FgBase)
	cs.Dash = c(palette.Separator)
	cs.ErrorHeader = [2]color.Color{
		c(palette.Warning),
		c(palette.Destructive),
	}
	cs.ErrorDetails = c(palette.Error)
	return cs
}
