package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"backup-manager/internal/api"
	"backup-manager/internal/api/handler"
	"backup-manager/internal/git"
	"backup-manager/internal/scheduler"
	"backup-manager/internal/servermgr"
	"backup-manager/internal/service"
	"backup-manager/internal/shortcut"
	"backup-manager/internal/store"
	"backup-manager/internal/tray"
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

	appDir := filepath.Join(homeDir, ".config", "backup-manager")
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
	var backupSvc *service.BackupService
	var sched *scheduler.Scheduler

	sched = scheduler.NewScheduler(func(repoID string) error {
		if backupSvc == nil {
			return fmt.Errorf("backup service not initialized")
		}
		_, err := backupSvc.Trigger(repoID, "")
		return err
	})

	repoMu := service.NewRepoMutexManager()
	authSvc := service.NewAuthService(dataStore, keyManager)
	symSvc := service.NewSymlinkService(dataStore)
	previewSvc := service.NewPreviewService(dataStore)
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
	previewHandler := handler.NewPreviewHandler(previewSvc)
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

	// Create desktop shortcut on first run (non-blocking)
	go func() {
		if err := shortcut.CreateOnDesktop(); err != nil {
			log.Printf("warning: failed to create desktop shortcut: %v", err)
		}
	}()

	// Build the server address
	addr := fmt.Sprintf(":%d", appConfig.Port)
	serverURL := fmt.Sprintf("http://localhost%s", addr)

	// Create HTTP server manager for start/stop lifecycle control
	srvMgr := servermgr.New(addr, router)

	// Create system tray manager.
	// Use a pointer variable declared before tray.New so the callbacks can
	// reference it (Go closures capture variables by reference).
	var trayMgr *tray.Manager
	trayMgr = tray.New(tray.Options{
		OnOpenUI: func() {
			log.Printf("[tray] opening UI: %s", serverURL)
			openURL(serverURL)
		},
		OnStartServer: func() {
			log.Println("[tray] starting server...")
			if err := srvMgr.Start(); err != nil {
				log.Printf("[tray] start server error: %v", err)
				return
			}
			trayMgr.SetServerRunning(true)
		},
		OnStopServer: func() {
			log.Println("[tray] stopping server...")
			if err := srvMgr.Stop(); err != nil {
				log.Printf("[tray] stop server error: %v", err)
				return
			}
			trayMgr.SetServerRunning(false)
		},
		OnQuit: func() {
			log.Println("[tray] quitting...")
			// Stop server if running
			_ = srvMgr.Stop()
		},
	})

	// Start the HTTP server
	if err := srvMgr.Start(); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
	trayMgr.SetServerRunning(true)

	// Open browser on startup if configured
	if appConfig.OpenBrowser {
		go openURL(serverURL)
	}

	log.Printf("Backup Manager started on %s", serverURL)
	log.Printf("System tray icon should appear in the menu bar / system tray")

	// Run the system tray event loop (blocks until Quit)
	trayMgr.Run()

	// Tray has exited — perform final cleanup
	log.Println("Backup Manager shutting down...")

	// Stop the HTTP server
	_ = srvMgr.Stop()

	log.Println("Backup Manager stopped")
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
	if err := os.WriteFile(configPath, data, 0600); err != nil {
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
