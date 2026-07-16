# Backup Manager

A visual management tool for file/directory aggregated backup. Based on Git's reverse tracking mode (whitelist mechanism), allowing users to manage backups intuitively by "specifying what to back up" rather than "what to exclude".

> 🇨🇳 [中文文档](docs/zh/README-zh.md)

## Core Concept

```
Specify which files to back up → Auto-create symlinks to aggregate → Incrementally sync to the backup repo → Git version control
```

### How It Works

1. **Create a backup repo** — Initialize a repo at a local path (contains `.links/`, `data/` directories and `.git/`)
2. **Add symlinks** — Select source files/directories to track, automatically create symlinks in `.links/` and copy source files to `data/`
3. **Run backup** — Incremental detection (mtime+size) → sync changes to `data/` → `git add` → `git commit` → (optional) `git push`

### Repository Directory Structure

```
<repo-root>/
├── .links/        # Symlink directory (pointing to source files)
├── data/          # Actual backup data (mirrors .links/ structure)
└── .git/          # Git repository
```

## Features

- **Repo Management** — Create/delete/view backup repos, visual config (remote URL, branch, Git user)
- **Symlink Management** — Tree view, add/delete/batch import, modify target path, auto-cleanup for deleted source files
- **File Preview** — Plain text/code syntax highlighting, Markdown rendering, binary file identification
- **Backup Execution** — Manual trigger or scheduled auto-backup (second-precision cron), incremental sync, optional Git push
- **Backup History** — View Git commit history with pagination
- **Source File Rollback** — Select a historical commit and restore source files to that version (full or partial)
- **File-Level Restore** — Preview and restore individual files from historical commits
- **Git Integration** — Remote repo config, SSH/HTTPS auth management (AES-256-GCM encrypted storage)
- **Local File Browsing** — Safely scoped to home directory and repo root, prevents path traversal
- **Scheduled Scheduling** — Cron-based auto-backup, auto-load on startup, dynamic register/unregister on config change
- **System Tray** — macOS menu bar / system tray icon, server start/stop control
- **All-in-One Binary** — Single binary, one-click launch

## Quick Start

For a detailed illustrated guide, see the [Quick Start Guide](docs/quick-start.md).

### Prerequisites

- Go 1.22+
- Node.js 18+ (development only)
- Git 2.3+

### One-Click Launch (Production)

```bash
# Download pre-built binary or build yourself
go build -o backup-manager .
./backup-manager
# Automatically opens browser at http://localhost:9800
```

### Development Mode

```bash
# One-click start (both frontend and backend)
./scripts/dev-start.sh

# One-click stop
./scripts/dev-stop.sh
```

Or start separately:

```bash
# Terminal 1: Start backend
go run .

# Terminal 2: Start frontend dev server (hot reload)
cd frontend && npm install && npm run dev
# Frontend at http://localhost:5173, proxies /api to backend
```

### Production Build

```bash
cd frontend && npm install && npm run build && cd ..
go build -o backup-manager .
# Outputs single binary backup-manager
```

## Architecture

```
┌─────────────────────────────────────────────┐
│  Frontend (React SPA, embedded via Go embed) │
├─────────────────────────────────────────────┤
│  API Layer (Gin handlers)                   │
├─────────────────────────────────────────────┤
│  Service Layer (business logic)             │
├─────────────────────────────────────────────┤
│  Store Layer (SQLite) + Git Engine + File IO│
└─────────────────────────────────────────────┘
```

### Tech Stack

| Layer | Technology |
|------|------|
| Backend | Go 1.22+ (Gin, SQLite via modernc.org/sqlite) |
| Frontend | React 18 + TypeScript + Vite + Ant Design 5 |
| State Management | Zustand |
| Scheduling | robfig/cron/v3 |
| Encryption | AES-256-GCM |
| Markdown | react-markdown + remark-gfm |
| Packaging | Go embed (frontend embedded in binary) |

### REST API

All endpoints prefixed with `/api/v1`, unified response format `{"data": ...}` or `{"error": "..."}`.

| Category | Endpoint | Function |
|------|------|------|
| Repos | `POST/GET/DELETE /repos` | Repo CRUD |
| Repos | `GET /repos/:id` | Repo detail (with config and status) |
| Repos | `PUT /repos/:id/config` | Update config (partial update) |
| Repos | `POST /repos/:id/git-init` | Initialize Git repository |
| Symlinks | `POST/GET /repos/:id/symlinks` | Create/list symlinks (list returns `is_new` flag) |
| Symlinks | `GET/DELETE/PUT /repos/:id/symlinks/:linkId` | Detail/delete/update target |
| Symlinks | `POST /repos/:id/symlinks/batch` | Batch import |
| Symlinks | `GET /repos/:id/symlinks/:linkId/entries?sub_path=` | List directory symlink entries |
| Symlinks | `POST /repos/:id/symlinks/:linkId/nested` | Add nested symlink within directory symlink |
| Browse | `GET /browse?path=...` | Browse local filesystem |
| Browse | `GET /browse/allowed-roots` | List allowed browsing roots |
| Preview | `GET /repos/:id/preview?path=...` | Preview source file content |
| Preview | `PUT /repos/:id/save` | Save edited file to source (syncs to data/) |
| Backup | `POST /repos/:id/backup` | Trigger backup (optional `commit_message` body) |
| Backup | `GET /repos/:id/backup/history?limit=&offset=` | Backup history (paginated) |
| Backup | `POST /repos/:id/push` | Push to remote (optional `force` body) |
| Rollback | `GET /repos/:id/commits/:hash/changed-files` | List changed files in commit |
| Rollback | `GET /repos/:id/commits/:hash/files?path=` | Preview file content at commit |
| Rollback | `POST /repos/:id/commits/:hash/restore` | Restore single file from commit |
| Rollback | `POST /repos/:id/rollback` | Batch rollback source files to historical version |
| Auth | `GET/PUT/DELETE /repos/:id/auth` | Git auth management |
| System | `GET /health` | Health check (status + uptime + version) |

## Workflow

```
1. Launch app → System tray icon appears in menu bar
2. Click tray icon → "Open UI" to open browser
3. Dashboard shows repo list
4. Click "Create Repo" → Enter name, select path
5. Enter repo detail → Add symlinks (select source files to back up)
6. View file content in the Preview tab
7. Switch to Backup tab → Click "Trigger Backup"
8. Configure remote repo and auth (optional)
9. Set up scheduled backup (optional)
10. Select a commit in backup history → Rollback source files (optional)
```

## Configuration

| Path | Description |
|------|------|
| `~/.config/backup-manager/config.json` | App config (port, theme, auto-open browser, etc.) — JSON keys: `port`, `open_browser`, `theme` |
| `~/.config/backup-manager/master.key` | AES-256 encryption key (auto-generated on first start) |
| `~/.config/backup-manager/backup-manager.db` | SQLite database |

## Security Design

- **Path Safety**: Four-layer validation (Clean→Abs→EvalSymlinks→Prefix) prevents path traversal
- **Auth Encryption**: SSH private keys and HTTPS passwords encrypted with AES-256-GCM
- **Concurrency Control**: Per-repo mutex prevents concurrent backups; preview API rate-limited (max 5 concurrent)
- **Error Isolation**: Git push failure does not block local commit

## Development

### Testing

```bash
# Run all Go tests
go test ./... -count=1

# Frontend type checking
cd frontend && npx tsc --noEmit
```

### Project Structure

```
backup-manager/
├── main.go                     # Entry: init modules → start HTTP → graceful shutdown
├── scripts/
│   ├── dev-start.sh            # One-click dev environment start
│   └── dev-stop.sh             # One-click dev environment stop
├── internal/                   # Backend code
│   ├── api/                    # API layer (router + handlers)
│   │   ├── router.go           # Route registration + SPA mount
│   │   ├── middleware.go       # CORS + error recovery
│   │   └── handler/            # HTTP handlers
│   │       ├── repo.go         # Repo CRUD + Git Init
│   │       ├── symlink.go      # Symlink CRUD + batch import + nested
│   │       ├── browse.go       # Local file browsing + allowed roots
│   │       ├── preview.go      # File preview + save
│   │       ├── backup.go       # Backup trigger + history + push
│   │       ├── auth.go         # Git auth management
│   │       ├── rollback.go     # Source file rollback + file restore
│   │       ├── system.go       # Health check
│   │       └── errors.go       # Error code mapping
│   ├── service/                # Business logic layer
│   │   ├── repo_service.go     # Repo lifecycle
│   │   ├── symlink_service.go  # Symlink CRUD + mirror sync + nested
│   │   ├── backup_service.go   # Backup execution engine
│   │   ├── auth_service.go     # Git auth management
│   │   ├── browser_service.go  # Safe file browsing
│   │   ├── preview_service.go  # File preview + save logic
│   │   ├── rollback_service.go # Source file rollback logic
│   │   └── repo_mutex.go       # Per-repo mutex
│   ├── store/                  # Data persistence layer
│   │   ├── db.go               # SQLite init + migration
│   │   ├── store.go            # Store aggregation
│   │   ├── repo_store.go       # repos table operations
│   │   ├── repo_config_store.go# repo_configs table operations
│   │   ├── repo_auth_store.go  # repo_auths table operations
│   │   └── symlink_store.go    # symlinks table operations
│   ├── model/                  # Data models
│   │   ├── repo.go             # Repo, RepoConfig, RepoStatus
│   │   ├── symlink.go          # Symlink, SymlinkType
│   │   └── auth.go             # GitAuth, GitAuthType
│   ├── git/                    # Git engine
│   │   └── git.go              # Init/Add/Commit/Push/Log/Status/Config/LsTree/Show/WriteFileContentTo
│   ├── resolver/               # Git path resolver
│   │   └── symlink_resolver.go # data/ path ↔ source file path mapping
│   ├── scheduler/              # Scheduled scheduler
│   │   └── scheduler.go        # Cron-based register/unregister
│   ├── servermgr/              # HTTP server lifecycle manager
│   ├── shortcut/               # Desktop shortcut creation
│   ├── tray/                   # System tray (menu bar) manager
│   └── util/                   # Utilities
│       ├── path.go             # SafeResolve four-layer path validation
│       ├── crypto.go           # KeyManager (AES-256-GCM)
│       └── file.go             # CopyFile/CopyDir/DetectMIME
└── frontend/                   # React SPA
    ├── package.json
    ├── vite.config.ts           # Dev proxy /api → localhost:9800
    └── src/
        ├── main.tsx             # React entry
        ├── App.tsx              # Route config
        ├── App.css              # Global styles
        ├── api/client.ts        # axios instance + all API functions
        ├── types/index.ts       # TypeScript type definitions
        ├── store/appStore.ts    # Zustand state management
        ├── routes/              # Page components
        │   ├── Dashboard.tsx    # Repo list
        │   └── RepoDetail.tsx   # Repo detail (4 tabs)
        └── components/          # Functional components
            ├── layout/
            │   ├── AppLayout.tsx
            │   └── Sidebar.tsx
            ├── repo/
            │   ├── RepoCard.tsx
            │   └── CreateRepoModal.tsx
            ├── symlink/
            │   ├── SymlinkPanel.tsx
            │   └── SymlinkAddModal.tsx
            ├── preview/
            │   ├── PreviewPanel.tsx
            │   ├── TextPreview.tsx
            │   ├── MarkdownPreview.tsx
            │   └── BinaryInfo.tsx
            ├── backup/
            │   ├── BackupPanel.tsx
            │   ├── RollbackConfirmModal.tsx
            │   └── RollbackResultModal.tsx
            └── config/
                └── ConfigPanel.tsx
```

## License

MIT
