package tray

import (
	_ "embed"
	"log"
	"sync"

	"github.com/getlantern/systray"
)

//go:embed icon.png
var iconData []byte

// Options configures the tray manager callbacks.
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

// Manager controls the system tray icon and menu.
type Manager struct {
	opts      Options
	running   bool
	mu        sync.Mutex
	quitCh    chan struct{}

	// systray menu item references, set in onReady
	mOpenUI     *systray.MenuItem
	mServerCtrl *systray.MenuItem
	mQuit       *systray.MenuItem
}

// New creates a tray Manager with the given callbacks.
func New(opts Options) *Manager {
	return &Manager{
		opts:   opts,
		quitCh: make(chan struct{}),
	}
}

// Run starts the system tray event loop. This call blocks until the user
// selects "Quit" from the tray menu. Must be called from the main goroutine.
func (m *Manager) Run() {
	systray.Run(m.onReady, m.onExit)
}

// Quit signals the tray to exit. Safe to call from any goroutine.
func (m *Manager) Quit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		systray.Quit()
	}
}

// Done returns a channel that is closed when the tray exits.
func (m *Manager) Done() <-chan struct{} {
	return m.quitCh
}

// UpdateServerStatus updates the Start/Stop menu item text based on whether
// the HTTP server is currently running. Safe to call from any goroutine.
func (m *Manager) UpdateServerStatus(serverRunning bool) {
	if m.mServerCtrl == nil {
		return
	}
	if serverRunning {
		m.mServerCtrl.SetTitle("Stop Server")
		m.mServerCtrl.SetTooltip("Stop the HTTP server")
	} else {
		m.mServerCtrl.SetTitle("Start Server")
		m.mServerCtrl.SetTooltip("Start the HTTP server")
	}
}

// onReady is called by systray when the tray is ready to receive menu items.
func (m *Manager) onReady() {
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()

	if len(iconData) > 0 {
		systray.SetIcon(iconData)
	} else {
		log.Printf("[tray] warning: tray icon is empty")
	}
	// Empty title — macOS menu bar should show icon only, not text
	systray.SetTitle("")
	systray.SetTooltip("Backup Manager")

	// Menu items
	m.mOpenUI = systray.AddMenuItem("Open UI", "Open Backup Manager in browser")
	systray.AddSeparator()
	m.mServerCtrl = systray.AddMenuItem("Stop Server", "Stop the HTTP server")
	systray.AddSeparator()
	m.mQuit = systray.AddMenuItem("Quit", "Exit Backup Manager")

	// Start the menu event handler goroutine
	go m.handleMenuEvents()
}

// onExit is called by systray when the tray is about to exit.
func (m *Manager) onExit() {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()
	close(m.quitCh)
}

// handleMenuEvents listens for clicks on menu items in a loop.
func (m *Manager) handleMenuEvents() {
	for {
		select {
		case <-m.mOpenUI.ClickedCh:
			if m.opts.OnOpenUI != nil {
				m.opts.OnOpenUI()
			}
		case <-m.mServerCtrl.ClickedCh:
			// Toggle: if currently "Stop Server", stop; if "Start Server", start
			if m.mServerCtrl != nil {
				// Check current title to decide action
				// The title is set by UpdateServerStatus, so we need to track state
				if m.opts.OnStartServer != nil && m.opts.OnStopServer != nil {
					// We'll check the title text
					m.handleServerToggle()
				}
			}
		case <-m.mQuit.ClickedCh:
			if m.opts.OnQuit != nil {
				m.opts.OnQuit()
			}
			systray.Quit()
			return
		}
	}
}

// serverRunning is an internal flag to track toggle state since menu item
// title might not be reliable across platforms.
var serverRunning bool

// handleServerToggle dispatches to the appropriate callback based on current state.
func (m *Manager) handleServerToggle() {
	if serverRunning {
		if m.opts.OnStopServer != nil {
			m.opts.OnStopServer()
		}
	} else {
		if m.opts.OnStartServer != nil {
			m.opts.OnStartServer()
		}
	}
}

// SetServerRunning updates the internal running state (for use before onReady).
func (m *Manager) SetServerRunning(running bool) {
	serverRunning = running
	m.UpdateServerStatus(running)
}
