package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"backup-manager/internal/api"
	"backup-manager/internal/api/handler"
	"backup-manager/internal/git"
	"backup-manager/internal/scheduler"
	"backup-manager/internal/service"
	"backup-manager/internal/store"
	"backup-manager/internal/util"
)

//go:embed frontend/dist/*
var frontendAssets embed.FS

// AppConfig is the application-level configuration.
type AppConfig struct {
	Port        int    `json:"port"`
	OpenBrowser bool   `json:"open_browser"`
	Theme       string `json:"theme"`
}

func defaultConfig() AppConfig {
	return AppConfig{
		Port:        9800,
		OpenBrowser: true,
		Theme:       "light",
	}
}

func main() {
	// Determine app data directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to get home directory: %v", err)
	}

	appDir := filepath.Join(homeDir, ".backup-manager")
	if err := os.MkdirAll(appDir, 0700); err != nil {
		log.Fatalf("failed to create app directory: %v", err)
	}

	// Load app config
	appConfig := loadConfig(appDir)

	// Initialize key manager (AES-256-GCM)
	keyPath := filepath.Join(appDir, "master.key")
	keyManager, err := util.NewKeyManager(keyPath)
	if err != nil {
		log.Fatalf("failed to initialize key manager: %v", err)
	}
	defer keyManager.Destroy()

	// Open database
	dbPath := filepath.Join(appDir, "backup-manager.db")
	db, err := store.OpenDB(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := store.Migrate(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// Initialize store
	dataStore := store.NewStore(db)

	// Initialize git engine
	gitEngine := git.NewGitEngine()

	// Initialize services
	// Scheduler needs backup service callback, so we create it later
	var backupSvc *service.BackupService
	var sched *scheduler.Scheduler

	// Create scheduler with a placeholder, will be updated after backupSvc is created
	sched = scheduler.NewScheduler(func(repoID string) error {
		if backupSvc == nil {
			return fmt.Errorf("backup service not initialized")
		}
		_, err := backupSvc.Trigger(repoID)
		return err
	})

	repoMu := service.NewRepoMutexManager()
	authSvc := service.NewAuthService(dataStore, keyManager)
	symSvc := service.NewSymlinkService(dataStore)
	backupSvc = service.NewBackupService(dataStore, gitEngine, symSvc, authSvc, repoMu)
	rollbackSvc := service.NewRollbackService(dataStore, gitEngine, repoMu)
	repoSvc := service.NewRepoService(dataStore, gitEngine, sched)
	browserSvc := service.NewBrowserService(dataStore, homeDir)

	// Register auto-backup jobs for repos with auto_backup enabled
	registerAutoBackups(repoSvc, sched)

	// Start scheduler
	sched.Start()
	defer sched.Stop()

	// Initialize handlers
	repoHandler := handler.NewRepoHandler(repoSvc)
	symlinkHandler := handler.NewSymlinkHandler(symSvc)
	browseHandler := handler.NewBrowseHandler(browserSvc)
	previewHandler := handler.NewPreviewHandler(repoSvc)
	backupHandler := handler.NewBackupHandler(backupSvc)
	authHandler := handler.NewAuthHandler(authSvc)
	systemHandler := handler.NewSystemHandler()
	rollbackHandler := handler.NewRollbackHandler(rollbackSvc)

	// Setup router
	router := api.SetupRouter(
		repoHandler,
		symlinkHandler,
		browseHandler,
		previewHandler,
		backupHandler,
		authHandler,
		systemHandler,
		rollbackHandler,
	)

	// Mount frontend static files
	api.MountStatic(router, frontendAssets)
	log.Println("Frontend static files mounted")

	// Create HTTP server
	addr := fmt.Sprintf(":%d", appConfig.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Backup Manager starting on http://localhost%s", addr)
		log.Printf("API docs: http://localhost%s/api/v1/health", addr)

		if appConfig.OpenBrowser {
			openURL(fmt.Sprintf("http://localhost%s", addr))
		}

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

// loadConfig loads application configuration from disk.
func loadConfig(appDir string) AppConfig {
	config := defaultConfig()
	configPath := filepath.Join(appDir, "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		// Write default config
		saveConfig(appDir, config)
		return config
	}

	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("warning: failed to parse config, using defaults: %v", err)
		saveConfig(appDir, config)
	}

	return config
}

// saveConfig writes application configuration to disk.
func saveConfig(appDir string, config AppConfig) {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Printf("warning: failed to marshal config: %v", err)
		return
	}

	configPath := filepath.Join(appDir, "config.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		log.Printf("warning: failed to write config: %v", err)
	}
}

// registerAutoBackups registers cron jobs for repos with auto_backup enabled.
func registerAutoBackups(repoSvc *service.RepoService, sched *scheduler.Scheduler) {
	repos, configs, err := repoSvc.List()
	if err != nil {
		log.Printf("warning: failed to list repos for auto-backup registration: %v", err)
		return
	}

	for i, repo := range repos {
		if i >= len(configs) {
			break
		}
		config := configs[i]
		if config.AutoBackup && config.AutoBackupInterval != "" {
			if err := sched.Register(repo.ID, config.AutoBackupInterval); err != nil {
				log.Printf("warning: failed to register auto-backup for repo %s: %v", repo.ID, err)
			} else {
				log.Printf("registered auto-backup for repo %s (interval: %s)", repo.Name, config.AutoBackupInterval)
			}
		}
	}
}

// openURL attempts to open a URL in the default browser.
func openURL(url string) {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return
	}

	execPath, err := exec.LookPath(cmd)
	if err != nil {
		log.Printf("warning: could not open browser (%s not found)", cmd)
		return
	}

	// Fork and detach the browser process
	procAttr := &os.ProcAttr{
		Files: []*os.File{nil, nil, nil},
	}
	process, err := os.StartProcess(execPath, append([]string{cmd}, args...), procAttr)
	if err != nil {
		log.Printf("warning: failed to open browser: %v", err)
		return
	}
	process.Release()
}
