//go:build linux

package tray

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Manager provides the same public API as the platform tray Manager,
// but on Linux there is no system tray. Run() blocks until the process
// receives SIGINT or SIGTERM, making the program behave as a headless
// server.
type Manager struct {
	opts    Options
	running bool
	mu      sync.Mutex
	quitCh  chan struct{}
}

// New creates a headless tray Manager for Linux.
func New(opts Options) *Manager {
	return &Manager{
		opts:   opts,
		quitCh: make(chan struct{}),
	}
}

// Run blocks until Quit() is called or SIGINT/SIGTERM is received.
func (m *Manager) Run() {
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()

	log.Println("[tray] headless mode — no system tray on Linux")
	log.Println("[tray] waiting for SIGINT/SIGTERM or Quit()...")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		log.Println("[tray] received signal, shutting down...")
	case <-m.quitCh:
		log.Println("[tray] quit requested, shutting down...")
	}

	signal.Stop(sigCh)

	m.mu.Lock()
	m.running = false
	m.mu.Unlock()

	close(m.quitCh)
}

// Quit signals the tray to exit. Safe to call from any goroutine.
func (m *Manager) Quit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		select {
		case m.quitCh <- struct{}{}:
		default:
		}
	}
}

// Done returns a channel that is closed when the tray exits.
func (m *Manager) Done() <-chan struct{} {
	return m.quitCh
}

// UpdateServerStatus is a no-op on Linux.
func (m *Manager) UpdateServerStatus(serverRunning bool) {}

// SetServerRunning is a no-op on Linux.
func (m *Manager) SetServerRunning(running bool) {}
