package app

import (
	"context"
	"time"

	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/session"
)

// sessionEndTimeout bounds the SessionEnd hooks so a slow script cannot
// hold the process open on exit.
const sessionEndTimeout = 2 * time.Second

// SessionCreated fires SessionStart for a freshly created session. Every
// session creation path that runs hooks goes through here.
func (app *App) SessionCreated(ctx context.Context, sess session.Session) {
	if sess.ID == "" || app.Sessions == nil {
		return
	}
	app.hookRunner.Run(ctx, hooks.Event{
		Name:      hooks.EventSessionStart,
		SessionID: sess.ID,
		Source:    "startup",
	})
}

// SessionResumed fires SessionStart for a session the user returned to.
func (app *App) SessionResumed(ctx context.Context, sess session.Session) {
	if sess.ID == "" || app.Sessions == nil {
		return
	}
	app.hookRunner.Run(ctx, hooks.Event{
		Name:      hooks.EventSessionStart,
		SessionID: sess.ID,
		Source:    "resume",
	})
}

// TrackCurrentSession records the session the user is working in, so
// SessionEnd fires for it when the app exits.
func (app *App) TrackCurrentSession(sessionID string) {
	app.currentSessionID.Store(sessionID)
	app.ReportCurrentSession(sessionID)
}

// fireSessionEnd fires SessionEnd for the session the user was working
// in. It runs after agents have stopped so Interrupt precedes it.
func (app *App) fireSessionEnd() {
	ctx, cancel := context.WithTimeout(context.Background(), sessionEndTimeout)
	defer cancel()
	sessionID, _ := app.currentSessionID.Load().(string)
	app.hookRunner.Run(ctx, hooks.Event{
		Name:      hooks.EventSessionEnd,
		SessionID: sessionID,
		Reason:    "exit",
	})
}
