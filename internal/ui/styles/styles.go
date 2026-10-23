// Package styles define styling and theming for the project.
package styles

import (
	"fmt"
	"image/color"
	"log/slog"
	"strings"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textinput"
	"charm.land/glamour/v2/ansi"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/bytedance/sonic"
	"github.com/charmbracelet/crush/internal/ui/diffview"
	"github.com/charmbracelet/crush/internal/ui/textarea"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	CheckIcon       string = "✓"
	SpinnerIcon     string = "⋯"
	LoadingIcon     string = "⟳"
	ModelIcon       string = "◇"
	HypercreditIcon string = "◆"

	// AgentIcon marks a dispatched sub-agent surface: the @ popup's agent
	// rows, the inspect-mode sidebar banner, and the agent tag on an
	// inspected transcript's metadata lines. Nerd Font md-robot; renders
	// as one cell on Nerd Font terminals and as a replacement glyph
	// elsewhere.
	AgentIcon string = "󰚩"

	// GitBranchIcon is the Powerline git branch glyph (U+E0A0), shown beside
	// the checked-out branch when the terminal is expected to render Nerd
	// Font glyphs.
	GitBranchIcon string = "\ue0a0"

	ArrowRightIcon string = "→"

	// CodespanPadding is the padding rendered around inline code spans in
	// markdown. It displays identically to the blank padding it replaced,
	// but selection copies recognize it and turn it back into the original
	// backticks (see list.HighlightContent).
	//
	// It is a no-break space tagged with the text-presentation variation
	// selector (U+FE0E): the pair is a single one-cell grapheme that
	// renders as a blank and, being non-breaking, keeps word wrap from
	// tearing a codespan between its padding and its text. The selector
	// makes the sentinel distinct from a real no-break space in message
	// text (pasted text, some LLM output), which a selection copy must
	// preserve verbatim.
	//
	// The selector must be U+FE0E, not the emoji-presentation U+FE0F:
	// ansi.StringWidth measures any cluster ending in U+FE0F as two
	// cells wide, while terminals render it as one, and that mismatch
	// makes the frame differ repaint the line on every frame (visible
	// as flicker).
	CodespanPadding string = "\u00a0\ufe0e"

	ToolPending string = "●"
	ToolSuccess string = "✓"
	ToolError   string = "×"

	RadioOn  string = "◉"
	RadioOff string = "○"

	BorderThin  string = "│"
	BorderThick string = "▌"

	SectionSeparator string = "─"

	TodoCompletedIcon  string = "✓"
	TodoPendingIcon    string = "•"
	TodoInProgressIcon string = "→"

	CronRecurringIcon string = "↻"
	CronOneShotIcon   string = "⏱"
	CronDeletedIcon   string = "✕"

	ImageIcon string = "▣"
	TextIcon  string = "≡"
	SkillIcon string = "▲"
	// PromptIcon marks a resolved MCP prompt. Single-width and geometric to
	// match the icons above: an emoji here would be double-width and
	// colour-rendered, breaking both the row's look and its width math.
	PromptIcon string = "◈"
	RemoveIcon string = "✕"

	ColorSwatchIcon string = "■"

	ScrollbarThumb string = "┃"
	ScrollbarTrack string = "│"

	ChannelIcon string = "󰍩"

	// LSP severity icons are Nerd Font glyphs, shown only when the terminal
	// is expected to render them. Each glyph carries a trailing space as a
	// separator from the count that follows it. Terminals without Nerd
	// Font support fall back to the letters E, W, I, and H through
	// nerdfont.Glyph at the call sites, which supply their own spacing.
	LSPErrorIcon   string = "\U000F0159 " // nf-md-close_circle
	LSPWarningIcon string = "\U000F0028 " // nf-md-alert
	LSPInfoIcon    string = "\U000F02FC " // nf-md-information
	LSPHintIcon    string = "\U000F0335 " // nf-md-bug_outline
)

const (
	defaultMargin     = 2
	defaultListIndent = 2
)

type Styles struct {
	Tool struct {
		IconPending             lipgloss.Style
		IconSuccess             lipgloss.Style
		IconError               lipgloss.Style
		IconCancelled           lipgloss.Style
		NameNormal              lipgloss.Style
		NameNested              lipgloss.Style
		ParamMain               lipgloss.Style
		ParamKey                lipgloss.Style
		ContentLine             lipgloss.Style
		ContentTruncation       lipgloss.Style
		ContentCodeLine         lipgloss.Style
		ContentCodeTruncation   lipgloss.Style
		ContentCodeBg           color.Color
		Body                    lipgloss.Style
		ContentBg               lipgloss.Style
		ContentText             lipgloss.Style
		ContentLineNumber       lipgloss.Style
		StateWaiting            lipgloss.Style
		StateCancelled          lipgloss.Style
		ErrorTag                lipgloss.Style
		ErrorMessage            lipgloss.Style
		WarnTag                 lipgloss.Style
		WarnMessage             lipgloss.Style
		DiffTruncation          lipgloss.Style
		NoteTag                 lipgloss.Style
		NoteMessage             lipgloss.Style
		JobIconPending          lipgloss.Style
		JobIconError            lipgloss.Style
		JobIconSuccess          lipgloss.Style
		JobToolName             lipgloss.Style
		JobAction               lipgloss.Style
		JobPID                  lipgloss.Style
		JobDescription          lipgloss.Style
		AgentTaskTag            lipgloss.Style
		AgentPrompt             lipgloss.Style
		AgenticFetchPromptTag   lipgloss.Style
		TodoRatio               lipgloss.Style
		TodoCompletedIcon       lipgloss.Style
		TodoInProgressIcon      lipgloss.Style
		TodoPendingIcon         lipgloss.Style
		TodoStatusNote          lipgloss.Style
		TodoItem                lipgloss.Style
		TodoJustStarted         lipgloss.Style
		CronRecurringIcon       lipgloss.Style
		CronOneShotIcon         lipgloss.Style
		CronDeletedIcon         lipgloss.Style
		CronTaskID              lipgloss.Style
		CronSchedule            lipgloss.Style
		CronNextRun             lipgloss.Style
		CronPrompt              lipgloss.Style
		CronMeta                lipgloss.Style
		CronError               lipgloss.Style
		MCPName                 lipgloss.Style
		MCPToolName             lipgloss.Style
		MCPArrow                lipgloss.Style
		ResourceLoadedText      lipgloss.Style
		ResourceLoadedIndicator lipgloss.Style
		ResourceName            lipgloss.Style
		ResourceSize            lipgloss.Style
		MediaType               lipgloss.Style
		HookLabel               lipgloss.Style
		HookName                lipgloss.Style
		HookMatcher             lipgloss.Style
		HookArrow               lipgloss.Style
		HookDetail              lipgloss.Style
		HookOK                  lipgloss.Style
		HookDenied              lipgloss.Style
		HookDeniedLabel         lipgloss.Style
		HookDeniedReason        lipgloss.Style
		HookRewrote             lipgloss.Style
		ActionCreate            lipgloss.Style
		ActionDestroy           lipgloss.Style
		ResultEmpty             lipgloss.Style
		ResultTruncation        lipgloss.Style
		ResultItemName          lipgloss.Style
		ResultItemDesc          lipgloss.Style
	}
	Dialog struct {
		Title              lipgloss.Style
		TitleText          lipgloss.Style
		TitleError         lipgloss.Style
		TitleAccent        lipgloss.Style
		TitleLineBase      lipgloss.Style
		TitleGradFromColor color.Color
		TitleGradToColor   color.Color
		View               lipgloss.Style
		PrimaryText        lipgloss.Style
		SecondaryText      lipgloss.Style
		HelpView           lipgloss.Style
		Help               struct {
			Ellipsis       lipgloss.Style
			ShortKey       lipgloss.Style
			ShortDesc      lipgloss.Style
			ShortSeparator lipgloss.Style
			FullKey        lipgloss.Style
			FullDesc       lipgloss.Style
			FullSeparator  lipgloss.Style
		}
		NormalItem     lipgloss.Style
		SelectedItem   lipgloss.Style
		InputPrompt    lipgloss.Style
		List           lipgloss.Style
		Spinner        lipgloss.Style
		ContentPanel   lipgloss.Style
		ContentPanelBg color.Color
		ScrollbarThumb lipgloss.Style
		ScrollbarTrack lipgloss.Style
		Arguments      struct {
			Content                  lipgloss.Style
			Description              lipgloss.Style
			InputLabelBlurred        lipgloss.Style
			InputLabelFocused        lipgloss.Style
			InputRequiredMarkBlurred lipgloss.Style
			InputRequiredMarkFocused lipgloss.Style
		}
		ListItem struct {
			InfoBlurred lipgloss.Style
			InfoFocused lipgloss.Style
		}
		Models struct {
			ConfiguredText lipgloss.Style
		}
		Permissions struct {
			KeyText   lipgloss.Style
			ValueText lipgloss.Style
			ParamsBg  color.Color
		}
		Quit struct {
			Content lipgloss.Style
			Hint    lipgloss.Style
			Frame   lipgloss.Style
		}
		APIKey struct {
			Spinner lipgloss.Style
		}
		AuthMethod struct {
			Prompt      lipgloss.Style
			CardBlurred lipgloss.Style
			CardFocused lipgloss.Style
		}
		OAuth struct {
			Spinner      lipgloss.Style
			Instructions lipgloss.Style
			UserCode     lipgloss.Style
			Success      lipgloss.Style
			Link         lipgloss.Style
			Enter        lipgloss.Style
			ErrorText    lipgloss.Style
			StatusText   lipgloss.Style
			UserCodeBg   color.Color
		}
		ImagePreview lipgloss.Style
		Sessions     struct {
			DeletingView                   lipgloss.Style
			DeletingItemFocused            lipgloss.Style
			DeletingItemBlurred            lipgloss.Style
			DeletingTitle                  lipgloss.Style
			DeletingMessage                lipgloss.Style
			DeletingTitleGradientFromColor color.Color
			DeletingTitleGradientToColor   color.Color
			RenamingView                   lipgloss.Style
			RenamingingItemFocused         lipgloss.Style
			RenamingItemBlurred            lipgloss.Style
			RenamingingTitle               lipgloss.Style
			RenamingingMessage             lipgloss.Style
			RenamingTitleGradientFromColor color.Color
			RenamingTitleGradientToColor   color.Color
			RenamingPlaceholder            lipgloss.Style
			InfoBlurred                    lipgloss.Style
			InfoFocused                    lipgloss.Style
		}
	}
	Editor struct {
		PromptBangIconBlurred     lipgloss.Style
		PromptBangDotsBlurred     lipgloss.Style
		PromptNormalIconBlurred   lipgloss.Style
		PromptNormalFocused       lipgloss.Style
		PromptNormalBlurred       lipgloss.Style
		PromptPlanIconFocused     lipgloss.Style
		PromptPlanIconBlurred     lipgloss.Style
		PromptPlanDotsFocused     lipgloss.Style
		PromptPlanDotsBlurred     lipgloss.Style
		PromptYoloIconFocused     lipgloss.Style
		PromptYoloIconBlurred     lipgloss.Style
		PromptYoloDotsFocused     lipgloss.Style
		PromptYoloDotsBlurred     lipgloss.Style
		PromptAutoIconFocused     lipgloss.Style
		PromptAutoIconBlurred     lipgloss.Style
		PromptAutoDotsFocused     lipgloss.Style
		PromptAutoDotsBlurred     lipgloss.Style
		PromptBangIconFocused     lipgloss.Style
		PromptNormalIconFocused   lipgloss.Style
		TokenSkill                lipgloss.Style
		PromptQuestionIconBlurred lipgloss.Style
		PromptQuestionIconFocused lipgloss.Style
		PromptBangDotsFocused     lipgloss.Style
		QuestionSelected          lipgloss.Style
		QuestionUnselected        lipgloss.Style
		QuestionBody              lipgloss.Style
		QuestionConfirm           lipgloss.Style
		QuestionNote              lipgloss.Style
		QuestionCursorBar         lipgloss.Style
		QuestionRadioOn           lipgloss.Style
		QuestionRadioOff          lipgloss.Style
		QuestionCheckOn           lipgloss.Style
		QuestionCheckOff          lipgloss.Style
		TokenFile                 lipgloss.Style
		Textarea                  textarea.Styles
	}
	Messages struct {
		UserBlurred            lipgloss.Style
		UserFocused            lipgloss.Style
		AssistantBlurred       lipgloss.Style
		AssistantFocused       lipgloss.Style
		NoContent              lipgloss.Style
		Thinking               lipgloss.Style
		ErrorTag               lipgloss.Style
		ErrorTitle             lipgloss.Style
		ErrorDetails           lipgloss.Style
		ToolCallFocused        lipgloss.Style
		ToolCallCompact        lipgloss.Style
		ToolCallBlurred        lipgloss.Style
		ShellBarFocused        lipgloss.Style
		ShellBarBlurred        lipgloss.Style
		ShellPrompt            lipgloss.Style
		ShellPromptBlurred     lipgloss.Style
		ShellCommand           lipgloss.Style
		ShellOutput            lipgloss.Style
		ShellExitCode          lipgloss.Style
		ShellTruncation        lipgloss.Style
		SectionHeader          lipgloss.Style
		PlanBox                lipgloss.Style
		CompactionBox          lipgloss.Style
		CompactionHeader       lipgloss.Style
		CompactionNote         lipgloss.Style
		ThinkingBox            lipgloss.Style
		ThinkingTruncationHint lipgloss.Style
		ThinkingFooterTitle    lipgloss.Style
		ThinkingFooterDuration lipgloss.Style
		AssistantInfoIcon      lipgloss.Style
		AssistantInfoModel     lipgloss.Style
		AssistantInfoProvider  lipgloss.Style
		AssistantInfoDuration  lipgloss.Style
		SubduedHypercreditIcon lipgloss.Style
		AssistantCanceled      lipgloss.Style
		AssistantCopyIcon      lipgloss.Style
		ChannelInfoIcon        lipgloss.Style
		ChannelInfoSender      lipgloss.Style
		ChannelInfoProvider    lipgloss.Style
		ChannelInfoTimestamp   lipgloss.Style
	}
	Diff   diffview.Style
	Status struct {
		Help                lipgloss.Style
		ModeBadgePlan       lipgloss.Style
		ModeBadgeYolo       lipgloss.Style
		ModeBadgeAuto       lipgloss.Style
		ModeBannerPlan      lipgloss.Style
		ModeBannerPlanBadge lipgloss.Style
		ModeBannerYolo      lipgloss.Style
		ModeBannerYoloBadge lipgloss.Style
		ModeBannerAuto      lipgloss.Style
		ModeBannerAutoBadge lipgloss.Style
		ErrorIndicator      lipgloss.Style
		WarnIndicator       lipgloss.Style
		InfoIndicator       lipgloss.Style
		UpdateIndicator     lipgloss.Style
		SuccessIndicator    lipgloss.Style
		ErrorMessage        lipgloss.Style
		WarnMessage         lipgloss.Style
		InfoMessage         lipgloss.Style
		UpdateMessage       lipgloss.Style
		SuccessMessage      lipgloss.Style
	}
	Pills struct {
		Base               lipgloss.Style
		Focused            lipgloss.Style
		QueueItemPrefix    lipgloss.Style
		CronItemPrefix     lipgloss.Style
		QueueItemText      lipgloss.Style
		QueueLabel         lipgloss.Style
		QueueIconBase      lipgloss.Style
		QueueGradFromColor color.Color
		QueueGradToColor   color.Color
		TodoLabel          lipgloss.Style
		TodoProgress       lipgloss.Style
		TodoCurrentTask    lipgloss.Style
		TodoSpinner        lipgloss.Style
		HelpKey            lipgloss.Style
		HelpText           lipgloss.Style
		Area               lipgloss.Style
	}
	Resource struct {
		Heading         lipgloss.Style
		Name            lipgloss.Style
		StatusText      lipgloss.Style
		OfflineIcon     lipgloss.Style
		DisabledIcon    lipgloss.Style
		BusyIcon        lipgloss.Style
		ErrorIcon       lipgloss.Style
		OnlineIcon      lipgloss.Style
		OnlineText      lipgloss.Style
		NeedsAuthIcon   lipgloss.Style
		AdditionalText  lipgloss.Style
		CapabilityCount lipgloss.Style
		RowTitleBase    lipgloss.Style
		RowDescBase     lipgloss.Style
		DefaultTitleFg  color.Color
		DefaultDescFg   color.Color
	}
	Header struct {
		Charm             lipgloss.Style
		Diagonals         lipgloss.Style
		Percentage        lipgloss.Style
		HypercreditIcon   lipgloss.Style
		Keystroke         lipgloss.Style
		KeystrokeTip      lipgloss.Style
		WorkingDir        lipgloss.Style
		GitBranch         lipgloss.Style
		Separator         lipgloss.Style
		Wrapper           lipgloss.Style
		LogoGradCanvas    lipgloss.Style
		LogoGradFromColor color.Color
		LogoGradToColor   color.Color
	}
	FilePicker filepicker.Styles
	ModelInfo  struct {
		Icon                 lipgloss.Style
		Name                 lipgloss.Style
		Provider             lipgloss.Style
		ProviderFallback     lipgloss.Style
		Reasoning            lipgloss.Style
		TokenCount           lipgloss.Style
		TokenPercentage      lipgloss.Style
		EstimatedUsagePrefix lipgloss.Style
		Cost                 lipgloss.Style
		HypercreditIcon      lipgloss.Style
		HypercreditText      lipgloss.Style
	}
	QuietMarkdown ansi.StyleConfig
	PlanMarkdown  ansi.StyleConfig
	Markdown      ansi.StyleConfig
	Attachments   AttachmentStyles
	Help          help.Styles
	Files         struct {
		Path           lipgloss.Style
		Additions      lipgloss.Style
		Deletions      lipgloss.Style
		SectionTitle   lipgloss.Style
		EmptyMessage   lipgloss.Style
		TruncationHint lipgloss.Style
	}
	Button struct {
		Focused  lipgloss.Style
		Blurred  lipgloss.Style
		Inactive lipgloss.Style
		Hovered  lipgloss.Style
		Negative lipgloss.Style
	}
	LSP struct {
		ErrorDiagnostic   lipgloss.Style
		WarningDiagnostic lipgloss.Style
		HintDiagnostic    lipgloss.Style
		InfoDiagnostic    lipgloss.Style
	}
	Completions struct {
		Normal  lipgloss.Style
		Focused lipgloss.Style
		Match   lipgloss.Style
		Agent   lipgloss.Style
	}
	Logo struct {
		FieldColor         color.Color
		TitleColorA        color.Color
		TitleColorB        color.Color
		CharmColor         color.Color
		VersionColor       color.Color
		SmallCharm         lipgloss.Style
		SmallDiagonals     lipgloss.Style
		GradCanvas         lipgloss.Style
		SmallGradFromColor color.Color
		SmallGradToColor   color.Color
	}
	Sidebar struct {
		SessionTitle lipgloss.Style
		WorkingDir   lipgloss.Style
		AgentInspect lipgloss.Style
	}
	CompactDetails struct {
		View    lipgloss.Style
		Version lipgloss.Style
		Title   lipgloss.Style
	}
	Initialize struct {
		Header  lipgloss.Style
		Content lipgloss.Style
		Accent  lipgloss.Style
	}
	Radio struct {
		On    lipgloss.Style
		Off   lipgloss.Style
		Label lipgloss.Style
	}
	Section struct {
		Title lipgloss.Style
		Line  lipgloss.Style
	}
	ToolCallSuccess      lipgloss.Style
	TextSelection        lipgloss.Style
	ANSI                 [16]color.Color
	Background           color.Color
	WorkingGradFromColor color.Color
	WorkingGradToColor   color.Color
	WorkingTimerColor    color.Color
	WorkingLabelColor    color.Color
	Tab                  struct {
		ActiveBorder          uv.Border
		InactiveBorder        uv.Border
		ActiveBorderBlurred   uv.Border
		InactiveBorderBlurred uv.Border
		ActiveStyle           uv.Style
		InactiveStyle         uv.Style
	}
	TextInput textinput.Styles // Icon styles with tool status
	// Text next to a radio button
}

// ChromaTheme converts the current markdown chroma styles to a chroma
// StyleEntries map.
func (s *Styles) ChromaTheme() chroma.StyleEntries {
	rules := s.Markdown.CodeBlock

	return chroma.StyleEntries{
		chroma.Text:                chromaStyle(rules.Chroma.Text),
		chroma.Error:               chromaStyle(rules.Chroma.Error),
		chroma.Comment:             chromaStyle(rules.Chroma.Comment),
		chroma.CommentPreproc:      chromaStyle(rules.Chroma.CommentPreproc),
		chroma.Keyword:             chromaStyle(rules.Chroma.Keyword),
		chroma.KeywordReserved:     chromaStyle(rules.Chroma.KeywordReserved),
		chroma.KeywordNamespace:    chromaStyle(rules.Chroma.KeywordNamespace),
		chroma.KeywordType:         chromaStyle(rules.Chroma.KeywordType),
		chroma.Operator:            chromaStyle(rules.Chroma.Operator),
		chroma.Punctuation:         chromaStyle(rules.Chroma.Punctuation),
		chroma.Name:                chromaStyle(rules.Chroma.Name),
		chroma.NameBuiltin:         chromaStyle(rules.Chroma.NameBuiltin),
		chroma.NameTag:             chromaStyle(rules.Chroma.NameTag),
		chroma.NameAttribute:       chromaStyle(rules.Chroma.NameAttribute),
		chroma.NameClass:           chromaStyle(rules.Chroma.NameClass),
		chroma.NameConstant:        chromaStyle(rules.Chroma.NameConstant),
		chroma.NameDecorator:       chromaStyle(rules.Chroma.NameDecorator),
		chroma.NameException:       chromaStyle(rules.Chroma.NameException),
		chroma.NameFunction:        chromaStyle(rules.Chroma.NameFunction),
		chroma.NameOther:           chromaStyle(rules.Chroma.NameOther),
		chroma.Literal:             chromaStyle(rules.Chroma.Literal),
		chroma.LiteralNumber:       chromaStyle(rules.Chroma.LiteralNumber),
		chroma.LiteralDate:         chromaStyle(rules.Chroma.LiteralDate),
		chroma.LiteralString:       chromaStyle(rules.Chroma.LiteralString),
		chroma.LiteralStringEscape: chromaStyle(rules.Chroma.LiteralStringEscape),
		chroma.GenericDeleted:      chromaStyle(rules.Chroma.GenericDeleted),
		chroma.GenericEmph:         chromaStyle(rules.Chroma.GenericEmph),
		chroma.GenericInserted:     chromaStyle(rules.Chroma.GenericInserted),
		chroma.GenericStrong:       chromaStyle(rules.Chroma.GenericStrong),
		chroma.GenericSubheading:   chromaStyle(rules.Chroma.GenericSubheading),
		chroma.Background:          chromaStyle(rules.Chroma.Background),
	}
}

// AttachmentStyles are the styles for one attachment chip row. They are
// passed to the attachments renderer as a unit so adding a style does not
// mean touching every construction site.
type AttachmentStyles struct {
	Normal   lipgloss.Style
	Image    lipgloss.Style
	Text     lipgloss.Style
	Skill    lipgloss.Style
	Prompt   lipgloss.Style
	Remove   lipgloss.Style
	Deleting lipgloss.Style
	More     lipgloss.Style // "N more…" hint for chips that didn't fit
}

// DialogHelpStyles returns the styles for dialog help.
func (s *Styles) DialogHelpStyles() help.Styles {
	return help.Styles(s.Dialog.Help)
}

// Clone returns a deep copy of the Styles struct, ensuring pointer fields
// (particularly within ansi.StyleConfig) are not aliased. This is used to
// safely snapshot styles before a theme preview.
func (s *Styles) Clone() Styles {
	clone := *s
	clone.Markdown = cloneStyleConfig(s.Markdown)
	clone.QuietMarkdown = cloneStyleConfig(s.QuietMarkdown)
	return clone
}

// cloneStyleConfig deep-copies an ansi.StyleConfig by JSON round-tripping.
//
// NOTE: This assumes ansi.StyleConfig remains fully JSON-round-trippable.
// If the glamour library adds non-serializable fields, this approach will
// need to be replaced with a reflect-based deep copy.
func cloneStyleConfig(src ansi.StyleConfig) ansi.StyleConfig {
	data, err := sonic.Marshal(src)
	if err != nil {
		slog.Error("Failed to marshal markdown styles for theme preview", "error", err)
		return src
	}
	var dst ansi.StyleConfig
	if err := sonic.Unmarshal(data, &dst); err != nil {
		slog.Error("Failed to unmarshal markdown styles for theme preview", "error", err)
		return src
	}
	return dst
}

// hex returns a pointer to the "#rrggbb" representation of c. It's used to
// satisfy glamour's string-pointer API when configuring markdown colors
// from the theme palette.
func hex(c color.Color) *string {
	r, g, b, _ := c.RGBA()
	s := fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
	return &s
}

func chromaStyle(style ansi.StylePrimitive) string {
	var s strings.Builder

	if style.Color != nil {
		s.WriteString(*style.Color)
	}
	if style.BackgroundColor != nil {
		if s.Len() > 0 {
			s.WriteString(" ")
		}
		s.WriteString("bg:")
		s.WriteString(*style.BackgroundColor)
	}
	if style.Italic != nil && *style.Italic {
		if s.Len() > 0 {
			s.WriteString(" ")
		}
		s.WriteString("italic")
	}
	if style.Bold != nil && *style.Bold {
		if s.Len() > 0 {
			s.WriteString(" ")
		}
		s.WriteString("bold")
	}
	if style.Underline != nil && *style.Underline {
		if s.Len() > 0 {
			s.WriteString(" ")
		}
		s.WriteString("underline")
	}

	return s.String()
}
