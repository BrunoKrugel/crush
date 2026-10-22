package dialog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// ActionClose is a message to close the current dialog.
type ActionClose struct{}

// ActionQuit is a message to quit the application.
type ActionQuit = tea.QuitMsg

// ActionOpenDialog is a message to open a dialog.
type ActionOpenDialog struct {
	DialogID string
}

// ActionSelectSession is a message indicating a session has been selected.
type ActionSelectSession struct {
	Session session.Session
}

// ActionSelectModel is a message indicating a model has been selected.
type ActionSelectModel struct {
	Provider       catwalk.Provider
	ModelType      config.SelectedModelType
	Model          config.SelectedModel
	ReAuthenticate bool
}

// Messages for commands
type (
	ActionNewSession              struct{}
	ActionToggleHelp              struct{}
	ActionToggleCompactMode       struct{}
	ActionToggleThinking          struct{}
	ActionTogglePills             struct{}
	ActionExternalEditor          struct{}
	ActionToggleYoloMode          struct{}
	ActionToggleNotifications     struct{}
	ActionSelectNotificationStyle struct {
		Style string
	}
	ActionToggleTransparentBackground struct{}
	ActionToggleMouseSupport          struct{}
	// ActionFork asks the workspace to branch a conversation from a
	// message: the new session continues the old one up to that point.
	// ActionForkSession asks the UI to open the fork dialog for the
	// current session, forking at its last user message.
	ActionForkSession struct {
		SessionID string
	}
	ActionFork struct {
		// SessionID is the session to fork from.
		SessionID string
		// MessageID is the fork point: the last message the fork keeps.
		MessageID string
		// Title names the forked session; empty derives one.
		Title string
		// Worktree asks for a fresh git worktree for the fork.
		Worktree bool
	}
	ActionSwitchTheme struct {
		Theme string
	}
	ActionPreviewTheme struct {
		Theme string
	}
	ActionRevertThemePreview  struct{}
	ActionPreviewThemePalette struct {
		Base    string
		Palette styles.Palette
	}
	ActionSaveThemePalette struct {
		Name    string
		Base    string
		Palette styles.Palette
	}
	ActionEditTheme struct {
		Name string
	}
	ActionRevertThemePalette    struct{}
	ActionRevertOverriddenTheme struct {
		Name string
	}
	ActionCreateTheme struct {
		Name string
		Base string
	}
	ActionRenameTheme struct {
		OldName string
		NewName string
	}
	ActionDeleteTheme struct {
		Name string
	}
	ActionInitializeProject struct{}
	// ActionSummarize compacts a session. Arguments describes what to
	// collect first, a non-nil Args marks that the values arrived, and
	// Instructions carries the user's emphasis for the checkpoint.
	ActionSummarize struct {
		Args         map[string]string
		SessionID    string
		Instructions string
		Arguments    []commands.Argument
	}
	// ActionSelectReasoningEffort is a message indicating a reasoning effort
	// has been selected.
	ActionSelectReasoningEffort struct {
		Effort string
	}
	ActionPermissionResponse struct {
		Permission permission.PermissionRequest
		Action     PermissionAction
	}
	// ActionRunCustomCommand is a message to run a custom command.
	ActionRunCustomCommand struct {
		Args      map[string]string
		Skill     *skills.Skill
		Content   string
		Arguments []commands.Argument
	}
	// ActionAttachSkill is sent when a skill is selected from the commands
	// dialog to be attached to the conversation as a markdown attachment.
	ActionAttachSkill struct {
		ID   string
		Name string
	}
	// ActionRunMCPPrompt is a message to run a custom command.
	ActionRunMCPPrompt struct {
		Args        map[string]string
		Title       string
		Description string
		PromptID    string
		ClientID    string
		Arguments   []commands.Argument
	}
	// ActionEnableDockerMCP is a message to enable Docker MCP.
	ActionEnableDockerMCP struct{}
	// ActionDisableDockerMCP is a message to disable Docker MCP.
	ActionDisableDockerMCP struct{}
	// ActionMCPReconnect reconnects (restarts) a single MCP server by name.
	// This re-resolves env vars (including OAuth/OIDC token refresh commands)
	// and is the mechanism for refreshing expired credentials.
	ActionMCPReconnect struct {
		ServerName string
	}
	// ActionReloadModelDiscovery triggers model re-discovery for custom
	// providers from the model selection dialog, picking up models that
	// appeared after startup (e.g. an `ollama pull`) without a restart.
	ActionReloadModelDiscovery struct{}
)

// Messages for MCP OAuth authentication dialog.
type (
	// ActionMCPAuthStarted is sent when the user approves authentication
	// for an MCP server. The UI should initiate the actual auth flow
	// using the provided context, which the dialog will cancel if the
	// user closes it.
	ActionMCPAuthStarted struct {
		Ctx  context.Context
		Name string
	}

	// ActionMCPAuthComplete is sent when MCP authentication succeeds.
	ActionMCPAuthComplete struct {
		Name string
	}

	// ActionMCPAuthErrored is sent when MCP authentication fails.
	ActionMCPAuthErrored struct {
		Error error
		Name  string
	}
)

// Messages for API key input dialog.
type (
	ActionChangeAPIKeyState struct {
		State APIKeyInputState
	}
)

// CompactInstructionsArg is the palette instructions argument ID used by
// the compact and preview commands.
const CompactInstructionsArg = "instructions"

// Messages for OAuth2 device flow dialog.
type (
	// ActionInitiateOAuth is sent when the device auth is initiated
	// successfully.
	ActionInitiateOAuth struct {
		DeviceCode      string
		UserCode        string
		VerificationURL string
		ExpiresIn       int
		Interval        int
	}

	// ActionCompleteOAuth is sent when the device flow completes successfully.
	ActionCompleteOAuth struct {
		Token *oauth.Token
	}

	// ActionOAuthErrored is sent when the device flow encounters an error.
	ActionOAuthErrored struct {
		Error error
	}

	// ActionDisconnectMemory removes the stored Honcho sign-in.
	ActionDisconnectMemory struct{}

	// ActionCloseOAuth closes the OAuth dialog and runs the given cleanup
	// command, cancelling any in-flight authorization. It exists so a
	// dismissed dialog does not leave a poller or loopback listener
	// running in the background.
	ActionCloseOAuth struct {
		Cmd tea.Cmd
	}

	// ActionSelectAuthMethod is sent when the user picks how to
	// authenticate a provider that supports both OAuth and API keys.
	ActionSelectAuthMethod struct {
		Provider  catwalk.Provider
		ModelType config.SelectedModelType
		Model     config.SelectedModel
		UseOAuth  bool
	}
)

// ActionCmd represents an action that carries a [tea.Cmd] to be passed to the
// Bubble Tea program loop.
// ActionInsertMCPPrompt is a message to attach an MCP prompt to the prompt
// currently being composed, rather than running it as a whole turn the way
// ActionRunMCPPrompt does. It is produced by the inline "/" completion, where
// the user is mid-sentence and the prompt is one ingredient of their message.
type ActionInsertMCPPrompt struct {
	Args       map[string]string
	Name       string
	MCPName    string
	PromptID   string
	Arguments  []commands.Argument
	StartIndex int
}

type ActionCmd struct {
	Cmd tea.Cmd
}

// ActionFilePickerSelected is a message indicating a file has been selected in
// the file picker dialog.
type ActionFilePickerSelected struct {
	Path string
}

// Cmd returns a command that reads the file at path and sends a
// [message.Attachement] to the program.
func (a ActionFilePickerSelected) Cmd() tea.Cmd {
	path := a.Path
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		isFileLarge, err := common.IsFileTooBig(path, common.MaxAttachmentSize)
		if err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  fmt.Sprintf("unable to read the file: %v", err),
			}
		}
		if isFileLarge {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  "file too large, max 5MB",
			}
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  fmt.Sprintf("unable to read the file: %v", err),
			}
		}

		fileName := filepath.Base(path)
		mimeType, ok := common.SniffAttachmentMIME(content)
		if !ok {
			// Allowed by extension but the content sniffs as binary —
			// attaching would inline byte soup or send a file part
			// providers reject.
			return util.InfoMsg{
				Type: util.InfoTypeWarn,
				Msg:  fmt.Sprintf("%s looks binary (%s), not attaching", fileName, mimeType),
			}
		}

		return message.Attachment{
			FilePath: path,
			FileName: fileName,
			MimeType: mimeType,
			Content:  content,
		}
	}
}
