package styles

import (
	"fmt"
	"image/color"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

// validHexColor matches "#rgb", "#rrggbb", "#rgba", or "#rrggbbaa".
var validHexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

// validANSIColor matches ANSI color indices 0-255.
var validANSIColor = regexp.MustCompile(`^([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)

// charmtoneByName maps lowercase charmtone color names to their hex values.
// Built once at init time from the charmtone package.
var charmtoneByName map[string]string

func init() {
	charmtoneByName = make(map[string]string, len(charmtone.Keys()))
	for _, k := range charmtone.Keys() {
		charmtoneByName[strings.ToLower(k.String())] = k.Hex()
	}
	// Include aliases.
	charmtoneByName["ash"] = charmtone.Sash.Hex()
	charmtoneByName["charcoal"] = charmtone.Char.Hex()
}

// PaletteField describes a single customizable color token in a Palette.
// It pairs the JSON field name with getter/setter accessors so that
// validation, serialization checks, and UI editors can all iterate
// over the same ordered list without duplicating field names.
type PaletteField struct {
	Get  func(Palette) string
	Set  func(*Palette, string)
	Name string
}

// PaletteFields returns the ordered list of all customizable color
// tokens in a Palette. This is the single source of truth for field
// names, ordering, and access; Validate, knownThemeFields, and the
// theme editor all derive from it.
func PaletteFields() []PaletteField {
	return []PaletteField{
		{Name: "primary", Get: func(p Palette) string { return p.Primary }, Set: func(p *Palette, v string) { p.Primary = v }},
		{Name: "secondary", Get: func(p Palette) string { return p.Secondary }, Set: func(p *Palette, v string) { p.Secondary = v }},
		{Name: "accent", Get: func(p Palette) string { return p.Accent }, Set: func(p *Palette, v string) { p.Accent = v }},
		{Name: "keyword", Get: func(p Palette) string { return p.Keyword }, Set: func(p *Palette, v string) { p.Keyword = v }},
		{Name: "fg_base", Get: func(p Palette) string { return p.FgBase }, Set: func(p *Palette, v string) { p.FgBase = v }},
		{Name: "fg_subtle", Get: func(p Palette) string { return p.FgSubtle }, Set: func(p *Palette, v string) { p.FgSubtle = v }},
		{Name: "fg_more_subtle", Get: func(p Palette) string { return p.FgMoreSubtle }, Set: func(p *Palette, v string) { p.FgMoreSubtle = v }},
		{Name: "fg_most_subtle", Get: func(p Palette) string { return p.FgMostSubtle }, Set: func(p *Palette, v string) { p.FgMostSubtle = v }},
		{Name: "bg_base", Get: func(p Palette) string { return p.BgBase }, Set: func(p *Palette, v string) { p.BgBase = v }},
		{Name: "bg_most_visible", Get: func(p Palette) string { return p.BgMostVisible }, Set: func(p *Palette, v string) { p.BgMostVisible = v }},
		{Name: "bg_less_visible", Get: func(p Palette) string { return p.BgLessVisible }, Set: func(p *Palette, v string) { p.BgLessVisible = v }},
		{Name: "bg_least_visible", Get: func(p Palette) string { return p.BgLeastVisible }, Set: func(p *Palette, v string) { p.BgLeastVisible = v }},
		{Name: "on_primary", Get: func(p Palette) string { return p.OnPrimary }, Set: func(p *Palette, v string) { p.OnPrimary = v }},
		{Name: "separator", Get: func(p Palette) string { return p.Separator }, Set: func(p *Palette, v string) { p.Separator = v }},
		{Name: "destructive", Get: func(p Palette) string { return p.Destructive }, Set: func(p *Palette, v string) { p.Destructive = v }},
		{Name: "error", Get: func(p Palette) string { return p.Error }, Set: func(p *Palette, v string) { p.Error = v }},
		{Name: "warning", Get: func(p Palette) string { return p.Warning }, Set: func(p *Palette, v string) { p.Warning = v }},
		{Name: "warning_subtle", Get: func(p Palette) string { return p.WarningSubtle }, Set: func(p *Palette, v string) { p.WarningSubtle = v }},
		{Name: "attention", Get: func(p Palette) string { return p.Attention }, Set: func(p *Palette, v string) { p.Attention = v }},
		{Name: "busy", Get: func(p Palette) string { return p.Busy }, Set: func(p *Palette, v string) { p.Busy = v }},
		{Name: "info", Get: func(p Palette) string { return p.Info }, Set: func(p *Palette, v string) { p.Info = v }},
		{Name: "info_more_subtle", Get: func(p Palette) string { return p.InfoMoreSubtle }, Set: func(p *Palette, v string) { p.InfoMoreSubtle = v }},
		{Name: "info_most_subtle", Get: func(p Palette) string { return p.InfoMostSubtle }, Set: func(p *Palette, v string) { p.InfoMostSubtle = v }},
		{Name: "success", Get: func(p Palette) string { return p.Success }, Set: func(p *Palette, v string) { p.Success = v }},
		{Name: "success_more_subtle", Get: func(p Palette) string { return p.SuccessMoreSubtle }, Set: func(p *Palette, v string) { p.SuccessMoreSubtle = v }},
		{Name: "success_most_subtle", Get: func(p Palette) string { return p.SuccessMostSubtle }, Set: func(p *Palette, v string) { p.SuccessMostSubtle = v }},
		{Name: "diff_insert_fg", Get: func(p Palette) string { return p.DiffInsertFg }, Set: func(p *Palette, v string) { p.DiffInsertFg = v }},
		{Name: "diff_insert_code_bg", Get: func(p Palette) string { return p.DiffInsertCodeBg }, Set: func(p *Palette, v string) { p.DiffInsertCodeBg = v }},
		{Name: "diff_insert_gutter_bg", Get: func(p Palette) string { return p.DiffInsertGutterBg }, Set: func(p *Palette, v string) { p.DiffInsertGutterBg = v }},
		{Name: "diff_delete_fg", Get: func(p Palette) string { return p.DiffDeleteFg }, Set: func(p *Palette, v string) { p.DiffDeleteFg = v }},
		{Name: "diff_delete_code_bg", Get: func(p Palette) string { return p.DiffDeleteCodeBg }, Set: func(p *Palette, v string) { p.DiffDeleteCodeBg = v }},
		{Name: "diff_delete_gutter_bg", Get: func(p Palette) string { return p.DiffDeleteGutterBg }, Set: func(p *Palette, v string) { p.DiffDeleteGutterBg = v }},
		{Name: "yolo", Get: func(p Palette) string { return p.Yolo }, Set: func(p *Palette, v string) { p.Yolo = v }},
		{Name: "plan", Get: func(p Palette) string { return p.Plan }, Set: func(p *Palette, v string) { p.Plan = v }},
		{Name: "plan_more_subtle", Get: func(p Palette) string { return p.PlanMoreSubtle }, Set: func(p *Palette, v string) { p.PlanMoreSubtle = v }},
		{Name: "button", Get: func(p Palette) string { return p.Button }, Set: func(p *Palette, v string) { p.Button = v }},
		{Name: "button_subtle", Get: func(p Palette) string { return p.ButtonSubtle }, Set: func(p *Palette, v string) { p.ButtonSubtle = v }},
		{Name: "button_inactive", Get: func(p Palette) string { return p.ButtonInactive }, Set: func(p *Palette, v string) { p.ButtonInactive = v }},
		{Name: "button_hovered", Get: func(p Palette) string { return p.ButtonHovered }, Set: func(p *Palette, v string) { p.ButtonHovered = v }},
		{Name: "ansi_black", Get: func(p Palette) string { return p.AnsiBlack }, Set: func(p *Palette, v string) { p.AnsiBlack = v }},
		{Name: "ansi_red", Get: func(p Palette) string { return p.AnsiRed }, Set: func(p *Palette, v string) { p.AnsiRed = v }},
		{Name: "ansi_green", Get: func(p Palette) string { return p.AnsiGreen }, Set: func(p *Palette, v string) { p.AnsiGreen = v }},
		{Name: "ansi_yellow", Get: func(p Palette) string { return p.AnsiYellow }, Set: func(p *Palette, v string) { p.AnsiYellow = v }},
		{Name: "ansi_blue", Get: func(p Palette) string { return p.AnsiBlue }, Set: func(p *Palette, v string) { p.AnsiBlue = v }},
		{Name: "ansi_magenta", Get: func(p Palette) string { return p.AnsiMagenta }, Set: func(p *Palette, v string) { p.AnsiMagenta = v }},
		{Name: "ansi_cyan", Get: func(p Palette) string { return p.AnsiCyan }, Set: func(p *Palette, v string) { p.AnsiCyan = v }},
		{Name: "ansi_white", Get: func(p Palette) string { return p.AnsiWhite }, Set: func(p *Palette, v string) { p.AnsiWhite = v }},
		{Name: "ansi_bright_black", Get: func(p Palette) string { return p.AnsiBrightBlack }, Set: func(p *Palette, v string) { p.AnsiBrightBlack = v }},
		{Name: "ansi_bright_red", Get: func(p Palette) string { return p.AnsiBrightRed }, Set: func(p *Palette, v string) { p.AnsiBrightRed = v }},
		{Name: "ansi_bright_green", Get: func(p Palette) string { return p.AnsiBrightGreen }, Set: func(p *Palette, v string) { p.AnsiBrightGreen = v }},
		{Name: "ansi_bright_yellow", Get: func(p Palette) string { return p.AnsiBrightYellow }, Set: func(p *Palette, v string) { p.AnsiBrightYellow = v }},
		{Name: "ansi_bright_blue", Get: func(p Palette) string { return p.AnsiBrightBlue }, Set: func(p *Palette, v string) { p.AnsiBrightBlue = v }},
		{Name: "ansi_bright_magenta", Get: func(p Palette) string { return p.AnsiBrightMagenta }, Set: func(p *Palette, v string) { p.AnsiBrightMagenta = v }},
		{Name: "ansi_bright_cyan", Get: func(p Palette) string { return p.AnsiBrightCyan }, Set: func(p *Palette, v string) { p.AnsiBrightCyan = v }},
		{Name: "ansi_bright_white", Get: func(p Palette) string { return p.AnsiBrightWhite }, Set: func(p *Palette, v string) { p.AnsiBrightWhite = v }},
	}
}

// Palette is a JSON-serializable theme palette. Each field maps to a
// quickStyleOpts color, stored as a hex string ("#rrggbb"). Empty
// strings mean "inherit from base" when merging.
type Palette struct {
	Primary   string `json:"primary,omitempty"`
	Secondary string `json:"secondary,omitempty"`
	Accent    string `json:"accent,omitempty"`
	Keyword   string `json:"keyword,omitempty"`

	FgBase       string `json:"fg_base,omitempty"`
	FgSubtle     string `json:"fg_subtle,omitempty"`
	FgMoreSubtle string `json:"fg_more_subtle,omitempty"`
	FgMostSubtle string `json:"fg_most_subtle,omitempty"`

	BgBase         string `json:"bg_base,omitempty"`
	BgMostVisible  string `json:"bg_most_visible,omitempty"`
	BgLessVisible  string `json:"bg_less_visible,omitempty"`
	BgLeastVisible string `json:"bg_least_visible,omitempty"`

	OnPrimary string `json:"on_primary,omitempty"`
	Separator string `json:"separator,omitempty"`

	Destructive       string `json:"destructive,omitempty"`
	Error             string `json:"error,omitempty"`
	Warning           string `json:"warning,omitempty"`
	WarningSubtle     string `json:"warning_subtle,omitempty"`
	Attention         string `json:"attention,omitempty"`
	Busy              string `json:"busy,omitempty"`
	Info              string `json:"info,omitempty"`
	InfoMoreSubtle    string `json:"info_more_subtle,omitempty"`
	InfoMostSubtle    string `json:"info_most_subtle,omitempty"`
	Success           string `json:"success,omitempty"`
	SuccessMoreSubtle string `json:"success_more_subtle,omitempty"`
	SuccessMostSubtle string `json:"success_most_subtle,omitempty"`

	DiffInsertFg       string `json:"diff_insert_fg,omitempty"`
	DiffInsertCodeBg   string `json:"diff_insert_code_bg,omitempty"`
	DiffInsertGutterBg string `json:"diff_insert_gutter_bg,omitempty"`
	DiffDeleteFg       string `json:"diff_delete_fg,omitempty"`
	DiffDeleteCodeBg   string `json:"diff_delete_code_bg,omitempty"`
	DiffDeleteGutterBg string `json:"diff_delete_gutter_bg,omitempty"`

	Yolo           string `json:"yolo,omitempty"`
	Plan           string `json:"plan,omitempty"`
	PlanMoreSubtle string `json:"plan_more_subtle,omitempty"`

	Button         string `json:"button,omitempty"`
	ButtonSubtle   string `json:"button_subtle,omitempty"`
	ButtonInactive string `json:"button_inactive,omitempty"`
	ButtonHovered  string `json:"button_hovered,omitempty"`

	AnsiBlack         string `json:"ansi_black,omitempty"`
	AnsiRed           string `json:"ansi_red,omitempty"`
	AnsiGreen         string `json:"ansi_green,omitempty"`
	AnsiYellow        string `json:"ansi_yellow,omitempty"`
	AnsiBlue          string `json:"ansi_blue,omitempty"`
	AnsiMagenta       string `json:"ansi_magenta,omitempty"`
	AnsiCyan          string `json:"ansi_cyan,omitempty"`
	AnsiWhite         string `json:"ansi_white,omitempty"`
	AnsiBrightBlack   string `json:"ansi_bright_black,omitempty"`
	AnsiBrightRed     string `json:"ansi_bright_red,omitempty"`
	AnsiBrightGreen   string `json:"ansi_bright_green,omitempty"`
	AnsiBrightYellow  string `json:"ansi_bright_yellow,omitempty"`
	AnsiBrightBlue    string `json:"ansi_bright_blue,omitempty"`
	AnsiBrightMagenta string `json:"ansi_bright_magenta,omitempty"`
	AnsiBrightCyan    string `json:"ansi_bright_cyan,omitempty"`
	AnsiBrightWhite   string `json:"ansi_bright_white,omitempty"`
}

// PaletteFromOpts extracts a Palette from quickStyleOpts, converting
// each color.Color to its "#rrggbb" hex representation. Diff tokens
// left unset stay empty so overrides can flow through the merge chain;
// derivation happens once, after all overrides are applied (see
// deriveDiffColors).
func PaletteFromOpts(o quickStyleOpts) Palette {
	return Palette{
		Primary:   colorToHex(o.primary),
		Secondary: colorToHex(o.secondary),
		Accent:    colorToHex(o.accent),
		Keyword:   colorToHex(o.keyword),

		FgBase:       colorToHex(o.fgBase),
		FgSubtle:     colorToHex(o.fgSubtle),
		FgMoreSubtle: colorToHex(o.fgMoreSubtle),
		FgMostSubtle: colorToHex(o.fgMostSubtle),

		BgBase:         colorToHex(o.bgBase),
		BgMostVisible:  colorToHex(o.bgMostVisible),
		BgLessVisible:  colorToHex(o.bgLessVisible),
		BgLeastVisible: colorToHex(o.bgLeastVisible),

		OnPrimary: colorToHex(o.onPrimary),
		Separator: colorToHex(o.separator),

		Destructive:       colorToHex(o.destructive),
		Error:             colorToHex(o.error),
		Warning:           colorToHex(o.warning),
		WarningSubtle:     colorToHex(o.warningSubtle),
		Attention:         colorToHex(o.attention),
		Busy:              colorToHex(o.busy),
		Info:              colorToHex(o.info),
		InfoMoreSubtle:    colorToHex(o.infoMoreSubtle),
		InfoMostSubtle:    colorToHex(o.infoMostSubtle),
		Success:           colorToHex(o.success),
		SuccessMoreSubtle: colorToHex(o.successMoreSubtle),
		SuccessMostSubtle: colorToHex(o.successMostSubtle),

		DiffInsertFg:       colorToHex(o.diffInsertFg),
		DiffInsertCodeBg:   colorToHex(o.diffInsertCodeBg),
		DiffInsertGutterBg: colorToHex(o.diffInsertGutterBg),
		DiffDeleteFg:       colorToHex(o.diffDeleteFg),
		DiffDeleteCodeBg:   colorToHex(o.diffDeleteCodeBg),
		DiffDeleteGutterBg: colorToHex(o.diffDeleteGutterBg),

		Yolo:           colorToHex(o.yolo),
		Plan:           colorToHex(o.plan),
		PlanMoreSubtle: colorToHex(o.planMoreSubtle),

		Button:         colorToHex(o.button),
		ButtonSubtle:   colorToHex(o.buttonSubtle),
		ButtonInactive: colorToHex(o.buttonInactive),
		ButtonHovered:  colorToHex(o.buttonHovered),

		AnsiBlack:         colorToHex(o.ansiBlack),
		AnsiRed:           colorToHex(o.ansiRed),
		AnsiGreen:         colorToHex(o.ansiGreen),
		AnsiYellow:        colorToHex(o.ansiYellow),
		AnsiBlue:          colorToHex(o.ansiBlue),
		AnsiMagenta:       colorToHex(o.ansiMagenta),
		AnsiCyan:          colorToHex(o.ansiCyan),
		AnsiWhite:         colorToHex(o.ansiWhite),
		AnsiBrightBlack:   colorToHex(o.ansiBrightBlack),
		AnsiBrightRed:     colorToHex(o.ansiBrightRed),
		AnsiBrightGreen:   colorToHex(o.ansiBrightGreen),
		AnsiBrightYellow:  colorToHex(o.ansiBrightYellow),
		AnsiBrightBlue:    colorToHex(o.ansiBrightBlue),
		AnsiBrightMagenta: colorToHex(o.ansiBrightMagenta),
		AnsiBrightCyan:    colorToHex(o.ansiBrightCyan),
		AnsiBrightWhite:   colorToHex(o.ansiBrightWhite),
	}
}

// ToQuickStyleOpts converts a Palette back to quickStyleOpts. Non-empty
// fields are parsed as hex colors; empty fields fall back to the
// provided base palette.
func (p Palette) ToQuickStyleOpts(base quickStyleOpts) quickStyleOpts {
	return quickStyleOpts{
		primary:   resolveColor(p.Primary, base.primary),
		secondary: resolveColor(p.Secondary, base.secondary),
		accent:    resolveColor(p.Accent, base.accent),
		keyword:   resolveColor(p.Keyword, base.keyword),

		fgBase:       resolveColor(p.FgBase, base.fgBase),
		fgSubtle:     resolveColor(p.FgSubtle, base.fgSubtle),
		fgMoreSubtle: resolveColor(p.FgMoreSubtle, base.fgMoreSubtle),
		fgMostSubtle: resolveColor(p.FgMostSubtle, base.fgMostSubtle),

		bgBase:         resolveColor(p.BgBase, base.bgBase),
		bgMostVisible:  resolveColor(p.BgMostVisible, base.bgMostVisible),
		bgLessVisible:  resolveColor(p.BgLessVisible, base.bgLessVisible),
		bgLeastVisible: resolveColor(p.BgLeastVisible, base.bgLeastVisible),

		onPrimary: resolveColor(p.OnPrimary, base.onPrimary),
		separator: resolveColor(p.Separator, base.separator),

		destructive:       resolveColor(p.Destructive, base.destructive),
		error:             resolveColor(p.Error, base.error),
		warning:           resolveColor(p.Warning, base.warning),
		warningSubtle:     resolveColor(p.WarningSubtle, base.warningSubtle),
		attention:         resolveColor(p.Attention, base.attention),
		busy:              resolveColor(p.Busy, base.busy),
		info:              resolveColor(p.Info, base.info),
		infoMoreSubtle:    resolveColor(p.InfoMoreSubtle, base.infoMoreSubtle),
		infoMostSubtle:    resolveColor(p.InfoMostSubtle, base.infoMostSubtle),
		success:           resolveColor(p.Success, base.success),
		successMoreSubtle: resolveColor(p.SuccessMoreSubtle, base.successMoreSubtle),
		successMostSubtle: resolveColor(p.SuccessMostSubtle, base.successMostSubtle),

		diffInsertFg:       resolveColor(p.DiffInsertFg, base.diffInsertFg),
		diffInsertCodeBg:   resolveColor(p.DiffInsertCodeBg, base.diffInsertCodeBg),
		diffInsertGutterBg: resolveColor(p.DiffInsertGutterBg, base.diffInsertGutterBg),
		diffDeleteFg:       resolveColor(p.DiffDeleteFg, base.diffDeleteFg),
		diffDeleteCodeBg:   resolveColor(p.DiffDeleteCodeBg, base.diffDeleteCodeBg),
		diffDeleteGutterBg: resolveColor(p.DiffDeleteGutterBg, base.diffDeleteGutterBg),

		yolo:           resolveColor(p.Yolo, base.yolo),
		plan:           resolveColor(p.Plan, base.plan),
		planMoreSubtle: resolveColor(p.PlanMoreSubtle, base.planMoreSubtle),

		button:         resolveColor(p.Button, base.button),
		buttonSubtle:   resolveColor(p.ButtonSubtle, base.buttonSubtle),
		buttonInactive: resolveColor(p.ButtonInactive, base.buttonInactive),
		buttonHovered:  resolveColor(p.ButtonHovered, base.buttonHovered),

		ansiBlack:   resolveColor(p.AnsiBlack, base.ansiBlack),
		ansiRed:     resolveColor(p.AnsiRed, base.ansiRed),
		ansiGreen:   resolveColor(p.AnsiGreen, base.ansiGreen),
		ansiYellow:  resolveColor(p.AnsiYellow, base.ansiYellow),
		ansiBlue:    resolveColor(p.AnsiBlue, base.ansiBlue),
		ansiMagenta: resolveColor(p.AnsiMagenta, base.ansiMagenta),
		ansiCyan:    resolveColor(p.AnsiCyan, base.ansiCyan),
		ansiWhite:   resolveColor(p.AnsiWhite, base.ansiWhite),

		ansiBrightBlack:   resolveColor(p.AnsiBrightBlack, base.ansiBrightBlack),
		ansiBrightRed:     resolveColor(p.AnsiBrightRed, base.ansiBrightRed),
		ansiBrightGreen:   resolveColor(p.AnsiBrightGreen, base.ansiBrightGreen),
		ansiBrightYellow:  resolveColor(p.AnsiBrightYellow, base.ansiBrightYellow),
		ansiBrightBlue:    resolveColor(p.AnsiBrightBlue, base.ansiBrightBlue),
		ansiBrightMagenta: resolveColor(p.AnsiBrightMagenta, base.ansiBrightMagenta),
		ansiBrightCyan:    resolveColor(p.AnsiBrightCyan, base.ansiBrightCyan),
		ansiBrightWhite:   resolveColor(p.AnsiBrightWhite, base.ansiBrightWhite),
	}
}

// IsValidColor reports whether s is a valid color value: a hex code
// (#rgb, #rrggbb, #rgba, #rrggbbaa), an ANSI index (0-255), or a named
// charmtone color (case-insensitive).
func IsValidColor(s string) bool {
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "#") {
		return validHexColor.MatchString(s)
	}
	if validANSIColor.MatchString(s) {
		return true
	}
	_, ok := charmtoneByName[strings.ToLower(s)]
	return ok
}

// ParseColor normalizes a supported color for storage and display. It accepts
// hex codes, ANSI indices (0-255), and named Charmtone colors. Colors matching
// a Charmtone entry use its canonical name; other valid values are returned
// unchanged. Unrecognized input returns an empty string.
func ParseColor(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "#") {
		if validHexColor.MatchString(s) {
			if name := charmtone.NameFromHex(s); name != "" {
				return name
			}
			return s
		}
		return ""
	}
	if validANSIColor.MatchString(s) {
		return s
	}
	if hex, ok := charmtoneByName[strings.ToLower(s)]; ok {
		if name := charmtone.NameFromHex(hex); name != "" {
			return name
		}
		return hex
	}
	return ""
}

// Validate checks that all non-empty color strings in the palette are
// valid color values (hex, ANSI 0-255, or charmtone name). Returns an
// error listing all invalid fields.
func (p Palette) Validate() error {
	var errs []string
	for _, f := range PaletteFields() {
		if v := f.Get(p); v != "" && !IsValidColor(v) {
			errs = append(errs, fmt.Sprintf("%s: invalid color %q", f.Name, v))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid palette colors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// IsEmpty reports whether no palette colors are set, meaning the theme
// inherits everything from its base.
func (p Palette) IsEmpty() bool {
	for _, f := range PaletteFields() {
		if f.Get(p) != "" {
			return false
		}
	}
	return true
}

// ThemePalette returns the Palette for a built-in theme by name.
// Returns an error if the theme is not recognized.
func ThemePalette(name string) (Palette, error) {
	optsFn, ok := builtinThemes[normalizeThemeName(name)]
	if !ok {
		return Palette{}, fmt.Errorf("unknown theme %q; available themes: %s", name, strings.Join(BuiltinThemeNames(), ", "))
	}
	opts := optsFn()
	opts.deriveDiffColors()
	return PaletteFromOpts(opts), nil
}

// MergePalette applies palette overrides on top of a built-in base theme
// and returns the fully resolved palette. Empty baseName uses Charmtone.
// MergePalette applies palette overrides on top of a built-in or user theme
// and returns the fully resolved palette. Empty baseName uses Charmtone.
func MergePalette(baseName string, palette Palette) (Palette, error) {
	base, root, err := resolveThemePalette(baseName, map[string]bool{})
	if err != nil {
		return Palette{}, err
	}
	if err := palette.Validate(); err != nil {
		return Palette{}, err
	}
	rootOpts, err := builtinThemeOpts(root)
	if err != nil {
		return Palette{}, err
	}
	opts := palette.ToQuickStyleOpts(base.ToQuickStyleOpts(rootOpts))
	opts.deriveDiffColors()
	return PaletteFromOpts(opts), nil
}

// baseThemeOpts returns the quickStyleOpts of the named built-in theme.
// An empty name yields the default Charmtone palette.
// resolveThemePalette returns the fully resolved palette and root built-in for
// a theme. User themes may inherit from other user themes; cycles are rejected.
func resolveThemePalette(name string, visiting map[string]bool) (Palette, string, error) {
	if name == "" {
		name = "charmtone-panther"
	}
	key := normalizeThemeName(name)
	if visiting[key] {
		return Palette{}, "", fmt.Errorf("theme inheritance cycle at %q", key)
	}
	visiting[key] = true
	defer delete(visiting, key)

	if path, err := FindThemeFile(key); err == nil {
		tf, err := LoadThemeFile(path)
		if err != nil {
			return Palette{}, "", err
		}
		baseName := tf.Base
		if baseName == "" {
			baseName = "charmtone-panther"
		}
		var base Palette
		var root string
		if strings.EqualFold(baseName, key) && IsBuiltinTheme(key) {
			opts, err := builtinThemeOpts(key)
			if err != nil {
				return Palette{}, "", err
			}
			base = PaletteFromOpts(opts)
			root = key
		} else {
			base, root, err = resolveThemePalette(baseName, visiting)
			if err != nil {
				return Palette{}, "", err
			}
		}
		if err := tf.Validate(); err != nil {
			return Palette{}, "", err
		}
		rootOpts, err := builtinThemeOpts(root)
		if err != nil {
			return Palette{}, "", err
		}
		return PaletteFromOpts(tf.ToQuickStyleOpts(base.ToQuickStyleOpts(rootOpts))), root, nil
	}

	optsFn, ok := builtinThemes[key]
	if !ok {
		return Palette{}, "", fmt.Errorf("unknown theme %q; available themes: %s", name, strings.Join(BuiltinThemeNames(), ", "))
	}
	return PaletteFromOpts(optsFn()), key, nil
}

// builtinThemeOpts returns the quickStyleOpts of a built-in theme.
func builtinThemeOpts(name string) (quickStyleOpts, error) {
	if name == "" {
		name = "charmtone-panther"
	}
	optsFn, ok := builtinThemes[normalizeThemeName(name)]
	if !ok {
		return quickStyleOpts{}, fmt.Errorf("unknown theme %q; available themes: %s", name, strings.Join(BuiltinThemeNames(), ", "))
	}
	return optsFn(), nil
}

// LoadPaletteTheme builds Styles by applying palette overrides on top of
// a built-in base theme. Empty baseName uses the default Charmtone theme.
// LoadPaletteTheme builds Styles by applying palette overrides on top of a
// built-in or user theme. Empty baseName uses the default Charmtone theme.
func LoadPaletteTheme(baseName string, palette Palette) (Styles, error) {
	base, root, err := resolveThemePalette(baseName, map[string]bool{})
	if err != nil {
		return Styles{}, err
	}
	if err := palette.Validate(); err != nil {
		return Styles{}, err
	}
	rootOpts, err := builtinThemeOpts(root)
	if err != nil {
		return Styles{}, err
	}
	s := quickStyle(palette.ToQuickStyleOpts(base.ToQuickStyleOpts(rootOpts)))
	// Root theme overrides hardcode built-in colors (e.g. the Charmtone
	// syntax palette) that only make sense for the untouched built-in.
	// Only apply them when the theme is unmodified, so user themes (e.g.
	// light themes) get fully token-driven styles.
	pure := palette.IsEmpty() && base == PaletteFromOpts(rootOpts)
	if pure {
		if override, ok := builtinThemeOverrides[root]; ok {
			s = override(s)
		}
	}
	return s, nil
}

// colorToHex returns a canonical display string for a color. If the color
// matches a Charmtone palette entry, its name is returned; otherwise the
// "#rrggbb" hex value is returned. Nil colors return an empty string.
func colorToHex(c color.Color) string {
	if c == nil {
		return ""
	}
	r, g, b, _ := c.RGBA()
	hex := fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
	if name := charmtone.NameFromHex(hex); name != "" {
		return name
	}
	return hex
}

// ColorString returns a lipgloss-compatible color string for the given
// input. Charmtone names are converted to hex; hex and ANSI values are
// returned as-is. Returns empty string for unrecognized input.
func ColorString(s string) string {
	resolved := ParseColor(s)
	if resolved == "" {
		return ""
	}
	if hex, ok := charmtoneByName[strings.ToLower(resolved)]; ok {
		return hex
	}
	return resolved
}

// resolveColor parses a color string into a color.Color, falling back
// to the provided default when the string is empty. Supports hex codes,
// ANSI indices, and charmtone names.
func resolveColor(s string, fallback color.Color) color.Color {
	if s == "" {
		return fallback
	}
	cs := ColorString(s)
	if cs == "" {
		return fallback
	}
	return lipgloss.Color(cs)
}
