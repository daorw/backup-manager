# Backup Manager

A visual management tool for file/directory aggregated backup. Based on Git's reverse tracking mode (whitelist mechanism), allowing users to manage backups intuitively by "specifying what to back up" rather than "what to exclude".

> 🇨🇳 [中文文档](docs/zh/README-zh.md)

## Core Concept

```
Specify what to back up → content moves into the repository → local paths become symlink views → Git version control
```

### How It Works

1. **Create a backup repo** — Initialize a repo at a local path (contains `data/`, `.backup-manager/` and `.git/`)
2. **Create an entry** — Select a local file/directory to track. Its content is **moved** into `data/<repo_path>` and the original location is replaced by a symlink — the entry's **`in`** link
3. **Distribute (optional)** — Add **`out`** links to make the same entry available at further local paths. `in` is a special case of `out`: both are symlinks to `data/<repo_path>`
4. **Run backup** — Flush the manifest → `git add -A` → `git commit` → (optional) `git push`. There is no incremental sync step, because the content already lives in `data/`

### Repository Directory Structure

```
<repo-root>/
├── .backup-manager/
│   └── manifest.json   # devices + entries + links (git-tracked, source of truth)
├── data/               # the real content — the only content store
└── .git/               # Git repository
```

### Multi-Device

Devices, entries and links live in the repository manifest rather than in the local SQLite database — which is per machine — so a new machine discovers every device's links with a plain `git clone`. Register the machine, click **Apply** to recreate its symlinks, and add a link wherever the files should live — nothing needs to be promoted, since all links are equal.

### Link Equality and Consistency Rules

- An entry has **0..N links** and all of them are **completely equal** — same target, same semantics. There is no `in`/`out` type, no tracked link, and nothing to switch.
- An entry with **no links** is legal: the content is in the repository with no local view.
- Consistency is enforced: once a directory is tracked, no link may bind a single file inside it. Entries never overlap, and a link always binds a whole entry — never a sub-path.

The system refuses anything else at creation time and reports drift in the consistency audit.

## Features

- **Repo Management** — Create/delete/view backup repos, visual config (remote URL, branch, Git user)
- **Entry & Link Management** — Each backed-up file/directory is an entry with **0..N links, all equal**; view, distribute, add links, repair, re-adopt, and remove (unlink / move_back / purge)
- **Link State Diagnosis** — Per-link states (`ok` / `missing` / `wrong_target` / `replaced` / `dangling` / `occupied`) with one-click repair and re-adopt
- **Consistency Audit** — Verifies the invariants (**at most one `in` per entry** — an entry with none is reported as a warning, since that is legal during new-device initialisation, links bind whole entries, entries never overlap, no symlink inside `data/`) and reports unmanaged links
- **Multi-Device** — Machine fingerprint detection, device registration, dry-run `apply` to recreate a machine's links, detach, and automatic `in`-link promotion when a device is deleted
- **File Preview & Edit** — Plain text/code syntax highlighting, Markdown rendering, binary file identification; edits write straight into `data/` and every link reflects them immediately
- **Backup Execution** — Manual trigger or scheduled auto-backup (second-precision cron), optional Git push
- **Backup History** — View Git commit history with pagination, plus the uncommitted change count
- **Content Rollback** — Select a historical commit and restore `data/` to that version (full, per-entry, or single file)
- **File-Level Restore** — Preview and restore individual files from historical commits
- **Git Integration** — Remote repo config, SSH/HTTPS auth management (AES-256-GCM encrypted storage)
- **Local File Browsing** — Safely scoped to home directory and repo root, prevents path traversal
- **Scheduled Backups** — Cron-based auto-backup, auto-load on startup, dynamic register/unregister on config change
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
| Entries | `GET /repos/:id/entries?device=&state=` | List entries with their links and states |
| Entries | `GET /repos/:id/entries/:entryId` | Entry detail |
| Entries | `POST /repos/:id/entries/adopt` | Create an entry + its first link (moves content in) |
| Entries | `PATCH /repos/:id/entries/:entryId` | Rename `repo_path` (re-validates non-overlap) |
| Entries | `DELETE /repos/:id/entries/:entryId?mode=` | `unlink` / `move_back` / `purge` |
| Links | `GET/POST /repos/:id/entries/:entryId/links` | List / add a link |
| Links | `POST /repos/:id/links/bulk` | Bulk links for many entries at a local root |
| Links | `PATCH /repos/:id/entries/:entryId/links/:linkId` | Update `local_path` / `enabled` |
| Links | `POST .../links/:linkId/repair` | Recreate the symlink |
| Links | `POST .../links/:linkId/readopt` | `replaced` → move new content into `data/`, recreate the link |
| Links | `POST .../links/:linkId/remove` | Remove one link |
| Devices | `GET /devices/current` | This machine's fingerprint / hostname |
| Devices | `GET/POST/PATCH/DELETE /repos/:id/devices[/:fp]` | Device registration, rename, delete |
| Devices | `GET /repos/:id/devices/:fp/links` | This device's links with states |
| Devices | `POST /repos/:id/devices/:fp/apply` | Converge this machine (dry-run supported) |
| Devices | `POST /repos/:id/devices/:fp/detach` | Detach this machine |
| Consistency | `GET /repos/:id/consistency` | Audit findings |
| Consistency | `POST /repos/:id/consistency/repair` | Repair everything convergable |
| Content | `GET /repos/:id/tree?path=` | List entries under `data/` with badges |
| Content | `GET /repos/:id/preview?path=` | Preview file content |
| Content | `PUT /repos/:id/save` | Save to `data/` |
| Content | `GET /repos/:id/changes` | Uncommitted changes under `data/` (`git status`) |
| Browse | `GET /browse?path=...` | Browse local filesystem |
| Browse | `GET /browse/allowed-roots` | List allowed browsing roots |
| Backup | `POST /repos/:id/backup` | Trigger backup (optional `commit_message` body) |
| Backup | `GET /repos/:id/backup/history?limit=&offset=` | Backup history (paginated) |
| Backup | `POST /repos/:id/push` | Push to remote (optional `force` body) |
| Rollback | `GET /repos/:id/commits/:hash/changed-files` | List changed files in commit |
| Rollback | `GET /repos/:id/commits/:hash/files?path=` | Preview file content at commit |
| Rollback | `POST /repos/:id/commits/:hash/restore` | Restore single file from commit |
| Rollback | `POST /repos/:id/rollback` | Batch rollback `data/` to a historical version |
| Auth | `GET/PUT/DELETE /repos/:id/auth` | Git auth management |
| System | `GET /health` | Health check (status + uptime + version) |

## Workflow

```
1. Launch app → System tray icon appears in menu bar
2. Click tray icon → "Open UI" to open browser
3. Dashboard shows repo list
4. Click "Create Repo" → Enter name, select path
5. Enter repo detail → Entries tab → "+ New Entry" (the content moves into the repo; the original location becomes its first link)
6. Optionally add more links to distribute the same entry to further local paths
7. Browse, preview and edit content in the Browse tab
8. Switch to Backup tab → Click "Trigger Backup"
9. Configure remote repo and auth (optional)
10. Set up scheduled backup (optional)
11. Select a commit in backup history → Rollback (optional)
12. On another machine: clone the repo, open it, click "Apply" to recreate that machine's links
```

## Configuration

| Path | Description |
|------|------|
| `~/.config/backup-manager/config.json` | App config (port, theme, auto-open browser, etc.) — JSON keys: `port`, `open_browser`, `theme` |
| `~/.config/backup-manager/master.key` | AES-256 encryption key (auto-generated on first start) |
| `~/.config/backup-manager/backup-manager.db` | SQLite database (local, per machine) — holds `repos`, `repo_configs`, `repo_auths` |
| `<repo-root>/.backup-manager/manifest.json` | **Inside the repo**, tracked by Git — holds entries, links and devices. This is *not* the SQLite database; it lives in the repo so it travels with `git clone` / `git push` |

## Security Design

- **Path Safety**: Four-layer validation (Clean→Abs→EvalSymlinks→Prefix) prevents path traversal; local link paths are confined to the allowed roots (`$HOME` + repo roots), and self-reference into the repository is rejected
- **Link Consistency**: the manifest is validated before every write against R-1..R-3 (links bind whole entries; no overlapping entries; no link inside a directory entry); an unparsable file blocks writes, and an invalid one is reported by the audit instead of being silently rewritten
- **Atomic Manifest Writes**: `manifest.json.tmp` → `fsync` → `os.Rename`
- **Non-Destructive by Default**: linking refuses an occupied path; `apply` never overwrites; deleting content requires typed confirmation and is preceded by a commit so `git revert` restores it
- **Auth Encryption**: SSH private keys and HTTPS passwords encrypted with AES-256-GCM
- **Concurrency Control**: Per-repo mutex serializes backup, rollback and all filesystem-mutating link operations; preview API rate-limited (max 5 concurrent)
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
│   │       ├── entry.go        # Entry list / adopt / switch / delete
│   │       ├── link.go         # Link add / bulk / repair / remove
│   │       ├── device.go       # Device current / register / rename / delete / apply
│   │       ├── consistency.go  # Consistency audit + repair
│   │       ├── browse.go       # Local file browsing + allowed roots
│   │       ├── content.go      # Tree / preview / save / changes
│   │       ├── backup.go       # Backup trigger + history + push
│   │       ├── auth.go         # Git auth management
│   │       ├── rollback.go     # Content rollback + file restore
│   │       ├── system.go       # Health check
│   │       └── errors.go       # Error code mapping
│   ├── entry/                  # Entry & link subsystem
│   │   ├── manifest.go         # Manifest load / save / atomic write / R-1..R-3 validation
│   │   ├── service.go          # Service wiring, repo mutex, manifest commit, helpers
│   │   ├── entry_service.go    # adopt, list, remove (unlink / move_back / purge)
│   │   ├── link_service.go     # add out link, bulk link, switch, repair, remove
│   │   ├── device_service.go   # register, rename, delete, apply
│   │   ├── entry_state.go      # Per-link state diagnosis + views
│   │   └── consistency.go      # Consistency audit + repair
│   ├── service/                # Business logic layer
│   │   ├── repo_service.go     # Repo lifecycle
│   │   ├── backup_service.go   # Backup execution (git add/commit/push)
│   │   ├── auth_service.go     # Git auth management
│   │   ├── browser_service.go  # Safe file browsing
│   │   ├── content_service.go  # Content tree / preview / save
│   │   └── rollback_service.go # Rollback logic
│   ├── store/                  # Data persistence layer
│   │   ├── db.go               # SQLite init + migration
│   │   ├── store.go            # Store aggregation
│   │   ├── repo_store.go       # repos table operations
│   │   ├── repo_config_store.go# repo_configs table operations
│   │   └── repo_auth_store.go  # repo_auths table operations
│   ├── model/                  # Data models
│   │   ├── repo.go             # Repo, RepoConfig, RepoStatus
│   │   ├── link.go             # Entry, Link, Device, Manifest, LinkState
│   │   └── auth.go             # GitAuth, GitAuthType
│   ├── git/                    # Git engine
│   │   └── git.go              # Init/Add/Commit/Push/Log/Status/Config/LsTree/Show/WriteFileContentTo
│   ├── scheduler/              # Scheduled scheduler
│   │   └── scheduler.go        # Cron-based register/unregister
│   ├── servermgr/              # HTTP server lifecycle manager
│   ├── shortcut/               # Desktop shortcut creation
│   ├── tray/                   # System tray (menu bar) manager
│   └── util/                   # Utilities
│       ├── path.go             # SafeResolve four-layer path validation
│       ├── crypto.go           # KeyManager (AES-256-GCM)
│       ├── device.go           # MachineFingerprint()
│       ├── repo_mutex.go       # Per-repo mutex shared by backup / rollback / links
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
            ├── entry/
            │   ├── EntriesPanel.tsx        # Entries + links: list, designate tracked, repair, remove, apply
            │   └── AdoptModal.tsx          # Create an entry (content moves into the repo)
            ├── files/
            │   └── FilesPanel.tsx          # Browse the data/ tree + preview/edit
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
