---
name: go-react-fullstack
description: >
  Go 后端 + React 前端全栈单体应用架构模板。单二进制部署、SQLite 持久化、
  embed.FS 静态文件内嵌、跨平台系统托盘、GitHub CI/CD 多平台构建发布。
  适用于桌面工具、后台管理系统、本地优先应用等场景。
  When the user asks to create a new project with Go backend and React frontend,
  or deploy as a single binary with embedded frontend, or set up a full-stack
  Go+React project with GitHub release pipeline.
agent_created: true
---

# Go + React Fullstack Monolith Template

Single-binary, full-stack application architecture: Go (Gin) backend serving
a React SPA frontend embedded via `embed.FS`. SQLite for persistence,
cross-platform system tray support, and multi-platform GitHub CI/CD release
pipeline.

## Prerequisites

- Go 1.22+ (see `go.mod` for exact version)
- Node.js 18+ (development only)
- Git
- macOS: Xcode Command Line Tools (for CGO with systray)

## Directory Structure

```
<project-root>/
├── main.go                         # Entry point
├── go.mod / go.sum
├── .gitignore
├── README.md
├── AGENTS.md                       # Project conventions (see references/)
├── DESIGN.md                       # Technical design
├── REQUIREMENT.md                  # Product requirements
├── assets/                         # App icons, images
│   └── app-icon.png                # 1024x1024 application icon
├── docs/
│   ├── quick-start.md
│   └── zh/                         # Chinese docs mirror
├── scripts/
│   ├── dev-start.sh                # One-click dev start
│   └── dev-stop.sh                 # One-click dev stop
├── internal/
│   ├── api/
│   │   ├── router.go               # Route registration + SPA mount
│   │   ├── middleware.go           # CORS + error recovery
│   │   └── handler/                # HTTP handlers (*_handler.go or handler/*.go)
│   ├── model/                      # Data models (structs, type aliases)
│   ├── service/                    # Business logic layer
│   ├── store/                      # Data persistence (SQLite)
│   │   ├── db.go                   # DB open + migration
│   │   └── store.go                # Store aggregation
│   ├── git/                        # External tool wrappers (e.g., git engine)
│   ├── scheduler/                  # Cron-based scheduling
│   ├── util/                       # Shared utilities (path, crypto, file)
│   ├── servermgr/                  # HTTP server lifecycle (start/stop)
│   ├── shortcut/                   # Desktop shortcut creation
│   └── tray/                       # System tray (menu bar) manager
├── frontend/
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── index.html
│   └── src/
│       ├── main.tsx                # React entry
│       ├── App.tsx                 # Route config
│       ├── App.css                 # Global styles
│       ├── api/client.ts           # Axios instance + all API functions
│       ├── types/index.ts          # TypeScript type definitions
│       ├── store/                  # State management (Zustand)
│       ├── routes/                 # Page components
│       └── components/             # Reusable UI components
└── .github/
    └── workflows/
        └── release.yml             # Multi-platform build + release
```

### Naming Conventions

| Layer | Rule | Example |
|-------|------|---------|
| Go package | lowercase singular | `store`, `model`, `service` |
| Go file | snake_case | `repo_service.go`, `auth_handler.go` |
| Go test | `_test.go` suffix, same dir | `path_test.go` |
| Go error | `fmt.Errorf("context: %w", err)` wrap | — |
| Go JSON tag | snake_case | `json:"last_backup_at,omitempty"` |
| Go nullable time | `*time.Time` | — |
| React component | PascalCase file | `BackupPanel.tsx` |
| TS utility | camelCase file | `client.ts` |
| TS types | centralized `types/index.ts` | — |

## Technology Stack

### Backend (Go)

| Component | Library | Version | Purpose |
|-----------|---------|---------|---------|
| HTTP framework | `github.com/gin-gonic/gin` | latest | Lightweight HTTP router |
| Database | `modernc.org/sqlite` | latest | Pure-Go SQLite (no CGO) |
| Cron scheduler | `github.com/robfig/cron/v3` | v3 | Second-precision cron |
| UUID | `github.com/google/uuid` | latest | ID generation |
| System tray | `github.com/getlantern/systray` | latest | macOS/Windows tray icon |
| Encryption | `crypto/aes` + `crypto/gcm` | stdlib | AES-256-GCM |

### Frontend (React)

| Component | Library | Version | Purpose |
|-----------|---------|---------|---------|
| UI framework | `antd` | ^5.20 | Component library |
| Icons | `@ant-design/icons` | ^5.4 | Icon set |
| HTTP client | `axios` | ^1.7 | API calls |
| State | `zustand` | ^5 | Lightweight state |
| Routing | `react-router-dom` | ^6.26 | SPA routing |
| Markdown | `react-markdown` + `remark-gfm` | ^9 / ^4 | MD rendering |
| Dates | `dayjs` | ^1.11 | Date utilities |
| Build tool | `vite` | ^5.4 | Fast dev server + bundler |
| Language | `typescript` | ^5.5 | Type safety |

## Project Initialization Workflow

### Step 1: Initialize Go module

```bash
go mod init <module-name>
```

### Step 2: Install Go dependencies

```bash
go get github.com/gin-gonic/gin
go get modernc.org/sqlite
go get github.com/robfig/cron/v3
go get github.com/google/uuid
go get github.com/getlantern/systray
```

### Step 3: Scaffold React frontend

```bash
mkdir -p frontend && cd frontend
npm init -y
npm install react@18 react-dom@18 antd @ant-design/icons axios dayjs \
  react-router-dom react-markdown remark-gfm zustand
npm install -D typescript vite @vitejs/plugin-react @types/react @types/react-dom
```

### Step 4: Configure Vite proxy

Create `frontend/vite.config.ts` — see `references/vite-config.md`.

### Step 5: Create dev scripts

Create `scripts/dev-start.sh` and `scripts/dev-stop.sh` — see `references/dev-scripts.md`.

### Step 6: Set up CI/CD

Create `.github/workflows/release.yml` — see `references/github-ci.md`.

### Step 7: Bootstrap main.go

Implement the layered initialization order:
1. Load app config (port, theme, open_browser)
2. Initialize key manager (AES-256-GCM master key)
3. Open SQLite database + run migrations
4. Create Store → GitEngine → Services → Scheduler
5. Register auto-backup jobs from DB
6. Start scheduler
7. Create handlers → SetupRouter → MountStatic
8. Create desktop shortcut (goroutine, first run only)
9. Create ServerManager + TrayManager
10. srvMgr.Start() → trayMgr.SetServerRunning(true)
11. Open browser (if configured)
12. trayMgr.Run() → blocks until quit

### Step 8: Create .gitignore

```
backup-manager       # binary
frontend/dist/       # build output
frontend/node_modules/
*.db                 # SQLite database
*.key                # encryption keys
```

## Architecture Principles

### Layered Backend

```
Handler (parse request, return response)
  → Service (business logic, orchestration)
    → Store (SQLite CRUD)
    → GitEngine / external tools
```

### Response Convention

All API responses use the unified format:
```json
{"data": <payload>}
```
or
```json
{"error": "<message>"}
```

The frontend axios interceptor auto-unwraps `{ data: ... }` so callers receive the payload directly.

### Error Mapping

The `respondError` helper maps error strings to HTTP status:
- Contains `"not found"` → 404
- Contains `"is required"` or `"invalid"` → 400
- Otherwise → 500

### Frontend-Backend Integration

**Dev mode**: Vite dev server (port 5173) proxies `/api/*` → Go backend (port 9800).

**Production mode**: Go binary serves the built frontend via `embed.FS` + Gin static file middleware. All non-API GET requests serve `index.html` (SPA fallback).

### Static File Mounting Pattern

```go
//go:embed frontend/dist/*
var frontendAssets embed.FS

// In router setup:
api.MountStatic(router, frontendAssets)
```

The `MountStatic` function:
1. Opens `frontend/dist` subdirectory from embed.FS
2. For each GET request not under `/api/`:
   - If the file exists → serve it
   - Otherwise → serve `index.html` (SPA routing)

### Type Sharing Strategy

Backend models define the canonical structure (Go struct with JSON tags).
Frontend mirrors in `types/index.ts`:

```typescript
// frontend/src/types/index.ts
export interface Repo {
  id: string;
  name: string;
  path: string;
  status: 'active' | 'error' | 'backing_up';
  // ...
}
```

No code generation — types are manually kept in sync. The `api/client.ts` file imports these types for all API functions.

### Axios Client Pattern

```typescript
import axios from 'axios';

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
});

// Response interceptor unwraps { data: ... }
api.interceptors.response.use(
  (response) => {
    if (response.data && 'data' in response.data) {
      response.data = response.data.data;
    }
    return response;
  },
  (error) => {
    const message = error.response?.data?.error || error.message || 'Request failed';
    return Promise.reject(new Error(message));
  }
);

// API functions export
export async function fetchRepos(): Promise<Repo[]> {
  const { data } = await api.get('/repos');
  return data;
}
```

### React Component Structure

```
src/
├── main.tsx         # ReactDOM.createRoot + App
├── App.tsx          # BrowserRouter + Routes + ConfigProvider
├── api/client.ts    # All API calls
├── types/index.ts   # All TS interfaces
├── store/           # Zustand stores
├── routes/          # Page-level components
└── components/      # Reusable components, organized by domain
    ├── layout/      # AppLayout, Sidebar
    ├── repo/        # RepoCard, CreateRepoModal
    └── ...
```

### State Management (Zustand)

```typescript
import { create } from 'zustand';

interface AppState {
  repos: Repo[];
  selectedRepoId: string | null;
  loading: boolean;
  setRepos: (repos: Repo[]) => void;
  fetchRepos: () => Promise<void>;
}

export const useAppStore = create<AppState>((set) => ({
  repos: [],
  selectedRepoId: null,
  loading: false,
  setRepos: (repos) => set({ repos }),
  fetchRepos: async () => {
    set({ loading: true });
    const repos = await fetchReposApi();
    set({ repos, loading: false });
  },
}));
```

### SQLite Migration Pattern

```go
// store/db.go
func OpenDB(dbPath string) (*sql.DB, error) {
    db, err := sql.Open("sqlite", dbPath)
    // Enable WAL mode
    db.Exec("PRAGMA journal_mode=WAL")
    // Enable foreign keys
    db.Exec("PRAGMA foreign_keys=ON")
    return db, nil
}

func Migrate(db *sql.DB) error {
    schema := `
    CREATE TABLE IF NOT EXISTS items (
        id         TEXT PRIMARY KEY,
        name       TEXT NOT NULL,
        created_at DATETIME DEFAULT (datetime('now'))
    );
    `
    _, err := db.Exec(schema)
    return err
}
```

### Path Security

All user-supplied file paths must pass through a multi-layer validation function:
1. `filepath.Clean()` — normalize
2. `filepath.Abs()` — resolve to absolute
3. `filepath.EvalSymlinks()` — prevent symlink escape
4. `strings.HasPrefix()` — verify within allowed root

Non-existent files are permitted for create operations (fs.ErrNotExist); other stat errors are rejected.

### Concurrency Control

- Per-entity mutex via `sync.Mutex` map
- API rate limiting via channel semaphore (`make(chan struct{}, maxConcurrency)`)
- Deferred unlock with `defer mu.Unlock()`

## Build and Run

### Development

```bash
# Terminal 1: Backend
go run .

# Terminal 2: Frontend (hot reload)
cd frontend && npm install && npm run dev
# → http://localhost:5173 (proxies /api to :9800)

# OR one-click:
./scripts/dev-start.sh
./scripts/dev-stop.sh
```

### Production Build

```bash
cd frontend && npm install && npm run build && cd ..
go build -ldflags="-s -w" -o <app-name> .
# Output: single binary with embedded frontend
```

### Cross-Platform CI Builds

Three separate build jobs per platform for different CGO requirements:
- **Linux**: `CGO_ENABLED=0` (pure Go, no systray)
- **macOS**: `CGO_ENABLED=1` (needs Cocoa for systray)
- **Windows**: `CGO_ENABLED=0` (pure Go systray via syscall)

See `references/github-ci.md` for the full workflow.

## Security Practices

- AES-256-GCM encryption for sensitive data at rest (SSH keys, passwords)
- Master key stored at `~/.config/<app>/master.key` with `0600` permissions
- Path traversal prevention via four-layer validation
- Preview/edit operations rate-limited (max 5 concurrent)
- Sensitive fields masked in API responses (e.g., `has_password: true` instead of actual password)

## Documentation

- English as primary language, Chinese mirror in `docs/zh/`
- Top of each English doc links to Chinese version: `> 🇨🇳 [中文文档](docs/zh/<name>.md)`
- Keep all docs in sync when making functional changes
