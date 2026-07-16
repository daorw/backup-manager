package servermgr

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// ServerManager wraps an http.Server and provides start/stop lifecycle control.
// It allows the tray to start and stop the HTTP server at runtime without
// restarting the entire application.
type ServerManager struct {
	mu      sync.Mutex
	server  *http.Server
	handler http.Handler
	addr    string
	running bool
}

// New creates a new ServerManager with the given address and handler.
func New(addr string, handler http.Handler) *ServerManager {
	return &ServerManager{
		addr:    addr,
		handler: handler,
	}
}

// Start begins listening on the configured address. If the server is already
// running, it returns an error. Safe to call from any goroutine.
func (sm *ServerManager) Start() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.running {
		return fmt.Errorf("server is already running on %s", sm.addr)
	}

	sm.server = &http.Server{
		Addr:    sm.addr,
		Handler: sm.handler,
	}

	go func() {
		log.Printf("HTTP server starting on http://localhost%s", sm.addr)
		if err := sm.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
		log.Printf("HTTP server stopped")
	}()

	sm.running = true
	return nil
}

// Stop gracefully shuts down the HTTP server with a 5-second timeout.
// If the server is not running, this is a no-op.
func (sm *ServerManager) Stop() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.running || sm.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Println("Shutting down HTTP server...")
	err := sm.server.Shutdown(ctx)
	sm.running = false
	sm.server = nil

	if err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}
	return nil
}

// IsRunning returns true if the HTTP server is currently active.
func (sm *ServerManager) IsRunning() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.running
}

// Addr returns the configured listen address.
func (sm *ServerManager) Addr() string {
	return sm.addr
}
