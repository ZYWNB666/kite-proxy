//go:build !desktop

package desktop

// startTray is a no-op in non-desktop builds so package tests can compile.
func (a *App) startTray() {}
