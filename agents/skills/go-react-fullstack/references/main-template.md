# main.go Bootstrap Template

Complete Go entry point showing layered initialization with tray/server lifecycle.

```go
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

    "<module>/internal/api"
    "<module>/internal/api/handler"
    "<module>/internal/service"
    "<module>/internal/store"
    "<module>/internal/scheduler"
    "<module>/internal/servermgr"
    "<module>/internal/shortcut"
    "<module>/internal/tray"
    "<module>/internal/util"
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
    homeDir, err := os.UserHomeDir()
    if err != nil {
        log.Fatalf("failed to get home directory: %v", err)
    }

    // --- Config directory ---
    appDir := filepath.Join(homeDir, ".config", "<app>")
    if err := os.MkdirAll(appDir, 0700); err != nil {
        log.Fatalf("failed to create app directory: %v", err)
    }

    // --- Load app config ---
    appConfig := loadConfig(appDir)

    // --- Initialize key manager (AES-256-GCM) ---
    keyPath := filepath.Join(appDir, "master.key")
    keyManager, err := util.NewKeyManager(keyPath)
    if err != nil {
        log.Fatalf("failed to initialize key manager: %v", err)
    }
    defer keyManager.Destroy()

    // --- Open database ---
    dbPath := filepath.Join(appDir, "<app>.db")
    db, err := store.OpenDB(dbPath)
    if err != nil {
        log.Fatalf("failed to open database: %v", err)
    }
    defer db.Close()

    // --- Run migrations ---
    if err := store.Migrate(db); err != nil {
        log.Fatalf("failed to run migrations: %v", err)
    }

    // --- Initialize store ---
    dataStore := store.NewStore(db)

    // --- Initialize services ---
    // ... create service instances ...

    // --- Initialize scheduler ---
    sched := scheduler.NewScheduler(func(id string) error {
        // job callback
        return nil
    })
    sched.Start()
    defer sched.Stop()

    // --- Initialize handlers ---
    // ... create handler instances (handler.NewXxxHandler(svc)) ...

    // --- Setup router ---
    router := api.SetupRouter(
        repoHandler, symlinkHandler, browseHandler,
        previewHandler, backupHandler, authHandler,
        systemHandler, rollbackHandler,
    )

    // --- Mount frontend static files ---
    api.MountStatic(router, frontendAssets)

    // --- Create desktop shortcut (first run, non-blocking) ---
    go func() {
        if err := shortcut.CreateOnDesktop(); err != nil {
            log.Printf("warning: failed to create desktop shortcut: %v", err)
        }
    }()

    // --- Start HTTP server ---
    addr := fmt.Sprintf(":%d", appConfig.Port)
    serverURL := fmt.Sprintf("http://localhost%s", addr)

    srvMgr := servermgr.New(addr, router)

    // --- System tray ---
    var trayMgr *tray.Manager
    trayMgr = tray.New(tray.Options{
        OnOpenUI: func() {
            openURL(serverURL)
        },
        OnStartServer: func() {
            srvMgr.Start()
            trayMgr.SetServerRunning(true)
        },
        OnStopServer: func() {
            srvMgr.Stop()
            trayMgr.SetServerRunning(false)
        },
        OnQuit: func() {
            srvMgr.Stop()
        },
    })

    // Start server and tray
    if err := srvMgr.Start(); err != nil {
        log.Fatalf("failed to start server: %v", err)
    }
    trayMgr.SetServerRunning(true)

    if appConfig.OpenBrowser {
        go openURL(serverURL)
    }

    log.Printf("<App> started on %s", serverURL)
    trayMgr.Run()

    // Cleanup
    log.Println("<App> shutting down...")
    srvMgr.Stop()
    log.Println("<App> stopped")
}

// loadConfig loads application configuration from disk,
// writing the default if not found.
func loadConfig(appDir string) AppConfig {
    config := defaultConfig()
    configPath := filepath.Join(appDir, "config.json")
    data, err := os.ReadFile(configPath)
    if err != nil {
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
        return
    }
    os.WriteFile(filepath.Join(appDir, "config.json"), data, 0600)
}

// openURL opens a URL in the default browser.
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
    exec.Command(cmd, args...).Start()
}
```

## Initialization Order

The `main()` function follows a strict layered initialization:

1. **Config directory** → `~/.config/<app>/`
2. **App config** → `config.json` (port, open_browser, theme)
3. **Key manager** → AES-256-GCM master key (`master.key`, 0600)
4. **Database** → SQLite with WAL mode + FK enabled + migrations
5. **Store** → wraps `*sql.DB`
6. **External engines** → Git, etc.
7. **Services** → business logic (RepoService, SymlinkService, etc.)
8. **Scheduler** → robfig/cron, register auto-backup jobs from DB
9. **Handlers** → HTTP handlers with service dependencies
10. **Router** → Gin router with all API routes + middleware
11. **Static mount** → embed.FS for frontend SPA
12. **Desktop shortcut** → goroutine, first run only
13. **Server manager** → wraps HTTP server lifecycle
14. **Tray manager** → system tray with Start/Stop/Quit callbacks
15. **Server start** → `srvMgr.Start()`
16. **Browser open** → goroutine, conditional on config
17. **Tray.Run()** → blocks until user quits
18. **Cleanup** → `srvMgr.Stop()`, deferred DB close, key destroy

## Policy

- `log.Fatalf` for startup failures (can't run without these)
- `log.Printf` + continue for non-critical failures (desktop shortcut, browser)
- Defer cleanup in reverse init order
- Single process, no daemonization
