package model

import (
	"errors"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/stretchr/testify/require"
)

// imageCapableConfig returns a config whose coder agent points at a model
// with SupportsImages=true, so currentModelSupportsImages() is true.
func imageCapableConfig() *config.Config {
	providers := csync.NewMap[string, config.ProviderConfig]()
	providers.Set("test-provider", config.ProviderConfig{
		ID: "test-provider",
		Models: []catwalk.Model{
			{ID: "test-model", SupportsImages: true},
		},
	})
	return &config.Config{
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {
				Provider: "test-provider",
				Model:    "test-model",
			},
		},
		Providers: providers,
		Agents: map[string]config.Agent{
			config.AgentCoder: {Model: config.SelectedModelTypeLarge},
		},
	}
}

// newTestUIWithImageSupport returns a fully wired test UI (focus, textarea,
// layout — via newTestUI) whose workspace reports an image-capable coder
// model, so currentModelSupportsImages() is true. We can't just swap the
// com.Config pointer because currentModelSupportsImages reads through
// m.com.Workspace.Config(); instead we embed a workspace stub that returns
// the config we want.
func newTestUIWithImageSupport() *UI {
	u := newTestUI()
	u.com.Workspace = &testWorkspace{cfg: imageCapableConfig()}
	return u
}

// clipboardCmdPointer returns the function pointer for the
// pasteImageFromClipboard method value on u. The paste-routing tests below
// assert against it to prove handlePasteMsg took the clipboard-fallback branch
// WITHOUT executing the returned command (which would touch the real system
// clipboard and go flaky per-environment).
//
// This deliberately couples the tests to the *mechanism* — handlePasteMsg
// returning m.pasteImageFromClipboard directly — rather than the behavior. Any
// future wrapping of that return (e.g. adding a warn-toast closure) will break
// these assertions even when routing is still correct; that is intentional, and
// whoever trips it should update the pointer comparison to match the new
// return shape rather than reverting the production change.
func clipboardCmdPointer(u *UI) uintptr {
	return reflect.ValueOf(u.pasteImageFromClipboard).Pointer()
}

// TestHandlePasteMsg_EmptyContentRoutesToClipboardImage is a regression test
// for GitHub issue #197: CMD+V paste of image from clipboard does nothing on
// macOS.
//
// When the system clipboard holds an image (not text), the terminal's paste
// produces an empty tea.PasteMsg. Before the fix, handlePasteMsg would flow
// through the text-paste path and silently do nothing (empty text insert).
// After the fix, an empty paste on an image-capable model delegates to
// pasteImageFromClipboard so image data on the clipboard is attached instead.
func TestHandlePasteMsg_EmptyContentRoutesToClipboardImage(t *testing.T) {
	t.Parallel()

	u := newTestUIWithImageSupport()
	u.dialog = dialog.NewOverlay()
	u.textarea.SetValue("existing text")

	// Empty paste - simulates clipboard holding an image on macOS.
	cmd := u.handlePasteMsg(tea.PasteMsg{Content: ""})

	// The textarea value must be unchanged.
	require.Equal(t, "existing text", u.textarea.Value(),
		"empty paste must not modify textarea content; it should route to clipboard image fallback")

	// The returned command must be the pasteImageFromClipboard method value,
	// proving the clipboard fallback path was taken (not the text insert path).
	require.NotNil(t, cmd, "empty paste must return a command (the clipboard fallback)")
	require.Equal(t, clipboardCmdPointer(u), reflect.ValueOf(cmd).Pointer(),
		"empty paste must route to pasteImageFromClipboard, not the text insertion path")
}

// TestHandlePasteMsg_EmptyContentOnImageIncapableModelWarns ensures the
// clipboard-image fallback is gated to image-capable models, mirroring the
// key-bound path (ui.go:2476-2480). On a text-only model, an empty paste must
// not attach an image chip that preparePrompt would silently drop at send
// time — but it must not silently no-op either: it returns a warning command
// so the user can see why nothing landed (the silent nil return is what made
// #197 undiagnosable from the UX).
func TestHandlePasteMsg_EmptyContentOnImageIncapableModelWarns(t *testing.T) {
	t.Parallel()

	// Start from the fully wired newTestUI (focus, textarea, layout) and
	// inject an image-incapable workspace: an empty config has no coder
	// agent, so currentModelSupportsImages() returns false.
	u := newTestUI()
	u.com.Workspace = &testWorkspace{cfg: &config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
		Agents:    map[string]config.Agent{},
	}}
	require.False(t, u.currentModelSupportsImages(),
		"test precondition: the stub config must be image-incapable")
	u.dialog = dialog.NewOverlay()
	u.textarea.SetValue("existing text")

	cmd := u.handlePasteMsg(tea.PasteMsg{Content: ""})

	require.NotNil(t, cmd,
		"empty paste on an image-incapable model must return a warning command, not nil")
	msg := cmd()
	infoMsg, ok := msg.(util.InfoMsg)
	require.True(t, ok, "expected util.InfoMsg, got %T", msg)
	require.Equal(t, util.InfoTypeWarn, infoMsg.Type,
		"incapable-model paste must warn the user")
	require.Contains(t, infoMsg.Msg, "doesn't support images")
	require.NotEqual(t, clipboardCmdPointer(u), reflect.ValueOf(cmd).Pointer(),
		"empty paste on an image-incapable model must not route to clipboard image fallback")
	require.Equal(t, "existing text", u.textarea.Value(),
		"empty paste must not modify textarea content")
}

// runKeyCmd runs a command the way the Bubble Tea runtime would, flattening
// a batch so tests can look for whichever message a path produced.
func runKeyCmd(msg tea.Msg) []tea.Msg {
	var out []tea.Msg
	switch m := msg.(type) {
	case tea.Cmd:
		if m == nil {
			return nil
		}
		out = append(out, runKeyCmd(m())...)
	case tea.BatchMsg:
		for _, c := range m {
			if c == nil {
				continue
			}
			out = append(out, runKeyCmd(c())...)
		}
	case nil:
		return nil
	default:
		out = append(out, m)
	}
	return out
}

// TestPasteKeyOnImageIncapableModelWarnsOnlyForAnImage exercises the
// key-bound path (ctrl+v / super+v) on a text-only model: a clipboard image
// must warn instead of attaching, and a clipboard that holds text must still
// paste it. The gate used to fire on the key alone, which refused every paste
// on a vision-less model, text included.
func TestPasteKeyOnImageIncapableModelWarnsOnlyForAnImage(t *testing.T) {
	// Not parallel: these stub the package-level clipboard seam, which is
	// process-global state shared by every paste test.

	imageOnClipboard := func(t *testing.T) {
		t.Helper()
		orig := readClipboardImage
		readClipboardImage = func() ([]byte, error) { return []byte("png-bytes"), nil }
		t.Cleanup(func() { readClipboardImage = orig })
	}

	t.Run("image on the clipboard warns", func(t *testing.T) {
		// No t.Parallel: the clipboard seam is process-global.
		u := newSidebarTestUI()
		u.com.Workspace = &testWorkspace{cfg: &config.Config{
			Providers: csync.NewMap[string, config.ProviderConfig](),
			Agents:    map[string]config.Agent{},
		}}
		u.attachments = attachments.New(nil, attachments.Keymap{})
		require.False(t, u.currentModelSupportsImages(),
			"test precondition: the stub config must be image-incapable")
		imageOnClipboard(t)

		cmd := u.handleKeyPressMsg(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
		require.NotNil(t, cmd, "an image paste on a vision-less model must warn")
		var infoMsg util.InfoMsg
		for _, got := range runKeyCmd(cmd()) {
			if im, ok := got.(util.InfoMsg); ok {
				infoMsg = im
				break
			}
		}
		require.Equal(t, util.InfoTypeWarn, infoMsg.Type)
		require.Contains(t, infoMsg.Msg, "doesn't support images")
		require.Empty(t, u.textarea.Value(), "the refused image must not land as text")
	})

	t.Run("text on the clipboard pastes", func(t *testing.T) {
		// No t.Parallel: the clipboard seam is process-global.
		u := newSidebarTestUI()
		u.com.Workspace = &testWorkspace{cfg: &config.Config{
			Providers: csync.NewMap[string, config.ProviderConfig](),
			Agents:    map[string]config.Agent{},
		}}
		u.attachments = attachments.New(nil, attachments.Keymap{})
		u.textarea.SetValue("existing text")
		require.False(t, u.currentModelSupportsImages(),
			"test precondition: the stub config must be image-incapable")

		origImage, origText := readClipboardImage, readClipboardText
		readClipboardImage = func() ([]byte, error) { return nil, errors.New("no image") }
		readClipboardText = func() ([]byte, error) { return []byte("pasted text"), nil }
		t.Cleanup(func() { readClipboardImage, readClipboardText = origImage, origText })

		cmd := u.handleKeyPressMsg(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
		require.NotNil(t, cmd)
		var paste tea.PasteMsg
		for _, got := range runKeyCmd(cmd()) {
			if pm, ok := got.(tea.PasteMsg); ok {
				paste = pm
				break
			}
		}
		require.Equal(t, "pasted text", paste.Content,
			"a text paste must not be gated on the model's vision")
	})
}

// TestPasteTextKeyPastesClipboardText covers the dedicated text-paste key,
// which reads the clipboard as text on every model.
func TestPasteTextKeyPastesClipboardText(t *testing.T) {
	// Not parallel: these stub the package-level clipboard seam, which is
	// process-global state shared by every paste test.

	u := newSidebarTestUI()
	u.com.Workspace = &testWorkspace{cfg: &config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
		Agents:    map[string]config.Agent{},
	}}
	u.attachments = attachments.New(nil, attachments.Keymap{})

	orig := readClipboardText
	readClipboardText = func() ([]byte, error) { return []byte("  spaced  "), nil }
	t.Cleanup(func() { readClipboardText = orig })

	cmd := u.handleKeyPressMsg(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl | tea.ModShift})
	require.NotNil(t, cmd)
	var paste tea.PasteMsg
	for _, got := range runKeyCmd(cmd()) {
		if pm, ok := got.(tea.PasteMsg); ok {
			paste = pm
			break
		}
	}
	require.Equal(t, "  spaced  ", paste.Content)
}

// TestHandlePasteMsg_WhitespaceOnlyContentInsertsText ensures that a
// whitespace-only paste is NOT routed to the clipboard fallback — the #197
// repro produces a strictly empty message, and the fix uses a strict == ""
// check so genuine whitespace pastes (e.g. pasting indentation) still insert
// as text rather than being silently dropped.
func TestHandlePasteMsg_WhitespaceOnlyContentInsertsText(t *testing.T) {
	t.Parallel()

	u := newTestUIWithImageSupport()
	u.dialog = dialog.NewOverlay()
	u.textarea.SetValue("")

	cmd := u.handlePasteMsg(tea.PasteMsg{Content: "   \n\t  "})

	// Whitespace content is real text and must be inserted (the textarea may
	// tab-expand, so we check it's non-empty rather than byte-identical),
	// not swallowed by the clipboard fallback.
	require.NotEqual(t, "", u.textarea.Value(),
		"whitespace-only paste must insert as text, not route to clipboard fallback")

	// It must NOT route to the clipboard image path.
	if cmd != nil {
		require.NotEqual(t, clipboardCmdPointer(u), reflect.ValueOf(cmd).Pointer(),
			"whitespace-only paste must not route to clipboard image fallback")
	}
}

// TestHandlePasteMsg_NonEmptyContentInsertsText is the control test: real
// text paste content must still go through the normal text insertion path
// and must NOT route to the clipboard fallback.
func TestHandlePasteMsg_NonEmptyContentInsertsText(t *testing.T) {
	t.Parallel()

	u := newTestUIWithImageSupport()
	u.dialog = dialog.NewOverlay()
	u.textarea.SetValue("")

	cmd := u.handlePasteMsg(tea.PasteMsg{Content: "hello world"})

	require.Contains(t, u.textarea.Value(), "hello world",
		"non-empty paste must insert text into the textarea")

	// Non-empty text must NOT route to the clipboard image path.
	if cmd != nil {
		require.NotEqual(t, clipboardCmdPointer(u), reflect.ValueOf(cmd).Pointer(),
			"non-empty paste must not route to clipboard image fallback")
	}
}
