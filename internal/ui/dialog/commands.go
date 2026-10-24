package dialog

import (
	"image"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/honcho"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
)

// CommandsID is the identifier for the commands dialog.
const CommandsID = "commands"

// menuLevel captures the parent state when navigating into a sub-menu.
type menuLevel struct {
	items []list.Item
}

// CommandType represents the type of commands being displayed.
type CommandType uint

// String returns the string representation of the CommandType.
func (c CommandType) String() string { return []string{"System", "User", "MCP"}[c] }

const (
	sidebarCompactModeBreakpoint = 120
	// commandsSpareRows is the breathing room kept below the commands so the
	// list does not end flush against the help line.
	commandsSpareRows = 2
	// commandsMaxHeight caps the palette at the standard dialog height plus
	// its spare rows; longer command lists scroll.
	commandsMaxHeight = defaultDialogHeight + commandsSpareRows
)

const (
	SystemCommands CommandType = iota
	UserCommands
	MCPPrompts
)

// Commands represents a dialog that shows available commands.
type dockerMCPAvailabilityCheckedMsg struct {
	available bool
}

type Commands struct {
	dockerMCPAvailable *bool
	com                *common.Common
	list               *CommandsList
	help               help.Model
	sessionID          string
	keyMap             struct{ Select, UpDown, Next, Previous, Tab, ShiftTab, Back, Close key.Binding }
	mcpPrompts         []commands.MCPPrompt
	customCommands     []commands.CustomCommand
	breadcrumb         []string
	menuStack          []menuLevel
	input              textinput.Model
	spinner            spinner.Model
	selected           CommandType
	windowWidth        int

	// bodyArea is the screen rectangle of the command list, tracked during
	// Draw so mouse events can be mapped to list items.
	bodyArea      image.Rectangle
	lastClickTime time.Time
	lastClickID   string

	// Items for every tab, kept so the dialog can measure the tallest one
	// instead of only the tab on screen.
	systemGroups []CommandGroup
	userItems    []*CommandItem
	mcpItems     []*CommandItem

	// tabHeights memoizes tallestTabHeight per width. Measuring renders
	// every item on every tab, so it is only recomputed when the command
	// items are rebuilt, which happens in setCommandItems.
	tabHeights   map[int]int
	tabHeightsOK bool

	hasCron                bool
	hasTodos               bool
	hasSession             bool
	dockerMCPCheckInFlight bool
	hasQueue               bool
	loading                bool
}

var _ Dialog = (*Commands)(nil)

// NewCommands creates a new commands dialog.
func NewCommands(com *common.Common, sessionID string, hasSession, hasTodos, hasQueue, hasCron bool, customCommands []commands.CustomCommand, mcpPrompts []commands.MCPPrompt) (*Commands, error) {
	c := &Commands{
		com:            com,
		selected:       SystemCommands,
		sessionID:      sessionID,
		hasSession:     hasSession,
		hasTodos:       hasTodos,
		hasQueue:       hasQueue,
		hasCron:        hasCron,
		customCommands: customCommands,
		mcpPrompts:     mcpPrompts,
	}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()

	c.help = help

	c.list = NewCommandsList(com.Styles)
	c.list.Focus()
	c.list.SetSelected(0)

	c.input = textinput.New()
	c.input.SetVirtualCursor(false)
	c.input.Placeholder = "Type to filter"
	c.input.SetStyles(com.Styles.TextInput)
	c.input.Focus()

	c.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	c.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	c.keyMap.Next = key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "next item"),
	)
	c.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	c.keyMap.Tab = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch selection"),
	)
	c.keyMap.ShiftTab = key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "switch selection prev"),
	)
	c.keyMap.Back = key.NewBinding(
		key.WithKeys("backspace", "alt+left"),
		key.WithHelp("backspace", "back"),
	)
	closeKey := CloseKey
	closeKey.SetHelp("esc", "cancel")
	c.keyMap.Close = closeKey

	if available, known := config.DockerMCPAvailabilityCached(); known {
		c.dockerMCPAvailable = &available
	}

	// Set initial commands
	c.setCommandItems(c.selected)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = com.Styles.Dialog.Spinner
	c.spinner = s

	return c, nil
}

// ID implements Dialog.
func (c *Commands) ID() string {
	return CommandsID
}

// HandleMsg implements [Dialog].
func (c *Commands) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case dockerMCPAvailabilityCheckedMsg:
		c.dockerMCPAvailable = &msg.available
		c.dockerMCPCheckInFlight = false
		if c.selected == SystemCommands {
			// Preserve the current selection across the rebuild to avoid reset
			var prevID string
			if item, ok := c.list.SelectedItem().(*CommandItem); ok && item != nil {
				prevID = item.id
			}
			c.setCommandItems(c.selected)
			if prevID != "" {
				for i, it := range c.list.VisibleItems() {
					if ci, ok := it.(*CommandItem); ok && ci != nil && ci.id == prevID {
						c.list.SetSelected(i)
						c.list.ScrollToSelected()
						break
					}
				}
			}
		}
		return nil
	case spinner.TickMsg:
		if c.loading {
			var cmd tea.Cmd
			c.spinner, cmd = c.spinner.Update(msg)
			return ActionCmd{Cmd: cmd}
		}
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, c.keyMap.Close):
			if c.inSubMenu() {
				c.popMenu()
				return nil
			}
			return ActionClose{}
		case key.Matches(msg, c.keyMap.Back) && c.inSubMenu():
			// Backspace pops out of a sub-menu, but only while in one. At the
			// top level it must fall through to the input so it can edit the
			// type-ahead filter (otherwise the case swallows it and backspace
			// does nothing).
			c.popMenu()
			return nil
		case key.Matches(msg, c.keyMap.Previous):
			c.list.Focus()
			if c.list.IsSelectedFirst() {
				c.list.SelectLast()
			} else {
				c.list.SelectPrev()
			}
			c.list.ScrollToSelected()
		case key.Matches(msg, c.keyMap.Next):
			c.list.Focus()
			if c.list.IsSelectedLast() {
				c.list.SelectFirst()
			} else {
				c.list.SelectNext()
			}
			c.list.ScrollToSelected()
		case key.Matches(msg, c.keyMap.Select):
			if selectedItem := c.list.SelectedItem(); selectedItem != nil {
				if item, ok := selectedItem.(*CommandItem); ok && item != nil {
					if item.HasChildren() {
						c.pushMenu(item)
						return nil
					}
					return item.Action()
				}
			}
		case key.Matches(msg, c.keyMap.Tab):
			if !c.inSubMenu() && (len(c.customCommands) > 0 || len(c.mcpPrompts) > 0) {
				c.selected = c.nextCommandType()
				c.setCommandItems(c.selected)
			}
		case key.Matches(msg, c.keyMap.ShiftTab):
			if !c.inSubMenu() && (len(c.customCommands) > 0 || len(c.mcpPrompts) > 0) {
				c.selected = c.previousCommandType()
				c.setCommandItems(c.selected)
			}
		default:
			// Typing '/' while the filter is empty closes the dialog
			// so the user can dismiss it without reaching for Escape.
			if msg.String() == "/" && c.input.Value() == "" {
				return ActionClose{}
			}
			var cmd tea.Cmd
			for _, item := range commandItems(c.list.VisibleItems()) {
				if msg.String() == item.Shortcut() {
					if item.HasChildren() {
						c.pushMenu(item)
						return nil
					}
					return item.Action()
				}
			}
			prevValue := c.input.Value()
			c.input, cmd = c.input.Update(msg)
			value := c.input.Value()
			if value != prevValue {
				c.list.SetFilter(value)
				c.list.ScrollToTop()
				c.list.SetSelected(0)
			}
			return ActionCmd{cmd}
		}
	case common.CoalescedWheelMsg:
		if image.Pt(msg.Mouse.X, msg.Mouse.Y).In(c.bodyArea) {
			c.list.ScrollBy(int(msg.DeltaY))
		}
	case tea.MouseClickMsg:
		return c.handleMouseClick(msg)
	}
	return nil
}

// handleMouseClick selects the command under a left click, or runs it when
// the same command is clicked twice in quick succession. Clicks that miss the
// list, land on a section header or spacer, or use another button are
// ignored so the dialog only reacts to commands the user can act on.
func (c *Commands) handleMouseClick(msg tea.MouseClickMsg) Action {
	if msg.Button != tea.MouseLeft {
		c.resetMouseClick()
		return nil
	}
	area := c.bodyArea
	area.Max.X = min(area.Max.X, area.Min.X+c.list.Width())
	point := image.Pt(msg.X, msg.Y)
	if !point.In(area) {
		c.resetMouseClick()
		return nil
	}
	index, _ := c.list.ItemIndexAtPosition(point.X-area.Min.X, point.Y-area.Min.Y)
	if index < 0 {
		c.resetMouseClick()
		return nil
	}
	item, ok := c.list.ItemAt(index).(*CommandItem)
	if !ok || item == nil {
		c.resetMouseClick()
		return nil
	}
	now := time.Now()
	if c.lastClickID == item.ID() && now.Sub(c.lastClickTime) <= dialogDoubleClickThreshold {
		c.resetMouseClick()
		if item.HasChildren() {
			c.pushMenu(item)
			return nil
		}
		return item.Action()
	}
	c.lastClickTime = now
	c.lastClickID = item.ID()
	c.list.SetSelected(index)
	return nil
}

// resetMouseClick forgets the pending click so the next one starts a fresh
// double-click window.
func (c *Commands) resetMouseClick() {
	c.lastClickTime = time.Time{}
	c.lastClickID = ""
}

func checkDockerMCPAvailabilityCmd() tea.Cmd {
	return func() tea.Msg {
		return dockerMCPAvailabilityCheckedMsg{available: config.RefreshDockerMCPAvailability()}
	}
}

func (c *Commands) InitialCmd() tea.Cmd {
	if c.dockerMCPAvailable != nil || c.dockerMCPCheckInFlight {
		return nil
	}
	c.dockerMCPCheckInFlight = true
	return checkDockerMCPAvailabilityCmd()
}

// Cursor returns the cursor position relative to the dialog.
func (c *Commands) Cursor() *tea.Cursor {
	return InputCursor(c.com.Styles, c.input.Cursor())
}

// commandsRadioView generates the command type selector radio buttons.
func commandsRadioView(sty *styles.Styles, selected CommandType, hasUserCmds bool, hasMCPPrompts bool) string {
	if !hasUserCmds && !hasMCPPrompts {
		return ""
	}

	selectedFn := func(t CommandType) string {
		if t == selected {
			return sty.Radio.On.Padding(0, 1).Render() + sty.Radio.Label.Render(t.String())
		}
		return sty.Radio.Off.Padding(0, 1).Render() + sty.Radio.Label.Render(t.String())
	}

	parts := []string{
		selectedFn(SystemCommands),
	}

	if hasUserCmds {
		parts = append(parts, selectedFn(UserCommands))
	}
	if hasMCPPrompts {
		parts = append(parts, selectedFn(MCPPrompts))
	}

	return strings.Join(parts, " ")
}

// Draw implements [Dialog].
func (c *Commands) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := c.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	if area.Dx() != c.windowWidth && c.selected == SystemCommands {
		c.windowWidth = area.Dx()
		// since some items in the list depend on width (e.g. toggle sidebar command),
		// we need to reset the command items when width changes. Repopulating
		// replaces the list with top-level items, so any open sub-menu must be
		// abandoned too — resetting to the top level is simpler than re-deriving
		// the open sub-menu's children against the regenerated item set, and
		// keeps the list, menu stack, and breadcrumb consistent.
		c.menuStack = nil
		c.breadcrumb = nil
		c.setCommandItems(c.selected)
	}

	innerWidth := width - c.com.Styles.Dialog.View.GetHorizontalFrameSize()

	c.input.SetWidth(dialogInputTextWidth(t, c.input, innerWidth))

	// Size to the tallest tab plus a couple of spare rows so the palette
	// keeps one height as tabs and filters change, bounded by the dialog
	// maximum and by what the screen has room for.
	contentHeight := c.tallestTabHeight(innerWidth)
	height := min(
		fitDialogHeight(t, contentHeight, commandsSpareRows, area.Dy()),
		commandsMaxHeight,
	)

	// The list is laid out one column narrower than innerWidth when its
	// content overflows and a scrollbar shows, and items can wrap taller at
	// the narrower width. When the tallest tab overflows the viewport it
	// will get, remeasure at the width the list will actually be laid out
	// at instead.
	if innerWidth > 0 && contentHeight > height-dialogChromeHeight(t) {
		contentHeight = c.tallestTabHeight(innerWidth - 1)
		height = min(
			fitDialogHeight(t, contentHeight, commandsSpareRows, area.Dy()),
			commandsMaxHeight,
		)
	}

	listHeight, listTotalHeight, listWidth := sizeDialogList(t, c.list, innerWidth, height)

	// Hide the shortcut hints uniformly when the widest would crowd names.
	applyInfoColumnVisibility(c.list.VisibleItems(), listWidth, commandInfoMaxPercent)

	rc := NewRenderContext(t, width)
	rc.Title = "Commands"
	if c.inSubMenu() {
		rc.Title = "Commands ▸ " + strings.Join(c.breadcrumb, " ▸ ")
	} else {
		rc.TitleInfo = commandsRadioView(t, c.selected, len(c.customCommands) > 0, len(c.mcpPrompts) > 0)
	}
	inputView := t.Dialog.InputPrompt.Render(c.input.View())
	rc.AddPart(inputView)
	listView := t.Dialog.List.Height(c.list.Height()).Render(c.list.Render())
	listView = joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, c.list.Offset())
	rc.AddPart(listView)
	rc.Help = renderDialogHelp(t, &c.help, c, innerWidth)

	if c.loading {
		rc.Help = t.Dialog.HelpView.Width(innerWidth).Render(c.spinner.View() + " Generating Prompt...")
	}

	view := rc.Render()
	c.bodyArea = dialogListArea(area, view, listView, rc.Help, rc.ViewStyle, t.Dialog.List, innerWidth, listHeight)

	cur := c.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements [help.KeyMap].
func (c *Commands) ShortHelp() []key.Binding {
	binds := []key.Binding{
		c.keyMap.Tab,
		c.keyMap.UpDown,
		c.keyMap.Select,
	}
	if c.inSubMenu() {
		binds = append(binds, c.keyMap.Back)
	}
	return append(binds, c.keyMap.Close)
}

// FullHelp implements [help.KeyMap].
func (c *Commands) FullHelp() [][]key.Binding {
	second := []key.Binding{c.keyMap.Close}
	if c.inSubMenu() {
		second = append([]key.Binding{c.keyMap.Back}, second...)
	}
	return [][]key.Binding{
		{c.keyMap.Select, c.keyMap.Next, c.keyMap.Previous, c.keyMap.Tab},
		second,
	}
}

// nextCommandType returns the next command type in the cycle.
func (c *Commands) nextCommandType() CommandType {
	switch c.selected {
	case SystemCommands:
		if len(c.customCommands) > 0 {
			return UserCommands
		}
		if len(c.mcpPrompts) > 0 {
			return MCPPrompts
		}
		fallthrough
	case UserCommands:
		if len(c.mcpPrompts) > 0 {
			return MCPPrompts
		}
		fallthrough
	case MCPPrompts:
		return SystemCommands
	default:
		return SystemCommands
	}
}

// previousCommandType returns the previous command type in the cycle.
func (c *Commands) previousCommandType() CommandType {
	switch c.selected {
	case SystemCommands:
		if len(c.mcpPrompts) > 0 {
			return MCPPrompts
		}
		if len(c.customCommands) > 0 {
			return UserCommands
		}
		return SystemCommands
	case UserCommands:
		return SystemCommands
	case MCPPrompts:
		if len(c.customCommands) > 0 {
			return UserCommands
		}
		return SystemCommands
	default:
		return SystemCommands
	}
}

// inSubMenu reports whether the dialog is currently inside a sub-menu.
func (c *Commands) inSubMenu() bool {
	return len(c.menuStack) > 0
}

// pushMenu saves the current list state and navigates into a sub-menu.
func (c *Commands) pushMenu(parent *CommandItem) {
	// Clear any active filter before snapshotting so we save the FULL parent
	// list, not just the currently-matched subset. Otherwise popping back would
	// drop every top-level item that didn't match the filter that was active
	// when the user entered the sub-menu.
	c.input.SetValue("")
	c.list.SetFilter("")

	// Save current visible items so popping back restores the parent list.
	c.menuStack = append(c.menuStack, menuLevel{
		items: c.list.VisibleItems(),
	})
	c.breadcrumb = append(c.breadcrumb, parent.title)

	c.list.SetItems(parent.children...)
	c.list.SetFilter("")
	c.list.ScrollToTop()
	c.list.SetSelected(0)
	c.input.SetValue("")
}

// popMenu restores the previous menu level from the stack.
func (c *Commands) popMenu() {
	if len(c.menuStack) == 0 {
		return
	}
	level := c.menuStack[len(c.menuStack)-1]
	c.menuStack = c.menuStack[:len(c.menuStack)-1]
	c.breadcrumb = c.breadcrumb[:len(c.breadcrumb)-1]

	// Restore items by re-setting them as flat command items.
	c.list.SetItems(commandItems(level.items)...)
	c.list.SetFilter("")
	c.list.ScrollToTop()
	c.list.SetSelected(0)
	c.input.SetValue("")
}

// setCommandItems sets the command items based on the specified command type.
// Every tab's items are built, not just the visible one, so the dialog can
// size itself to the tallest tab and keep that height as tabs change.
func (c *Commands) setCommandItems(commandType CommandType) {
	c.selected = commandType

	c.systemGroups = c.defaultCommandGroups()
	c.userItems = c.customCommandItems()
	c.mcpItems = c.mcpPromptItems()
	// The cached heights describe the previous items.
	c.tabHeightsOK = false

	switch c.selected {
	case SystemCommands:
		c.list.SetGroups(c.systemGroups...)
	case UserCommands:
		c.list.SetItems(c.userItems...)
	case MCPPrompts:
		c.list.SetItems(c.mcpItems...)
	}

	c.list.SetFilter("")
	c.list.ScrollToTop()
	c.list.SetSelected(0)
	c.input.SetValue("")
}

// customCommandItems returns the items for the user commands tab.
func (c *Commands) customCommandItems() []*CommandItem {
	items := []*CommandItem{}
	for _, cmd := range c.customCommands {
		var action Action
		if cmd.Skill != nil {
			action = ActionAttachSkill{ID: cmd.Skill.SkillFilePath, Name: cmd.Skill.Name}
		} else {
			action = ActionRunCustomCommand{
				Content:   cmd.Content,
				Arguments: cmd.Arguments,
				Skill:     cmd.Skill,
			}
		}
		item := NewCommandItem(c.com.Styles, "custom_"+cmd.ID, cmd.Name, "", action)
		if cmd.Skill != nil {
			item = item.WithDescription(cmd.Skill.Description)
		}
		items = append(items, item)
	}
	return items
}

// mcpPromptItems returns the items for the MCP prompts tab.
func (c *Commands) mcpPromptItems() []*CommandItem {
	items := []*CommandItem{}
	for _, cmd := range c.mcpPrompts {
		action := ActionRunMCPPrompt{
			Title:       cmd.Title,
			Description: cmd.Description,
			PromptID:    cmd.PromptID,
			ClientID:    cmd.ClientID,
			Arguments:   cmd.Arguments,
		}
		items = append(items, NewCommandItem(c.com.Styles, "mcp_"+cmd.ID, cmd.PromptID, "", action))
	}
	return items
}

// tallestTabHeight returns the content height of the tab with the most rows
// at the given width, so switching tabs never resizes the dialog. Results
// are memoized per width and only recomputed when the command items are
// rebuilt, so Draw does not re-render every item on every tab every frame.
func (c *Commands) tallestTabHeight(width int) int {
	if !c.tabHeightsOK {
		c.tabHeights = make(map[int]int)
		c.tabHeightsOK = true
	}
	if height, ok := c.tabHeights[width]; ok {
		return height
	}
	height := max(
		groupsContentHeight(width, c.systemGroups),
		itemsContentHeight(width, c.userItems),
		itemsContentHeight(width, c.mcpItems),
	)
	c.tabHeights[width] = height
	return height
}

// defaultCommandGroups returns the default system commands grouped into
// sections. Empty sections are omitted. The fork's extra commands slot into
// the section that matches what they act on.
func (c *Commands) defaultCommandGroups() []CommandGroup {
	t := c.com.Styles

	session := []*CommandItem{
		NewCommandItem(t, "new_session", "New Session", "ctrl+n", ActionNewSession{}).WithAliases("clear"),
		NewCommandItem(t, "switch_session", "Open Session", "ctrl+s", ActionOpenDialog{SessionsID}),
	}

	// Only show these when there's an active session.
	if c.hasSession {
		session = append(session,
			NewCommandItem(t, "summarize", "Summarize Session", "/compact", ActionSummarize{SessionID: c.sessionID}),
			NewCommandItem(t, "fork_session", "Fork Conversation", "", ActionForkSession{SessionID: c.sessionID}).WithAliases("fork"),
			NewCommandItem(t, "goal_mode", "Goal Mode", "/goal", ActionGoalMode{
				SessionID: c.sessionID,
				Arguments: goalArguments(),
			}).WithAliases("goal"),
		)
	}

	// Add file picker for models that support images.
	if c.hasSession {
		cfgPrime := c.com.Config()
		agentCfg := cfgPrime.Agents[config.AgentCoder]
		model := cfgPrime.GetModelByType(agentCfg.Model)
		if model != nil && model.SupportsImages {
			session = append(session, NewCommandItem(t, "file_picker", "Open File Picker", "ctrl+f", ActionOpenDialog{
				DialogID: FilePickerID,
			}))
		}
	}

	session = append(
		session,
		NewCommandItem(t, "init", "Initialize Project", "", ActionInitializeProject{}),
		NewCommandItem(t, "toggle_yolo", "Cycle Permission Mode", "ctrl+y", ActionToggleYoloMode{}),
	)

	model := []*CommandItem{
		NewCommandItem(t, "switch_model", "Switch Model", "ctrl+l", ActionOpenDialog{ModelsID}),
	}

	// Add reasoning toggle for models that support it
	cfg := c.com.Config()
	if agentCfg, ok := cfg.Agents[config.AgentCoder]; ok {
		providerCfg := cfg.GetProviderForModel(agentCfg.Model)
		reasoningModel := cfg.GetModelByType(agentCfg.Model)
		if providerCfg != nil && reasoningModel != nil && reasoningModel.CanReason {
			selectedModel := cfg.Models[agentCfg.Model]

			// Anthropic models: thinking toggle
			if reasoningModel.CanReason && len(reasoningModel.ReasoningLevels) == 0 {
				status := "Enable"
				if selectedModel.Think {
					status = "Disable"
				}
				model = append(model, NewCommandItem(t, "toggle_thinking", status+" Thinking Mode", "", ActionToggleThinking{}))
			}

			// OpenAI models: reasoning effort dialog
			if len(reasoningModel.ReasoningLevels) > 0 {
				model = append(model, NewCommandItem(t, "select_reasoning_effort", "Select Reasoning Effort", "", ActionOpenDialog{
					DialogID: ReasoningID,
				}))
			}
		}
	}

	settings := []*CommandItem{
		NewCommandItem(t, "switch_theme", "Themes", "", ActionOpenDialog{DialogID: ThemeID}),
	}

	// Only show toggle compact mode command if window width is larger than compact breakpoint (120)
	if c.windowWidth >= sidebarCompactModeBreakpoint && c.hasSession {
		settings = append(settings, NewCommandItem(t, "toggle_sidebar", "Toggle Sidebar", "ctrl+b", ActionToggleCompactMode{}))
	}

	// Add Docker MCP command if available and not already enabled.
	if !cfg.IsDockerMCPEnabled() && c.dockerMCPAvailable != nil && *c.dockerMCPAvailable {
		settings = append(settings, NewCommandItem(t, "enable_docker_mcp", "Enable Docker MCP Catalog", "", ActionEnableDockerMCP{}))
	}

	// Add disable Docker MCP command if it's currently enabled
	if cfg.IsDockerMCPEnabled() {
		settings = append(settings, NewCommandItem(t, "disable_docker_mcp", "Disable Docker MCP Catalog", "", ActionDisableDockerMCP{}))
	}

	// Toggle MCP servers for this repository when any are configured.
	if len(cfg.MCP) > 0 {
		settings = append(settings, NewCommandItem(t, "toggle_mcps", "Toggle MCPs", "", ActionOpenDialog{
			DialogID: MCPTogglesID,
		}))
	}

	// ctrl+t expands every section that has content, so the label names all of
	// them rather than just the first.
	if c.hasTodos || c.hasQueue || c.hasCron {
		var sections []string
		if c.hasTodos {
			sections = append(sections, "To-Dos")
		}
		if c.hasQueue {
			sections = append(sections, "Queue")
		}
		if c.hasCron {
			sections = append(sections, "Scheduled")
		}
		label := "Toggle " + strings.Join(sections, "/")
		settings = append(settings, NewCommandItem(t, "toggle_pills", label, "ctrl+t", ActionTogglePills{}))
	}

	// Offer memory sign-in when it is not yet connected, and a way
	// out when it is. Reading the token file here is cheap and keeps
	// the label truthful without watching for changes.
	if honcho.SignedIn() {
		settings = append(settings, NewCommandItem(t, "disconnect_memory", "Disconnect Memory", "", ActionDisconnectMemory{}))
	} else {
		settings = append(settings, NewCommandItem(t, "connect_memory", "Connect Memory", "", ActionOpenDialog{DialogID: OAuthHonchoID}).WithAliases("honcho"))
	}

	// Add a command for selecting notification style via picker dialog.
	notificationLabel := "Notification Style"
	settings = append(settings, NewCommandItem(t, "select_notifications", notificationLabel, "", ActionOpenDialog{DialogID: NotificationsID}))

	// Add transparent background toggle.
	transparentLabel := "Disable Background Color"
	if cfg != nil && cfg.Options != nil && cfg.Options.TUI.IsTransparent() {
		transparentLabel = "Enable Background Color"
	}
	settings = append(settings, NewCommandItem(t, "toggle_transparent", transparentLabel, "", ActionToggleTransparentBackground{}))

	// Add Channels command — opens the channels management dialog.
	settings = append(settings, NewCommandItem(t, "channels", "Channels", "", ActionOpenDialog{DialogID: ChannelsID}))

	// Add Skills management command — opens the repository-scoped skill
	// toggles dialog.
	settings = append(settings, NewCommandItem(t, "skills", "Toggle Skills", "", ActionOpenDialog{DialogID: SkillsTogglesID}))

	// Add mouse support toggle.
	mouseLabel := "Disable Mouse"
	if cfg != nil && cfg.Options != nil && cfg.Options.TUI.Mouse != nil && !*cfg.Options.TUI.Mouse {
		mouseLabel = "Enable Mouse"
	}
	settings = append(settings, NewCommandItem(t, "toggle_mouse", mouseLabel, "", ActionToggleMouseSupport{}))

	application := []*CommandItem{}

	// Add external editor command if $EDITOR is available.
	//
	// TODO: Use [tea.EnvMsg] to get environment variable instead of os.Getenv;
	// because os.Getenv does IO is breaks the TEA paradigm and is generally an
	// antipattern.
	if os.Getenv("EDITOR") != "" {
		application = append(application, NewCommandItem(t, "open_external_editor", "Open External Editor", "ctrl+o", ActionExternalEditor{}))
	}

	application = append(
		application,
		NewCommandItem(t, "toggle_help", "Toggle Help", "ctrl+g", ActionToggleHelp{}),
		NewCommandItem(t, "quit", "Quit", "ctrl+c", tea.QuitMsg{}).WithAliases("exit"),
	)

	groups := []CommandGroup{}
	addGroup := func(title string, items []*CommandItem) {
		if len(items) > 0 {
			groups = append(groups, NewCommandGroup(t, title, items...))
		}
	}
	addGroup("Sessions", session)
	addGroup("Models", model)
	addGroup("Settings", settings)
	addGroup("Application", application)
	return groups
}

// SetCustomCommands sets the custom commands and refreshes the view if user commands are currently displayed.
func (c *Commands) SetCustomCommands(customCommands []commands.CustomCommand) {
	c.customCommands = customCommands
	if c.selected == UserCommands {
		c.setCommandItems(c.selected)
	}
}

// SetMCPPrompts sets the MCP prompts and refreshes the view if MCP prompts are currently displayed.
func (c *Commands) SetMCPPrompts(mcpPrompts []commands.MCPPrompt) {
	c.mcpPrompts = mcpPrompts
	if c.selected == MCPPrompts {
		c.setCommandItems(c.selected)
	}
}

// StartLoading implements [LoadingDialog].
func (c *Commands) StartLoading() tea.Cmd {
	if c.loading {
		return nil
	}
	c.loading = true
	return c.spinner.Tick
}

// StopLoading implements [LoadingDialog].
func (c *Commands) StopLoading() {
	c.loading = false
}

func compactArguments() []commands.Argument {
	return []commands.Argument{{
		ID:          CompactInstructionsArg,
		Title:       "Instructions",
		Description: `e.g. "focus on the failing tests"`,
	}}
}

// goalArguments are the fields the goal mode dialog collects: the goal
// itself, and the number of turns it may consume.
func goalArguments() []commands.Argument {
	return []commands.Argument{
		{
			ID:          GoalTextArg,
			Title:       "Goal",
			Description: `e.g. "make the failing tests pass"`,
			Required:    true,
		},
		{
			ID:          GoalBudgetArg,
			Title:       "Turn budget",
			Description: "how many turns the goal may use (default 25)",
		},
	}
}
