package tray

// Options configures the tray manager callbacks.
// All callbacks are optional — set nil to disable the corresponding action.
type Options struct {
	// OnOpenUI is called when the user clicks "Open UI".
	OnOpenUI func()
	// OnStartServer is called when the user clicks "Start Server".
	OnStartServer func()
	// OnStopServer is called when the user clicks "Stop Server".
	OnStopServer func()
	// OnQuit is called when the user clicks "Quit". The tray will exit after this.
	OnQuit func()
}
