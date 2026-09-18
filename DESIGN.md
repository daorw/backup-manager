# Technical Design — Backup Manager (Revised)

> 🇨🇳 [中文文档](docs/zh/DESIGN.md)
>
> Fixed 5 P0 review issues: repo config editing API, path traversal security, scheduled backup mechanism, Git auth configuration, incremental sync and data consistency conflicts.
> Second review "conditionally passed", 6 P1 issues fixed before entering development phase.

## 1. Technology Stack Selection

| Layer | Technology Component | Rationale |
|------|----------|----------|
| Language | Go 1.22+ | Cross-platform compilation, single binary, rich standard library |
| HTTP Framework | Gin | Lightweight, high performance, mature middleware ecosystem |
| Database | SQLite (modernc.org/sqlite) | No additional database service needed, single file storage, suitable for desktop-grade applications |
| Frontend | React 18 + TypeScript + Vite + Ant Design 5 | Mature ecosystem, rich component library |
| Scheduling | `robfig/cron/v3` | Standard cron library in Go ecosystem |
| Git Operations | `os/exec` calling system git | Reuses user's local git config |
| Encryption | `crypto/aes` + `crypto/gcm` | Symmetric encryption for storing sensitive info |

### Full-stack Integration Strategy

```
Production mode: Single binary, Go embed embeds frontend build artifacts, Go Server serves static files and API simultaneously
Dev mode: Vite Dev Server (5173) proxies /api/* to Go backend (9800)
```

## 2. Architecture Design

```
┌────────────────────────────────────────────────────────────┐
│                     Backup Manager (Single Process)         │
│  ┌──────────────┐  ┌─────────────────────────────────────┐ │
│  │  Frontend     │  │  Backend (Go)                       │ │
│  │  (embed.FS)   │  │  ┌────────┐ ┌─────────┐ ┌────────┐ │ │
│  │               │──│──│ Router │─│ Service │─│ Store  │ │ │
│  │  React SPA    │  │  │ (Gin)  │ │  Layer  │ │(SQLite)│ │ │
│  └──────────────┘  │  └────────┘ └─────────┘ └────────┘ │ │
│                    │       │            │                 │ │
│                    │  ┌────┴────┐ ┌────┴──────┐          │ │
│                    │  │  Path   │ │ Git Engine│          │ │
│                    │  │ Security│ │ (os/exec) │          │ │
│                    │  └─────────┘ └───────────┘          │ │
│                    │       │            │                 │ │
│                    │  ┌────┴────────────┴──────┐          │ │
│                    │  │ Scheduler (robfig/cron) │          │ │
│                    │  └─────────────────────────┘          │ │
└────────────────────────────────────────────────────────────┘
         │                          │
         ▼                          ▼
   ┌──────────────┐        ┌──────────────────────────┐
   │ Local Paths  │        │  Repository Root         │
   │  (views)     │◀───────│  ├─ .backup-manager/     │
   │  symlink ────┼───────▶│  │   └─ manifest.json    │
   └──────────────┘        │  ├─ data/   (real files) │
                           │  └─ .git/                │
                           └──────────────────────────┘
```

### Architecture Core Principles

| Principle | Description |
|------|----------|
| **Full-stack Integrated** | All code compiled into a single binary, frontend embedded via embed.FS |
| **Responsive API** | RESTful JSON API; repo-inherent state lives in SQLite, entry/link/device definitions live inside the repository (§9) |
| **Path Safety First** | All user-input paths must pass through SafeResolve security validation function |
| **Single Content Owner** | Content lives only in `data/`; a local path is a symlink view onto it — no mirror directory, no sync step (§9) |
| **Link Equality** | Links bind whole entries, never sub-paths; all links are equal — no in/out distinction (§9.3.3) |
| **Auth Isolation** | Git auth info stored encrypted, only injected as environment variables during git operations |

## 3. Detailed Design

### 3.1 Route Registration

```
POST   /api/v1/repos                          → RepoHandler.Create
GET    /api/v1/repos                          → RepoHandler.List
GET    /api/v1/repos/:id                      → RepoHandler.Get
DELETE /api/v1/repos/:id                      → RepoHandler.Delete
PUT    /api/v1/repos/:id/config               → RepoHandler.UpdateConfig  // ★ P0-1: Config Editing
POST   /api/v1/repos/:id/git-init             → RepoHandler.GitInit

POST   /api/v1/repos/:id/entries/adopt          → EntryHandler.Adopt          // creates the entry + its first link (mv local → data/)
GET    /api/v1/repos/:id/entries?device=&state= → EntryHandler.List
GET    /api/v1/repos/:id/entries/:entryId       → EntryHandler.Get
DELETE /api/v1/repos/:id/entries/:entryId       → EntryHandler.Delete

GET    /api/v1/repos/:id/entries/:entryId/links           → LinkHandler.List
POST   /api/v1/repos/:id/entries/:entryId/links           → LinkHandler.Create // adds a link
POST   /api/v1/repos/:id/links/bulk                       → LinkHandler.Bulk
PATCH  /api/v1/repos/:id/entries/:entryId/links/:linkId   → LinkHandler.Update
POST   /api/v1/repos/:id/entries/:entryId/links/:linkId/repair  → LinkHandler.Repair
POST   /api/v1/repos/:id/entries/:entryId/links/:linkId/readopt → LinkHandler.Readopt
POST   /api/v1/repos/:id/entries/:entryId/links/:linkId/remove  → LinkHandler.Remove

GET    /api/v1/devices/current                  → DeviceHandler.Current
GET    /api/v1/repos/:id/devices                → DeviceHandler.List
POST   /api/v1/repos/:id/devices                → DeviceHandler.Register
PATCH  /api/v1/repos/:id/devices/:fp            → DeviceHandler.Rename
DELETE /api/v1/repos/:id/devices/:fp            → DeviceHandler.Delete
GET    /api/v1/repos/:id/devices/:fp/links       → DeviceHandler.Links
POST   /api/v1/repos/:id/devices/:fp/apply       → DeviceHandler.Apply
POST   /api/v1/repos/:id/devices/:fp/detach      → DeviceHandler.Detach

GET    /api/v1/repos/:id/consistency             → ConsistencyHandler.Audit
POST   /api/v1/repos/:id/consistency/repair      → ConsistencyHandler.Repair

GET    /api/v1/browse         ?path=...         → BrowseHandler.Browse       // ★ P0-2: Security Fix
GET    /api/v1/browse/allowed-roots              → BrowseHandler.AllowedRoots

GET    /api/v1/repos/:id/tree    ?path=...        → ContentHandler.Tree
GET    /api/v1/repos/:id/preview ?path=...        → ContentHandler.Preview
PUT    /api/v1/repos/:id/save                     → ContentHandler.Save
GET    /api/v1/repos/:id/changes                  → ContentHandler.Changes

POST   /api/v1/repos/:id/backup                   → BackupHandler.Trigger
GET    /api/v1/repos/:id/backup/history?limit=&offset= → BackupHandler.History
POST   /api/v1/repos/:id/push                     → BackupHandler.Push

GET    /api/v1/repos/:id/auth                     → AuthHandler.Get
PUT    /api/v1/repos/:id/auth                     → AuthHandler.Set
DELETE /api/v1/repos/:id/auth                     → AuthHandler.Clear

GET    /api/v1/repos/:id/commits/:hash/changed-files  → RollbackHandler.ListFiles
GET    /api/v1/repos/:id/commits/:hash/files?path=    → RollbackHandler.GetCommitFile
POST   /api/v1/repos/:id/commits/:hash/restore        → RollbackHandler.RestoreFile
POST   /api/v1/repos/:id/rollback                     → RollbackHandler.Rollback

GET    /api/v1/health                                → SystemHandler.Health
```

### 3.2 Repo Config Editing (★ P0-1 Fix)

```go
// PUT /api/v1/repos/:id/config
// Accepts partial update, only submit fields to modify
type UpdateConfigRequest struct {
    RemoteURL          *string `json:"remote_url,omitempty"`
    Branch             *string `json:"branch,omitempty"`
    AutoBackup         *bool   `json:"auto_backup,omitempty"`
    AutoBackupInterval *string `json:"auto_backup_interval,omitempty"`
    GitUserName        *string `json:"git_user_name,omitempty"`
    GitUserEmail       *string `json:"git_user_email,omitempty"`
}
```

### 3.3 Path Security Validation (★ P0-2 Fix)

**Core Security Function SafeResolve:**

```go
func SafeResolve(allowedRoot, userPath string) (string, error) {
    // Step 1: filepath.Clean() eliminates ../ traversal
    cleaned := filepath.Clean(userPath)
    // Step 2: join relative path to allowedRoot
    if !filepath.IsAbs(cleaned) {
        cleaned = filepath.Join(allowedRoot, cleaned)
    }
    // Step 3: convert to absolute path
    absPath, _ := filepath.Abs(cleaned)
    // Step 4: resolve symlinks (prevent symlink escape)
    realPath, err := filepath.EvalSymlinks(absPath)
    if err != nil { realPath = absPath }
    // Step 5: verify it's within allowedRoot
    absRoot, _ := filepath.Abs(allowedRoot)
    if !strings.HasPrefix(realPath, absRoot + string(filepath.Separator)) && realPath != absRoot {
        return "", fmt.Errorf("path outside allowed root")
    }
    return realPath, nil
}
```

**Security Measures:**

| Measure | Description |
|------|----------|
| Clean → Abs → EvalSymlinks → Prefix | Four-layer path security validation |
| File size limit | Preview limited to ≤ 10MB |
| Binary detection | Read first 512 bytes to detect MIME type |
| Encoding detection | Non-UTF-8 encoded files return a prompt |
| Concurrency limit | Preview max 5 concurrent |

### 3.4 Entry & Link Model (supersedes the P0-5 mirror-consistency design)

The original P0-5 fix kept `.links/`, `data/`, and the source file consistent by copying. That duplication is removed by the entry/link model (§9): an **entry** owns the content, and every **link** is the same thing — a symlink to `data/<repo_path>`.

```
ADOPT (creates the entry + its first link):
  1. Validate local_path → SafeResolve
  2. Compute repo_path
  3. MOVE local_path → data/<repo_path>          (os.Rename, cross-fs degrade)
  4. os.Symlink(data/<repo_path> → local_path)
  5. Append {entry, link} to .backup-manager/manifest.json, commit

ADD LINK (distribute; identical filesystem effect):
  1. Validate that the entry exists and data/<repo_path> exists
  2. os.Symlink(data/<repo_path> → local_path)   (no content copied)
  3. Append the link to the entry, commit
```

There is no mirror directory and no copy step, so the backup operation reduces to: `git add -A → git commit → git push`.
See §9 for the full design, including the consistency invariants that prevent a link from binding a sub-path of a directory entry.

### 3.5 Scheduled Backup Scheduler (★ P0-3 Fix)

Implemented using `robfig/cron/v3`:

```go
type Scheduler struct {
    cron     *cron.Cron
    entries  map[string]cron.EntryID  // repoID → cron entryID
    backupFn BackupJobFunc
}

func (s *Scheduler) Start()
func (s *Scheduler) Stop()  // graceful shutdown, wait for tasks to complete
func (s *Scheduler) Register(repoID, cronExpr string) error
func (s *Scheduler) Unregister(repoID string)
func (s *Scheduler) IsRegistered(repoID string) bool
```

- On app startup, load all repos with auto_backup enabled from database and register them to scheduler
- Config updates automatically trigger Register/Unregister
- Backup task execution uses repo-level mutex lock to prevent concurrency

### 3.6 Git Auth Configuration (★ P0-4 Fix)

**Data Model:**

```go
type GitAuthType string
const (
    GitAuthNone     GitAuthType = "none"
    GitAuthSSHKey   GitAuthType = "ssh_key"
    GitAuthPassword GitAuthType = "password"
)

type GitAuth struct {
    RepoID           string
    AuthType         GitAuthType
    SSHPrivateKey    string  // AES-GCM encrypted storage
    SSHPrivateKeyPath string
    Username         string
    PasswordEncrypted []byte // AES-GCM encrypted storage
}
```

**Auth Injection Implementation:**
- SSH: via `GIT_SSH_COMMAND=ssh -i <key_path>` environment variable
- HTTPS: via `GIT_ASKPASS` script to inject password/Token
- Keys and passwords stored encrypted with AES-256-GCM in SQLite

## 4. Data Storage

There are **three separate stores**, each with its own file and its own reason to exist. They must not be confused with one another:

| Data | File | Format | Why there |
|------|------|------|------|
| `repos`, `repo_configs`, `repo_auths` | `~/.config/backup-manager/backup-manager.db` | SQLite (single binary file) | Machine-private: encrypted credentials, local repo paths, schedules. Must never be committed |
| entries, links, devices | `<repo-root>/.backup-manager/manifest.json` | JSON, tracked by Git | Must travel across machines with `git clone` / `git push`. The SQLite file is per machine and cannot |
| app settings | `~/.config/backup-manager/config.json` | JSON | Application-level settings, unrelated to any repo |

### 4.1 SQLite Database (local, per machine)

**File**: `~/.config/backup-manager/backup-manager.db` — a single SQLite binary file.

```sql
CREATE TABLE repos (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    path          TEXT NOT NULL UNIQUE,
    created_at    DATETIME DEFAULT (datetime('now')),
    updated_at    DATETIME DEFAULT (datetime('now')),
    last_backup_at DATETIME,
    status        TEXT DEFAULT 'active'
);

CREATE TABLE repo_configs (
    repo_id             TEXT PRIMARY KEY,
    remote_url          TEXT,
    branch              TEXT DEFAULT 'main',
    auto_backup         INTEGER DEFAULT 0,
    auto_backup_interval TEXT,
    git_user_name       TEXT,
    git_user_email      TEXT,
    FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE CASCADE
);

CREATE TABLE repo_auths (
    repo_id             TEXT PRIMARY KEY,
    auth_type           TEXT NOT NULL DEFAULT 'none',
    ssh_private_key     BLOB,
    ssh_private_key_path TEXT,
    username            TEXT,
    password_encrypted  BLOB,
    updated_at          DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE CASCADE
);
```

That is the complete schema — three tables. There is **no** table for entries, links or devices: the old `symlinks` table was dropped (§9.17) and nothing replaced it. Those definitions deliberately live outside SQLite, in §4.2.

### 4.2 Repository Manifest (inside the repo, tracked by Git)

**File**: `<repo-root>/.backup-manager/manifest.json` — a JSON file **inside the repository**, not in the SQLite database.

```json
{
  "version": 1,
  "updated_at": "2026-09-18T10:00:00Z",
  "devices": [ { "fingerprint": "9f2c…", "name": "MacBook Pro", "hostname": "mbp.local",
                 "os": "darwin", "last_seen_at": "2026-09-18T10:00:00Z" } ],
  "entries": [ { "id": "e1a2…", "repo_path": "opencode/opencode.json", "kind": "file",
                 "created_at": "2026-09-01T08:12:00Z",
                 "links": [ { "id": "l1a2…", "device": "9f2c…",
                              "local_path": "/Users/x/.config/opencode/opencode.json",
                              "enabled": true, "created_at": "2026-09-01T08:12:00Z" } ] } ]
}
```

Why entries/links/devices are **not** in SQLite — see §9.3.6:

- The SQLite database lives in `~/.config/backup-manager/` and is therefore **per machine**. A definition stored there cannot reach a second machine.
- The repository is the thing that gets cloned and pushed. Storing the definitions inside it makes the backup self-describing and lets a new machine discover every device's links with a plain `git clone`.
- It also gets versioning, diffing, merging and transport for free — no separate sync mechanism to design.

### 4.3 App Configuration

**File**: `~/.config/backup-manager/config.json`

```json
{
  "port": 9800,
  "open_browser": true,
  "theme": "light"
}
```

## 5. Project Directory Structure

```
backup-manager/
├── main.go
├── go.mod / go.sum
├── Makefile
├── REQUIREMENT.md
├── DESIGN.md
├── internal/
│   ├── api/
│   │   ├── router.go
│   │   ├── middleware.go
│   │   └── handler/
│   │       ├── repo.go
│   │       ├── entry.go         # list / adopt / delete an entry
│   │       ├── link.go          # add / bulk / repair / remove a link
│   │       ├── device.go        # current / register / rename / delete / apply
│   │       ├── consistency.go   # audit + repair
│   │       ├── browse.go
│   │       ├── content.go       # tree / preview / save / changes
│   │       ├── backup.go
│   │       ├── auth.go
│   │       ├── rollback.go
│   │       ├── system.go
│   │       └── errors.go
│   ├── model/
│   │   ├── repo.go
│   │   ├── link.go              # Entry, Link, Device, Manifest, LinkState
│   │   └── auth.go
│   ├── entry/                   # entry & link subsystem (§9)
│   │   ├── manifest.go          # load / save / atomic write / R-1..R-3 validation
│   │   ├── service.go           # Service wiring, repo mutex, manifest commit, helpers
│   │   ├── entry_service.go     # adopt, list, remove (unlink / move_back / purge)
│   │   ├── link_service.go      # add link, bulk link, repair, readopt, remove
│   │   ├── device_service.go    # register, rename, delete, apply
│   │   ├── entry_state.go       # per-link state diagnosis + view building
│   │   └── consistency.go       # audit + repair (§9.8)
│   ├── service/
│   │   ├── repo_service.go
│   │   ├── backup_service.go
│   │   ├── auth_service.go
│   │   ├── browser_service.go
│   │   ├── content_service.go
│   │   └── rollback_service.go
│   ├── store/
│   │   ├── db.go
│   │   ├── store.go
│   │   ├── repo_store.go
│   │   ├── repo_config_store.go
│   │   └── repo_auth_store.go
│   ├── git/
│   │   └── git.go
│   ├── scheduler/
│   │   └── scheduler.go
│   ├── servermgr/
│   ├── shortcut/
│   ├── tray/
│   └── util/
│       ├── path.go          # SafeResolve security function
│       ├── crypto.go        # AES-GCM encryption
│       ├── device.go        # MachineFingerprint()
│       ├── repo_mutex.go    # per-repo mutex shared by backup / rollback / links
│       └── file.go          # File operation utilities
├── frontend/
│   ├── package.json
│   ├── vite.config.ts
│   ├── index.html
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       ├── routes/
│       ├── components/
│       ├── api/client.ts
│       ├── store/
│       ├── types/
│       └── utils/
└── scripts/build.sh
```

## 6. Review Issue Fix Cross Reference

| Review Issue | Fix Plan |
|----------|----------|
| **P0-1** Missing repo config editing API | Added `PUT /api/v1/repos/:id/config` + SQLite persistent config |
| **P0-2** High-risk path traversal | SafeResolve four-layer protection (Clean→Abs→EvalSymlinks→Prefix) + file size / concurrency limits |
| **P0-3** Missing scheduled backup | `robfig/cron/v3` scheduler + lifecycle management + config control |
| **P0-4** Missing Git auth config | SSH/HTTPS auth + AES-GCM encrypted storage + environment variable injection |
| **P0-5** Incremental sync and consistency conflict | Sync copy to data/ when adding symlink, backup reduces to git add+commit+push |
| **P1-1** Browse API security boundary ambiguous | Removed `root` parameter, introduced AllowedRoots mechanism (only $HOME and repo root directory are browsable) |
| **P1-2** Missing AES-GCM key management strategy | Generate AES-256 key on first startup stored at `~/.config/backup-manager/master.key` (0600 permission), managed as `[]byte` in memory + zeroed after use |
| **P1-3** data/ sync strategy when source file is externally deleted not clear | Incremental backup detection phase supplement: auto-delete data/ and symlink when source file does not exist |
| **P1-4** Incremental backup detection algorithm not defined | Explicitly defined as mtime + fileSize dual field comparison |
| **P1-5** SafeResolve EvalSymlinks fallback security blind spot | Distinguish error types: fs.ErrNotExist can be downgraded, other errors rejected |
| **P1-6** Missing error handling and user notification plan | Added complete error handling section: error classification, SSE notification, crash recovery, rollback mechanism |

## 7. Preview & Edit

The mount model (§9) removes the source-vs-copy duality: content lives in `<repo>/data/`, and a local path is only a symlink into it. Preview and edit therefore operate on a single file — `data/<repo_path>` — with no dual write and no sync step.

| Operation | Target | Note |
|------|------|------|
| Preview | `<repo>/data/<repo_path>` | `repo_path` is the node path in the Browse tree |
| Save | `<repo>/data/<repo_path>` | Written in place; the local mount reflects it immediately, because it is a symlink to the same inode |
| Backup | `data/` | `git add -A` picks the edit up — no incremental sync involved |

### 7.1 API Contract

```
GET  /api/v1/repos/:id/tree?path=          → list entries under data/<path>, each with mount badges
GET  /api/v1/repos/:id/preview?path=       → {content, mime_type, size, text, truncated}
PUT  /api/v1/repos/:id/save                → {path, content} → {file_size, modified_at}
GET  /api/v1/repos/:id/changes             → {dirty, changes:[{status, path}]}  (git status --porcelain data/)
```

`path` is always relative to `data/` (never an absolute local path), which removes the old `ResolveSource` prefix-matching logic entirely.

### 7.2 Constraints

| Condition | Handling |
|------|------|
| `path` escapes `data/` | `util.SafeJoin` rejects → 400 |
| Content > 10 MB | 413 |
| `path` is a directory | 400 |
| `path` does not exist | 404 |
| Concurrency | repo-level mutex for save; preview limited to 5 concurrent |
| Permission preservation | read the original mode, write, then `Chmod` back |

### 7.3 Security

| Risk | Protection |
|------|------|
| Path traversal | `util.SafeJoin(dataDir, path)` on every request |
| Binary content | Frontend disables editing when `truncated` or `text == false`; backend enforces the size limit |
| OOM | Content length ≤ 10 MB |

## 8. System Tray Design

### 8.1 Overview

The application runs with a system tray (menu bar on macOS) icon, providing always-available controls:

- **Open UI**: Opens the web UI in the default browser
- **Start/Stop Server**: Toggles the HTTP server on/off independently from the tray process
- **Quit**: Exits the application completely

### 8.2 Implementation

- `internal/tray/` — System tray manager (macOS menu bar / system tray icon)
- `internal/servermgr/` — HTTP server lifecycle manager (start/stop without exiting the process)
- `internal/shortcut/` — Desktop shortcut creation on first run

### 8.3 Lifecycle

```
main()
  ├── Initialize all services (DB, git, scheduler, handlers)
  ├── Create servermgr (HTTP server manager)
  ├── Create tray manager (with callbacks for OpenUI/StartServer/StopServer/Quit)
  ├── srvMgr.Start() → HTTP server begins listening
  ├── trayMgr.SetServerRunning(true)
  ├── openURL(serverURL) → opens browser (if open_browser: true)
  ├── trayMgr.Run() → blocks until user clicks "Quit"
  ├── srvMgr.Stop() → gracefully stops HTTP server
  └── Final cleanup (DB close, key manager destroy)
```

## 9. Entry & Link Model — Clean-Slate Redesign

> Implements Issue #4 (bidirectional symlink management + multi-device support).
> **Supersedes §3.4 and the whole `/symlinks` API.** No backward compatibility with the old `symlinks` table, the `.links/` directory, or the copy-based backup flow — see §9.17.

### 9.1 Why the Old Model Was Complex

The previous design kept **three representations of the same file**: the source file, the `data/` copy, and the `.links/` symlink. Keeping them consistent required an invasive amount of machinery:

| Machinery | Existed because |
|------|------|
| Incremental detection (mtime + size) | source and `data/` can drift |
| `syncOneFile` / `syncDirectoryFiles` / `walkSourceDir` | re-copying the source into `data/` |
| Mirror consistency in `.links/` | a third representation to keep in sync |
| `SyncDeletedSource` | guessing whether a missing path meant "deleted source" or "deleted link" (a data-loss bug) |
| `is_new` comparison | source ≠ `data/` |
| Preview dual write (source **and** `data/`) | two writable copies |
| `ResolveSource` longest-prefix matching | mapping a repo path back to a local path |
| Nested-symlink special case | symlinks inside a source tree |

The root cause is the duplication. This redesign removes it:

> **Content lives in the repository. Local paths are only views onto it.**

### 9.2 Design Principles

| # | Principle | Consequence |
|------|------|------|
| **P-1** | Content has exactly one owner: `data/<repo_path>` | No drift, no sync, no comparison |
| **P-2** | A local path is a **symlink** into `data/` | Writing through it writes the repository copy |
| **P-3** | **All links are equal** | One link mechanism; no `in`/`out` distinction to define, validate or switch |
| **P-4** | Links bind **whole entries**, never sub-paths | An entry and its links stay perfectly consistent |
| **P-5** | Definitions live in the repository, tracked by Git | `<repo>/.backup-manager/manifest.json` is the single source of truth |
| **P-6** | Git is the safety net | Every destructive operation is preceded by a commit |
| **P-7** | Idempotent convergence, not incremental bookkeeping | `apply` diffs desired vs. actual state and converges |

### 9.3 Core Model

Two entities: **Entry** (the backed-up file/directory) and **Link** (a local view of it).

#### 9.3.1 Entry

```
Entry:  repo_path = opencode/opencode.json   kind = file
        content   = <repo>/data/opencode/opencode.json      ← the one and only copy
```

An **entry** is the unit of backup: one `repo_path` under `data/` plus every link that points at it. An entry *is* the whitelist member — as long as it exists in the manifest it is a backed-up object. It may have **zero links**: the content is in the repository, there is simply no local view of it right now.

| Field | Description |
|------|------|
| `id` | stable short id, used by the API |
| `repo_path` | path relative to `data/` — the global identity of the content |
| `kind` | `file` \| `dir` — cached content kind |
| `created_at` | |
| `links` | the ordered list of links bound to this entry |

#### 9.3.2 Link

```
   link.local_path                          entry.repo_path
   /Users/x/.config/opencode/o.json   ⟷   opencode/opencode.json
                  │
                  └── symlink ──▶  <repo>/data/opencode/opencode.json
```

A **link** binds one local path to one entry. Its filesystem form is *always* the same: `local_path` is a symlink to `<repo>/data/<entry.repo_path>`. All links of an entry are **completely equivalent** — no type, no primary, no tracked link.

| Field | Description |
|------|------|
| `id` | stable short id |
| `device` | fingerprint of the owning device |
| `local_path` | absolute local path of the symlink |
| `enabled` | disabled links are kept but skipped by `apply` |
| `created_at` | |

#### 9.3.3 All Links Are Equal

Every link is a symlink to the same `data/<repo_path>`, so all of the following are the same operation on the same content:

- editing through the link that `adopt` created together with the entry
- editing through a link added later to distribute the entry elsewhere
- editing through the Browse tab

The model therefore does **not** distinguish "the link that created the entry" (the Issue's *inbound* direction) from "links added later" (the Issue's *outbound* direction). Inbound/outbound survives as a description of **how a link came to exist**, not as stored state:

| | How it comes to exist | Stored difference |
|------|------|------|
| First link | created by `adopt`, together with the entry, after moving the content into `data/` | none |
| Later links | created by `add link`, pointing at content that already exists | none |

Consequences:

- **One implementation.** `createLink(entry, localPath)` serves both; `adopt` is "add link" preceded by the `mv` of the content.
- **Nothing to switch.** There is no "tracked" link to hand over between machines — each machine simply has its own links (§9.5).
- **An entry with no links is legal.** It is how content arriving from a clone or an import can exist in the repository before any local path is bound to it.

#### 9.3.4 Consistency Invariants

Issue #4 requires that links stay **completely consistent** with the backed-up file/directory: a link tracking a directory must not be accompanied by a link to a single file inside it. That rule is generalized into three invariants:

| # | Invariant | Enforced by |
|------|------|------|
| **R-1** | Every link of an entry binds **that entry's full `repo_path`** — never a path inside it | Structural: links can only be attached to an entry; the API does not accept sub-paths |
| **R-2** | Entries **never overlap**: no `repo_path` is an ancestor or descendant of another's | `adopt` validation → 409 |
| **R-3** | A link's `local_path` must not lie **inside** a directory entry's `local_path` | Link creation validation → 409 |

R-2 is the generalization of the Issue's rule: if a directory is tracked as `~/Documents`, nothing inside it may be separately tracked or separately linked. Choose a non-overlapping `repo_path` instead (e.g. track `~/Projects/vendor` as `projects/vendor`, not as `docs/vendor`).

R-3 is the literal rule: with `docs` adopted as a directory entry at `~/Documents`, a link at `~/Desktop/notes/file.md` is refused.

That all links of an entry point at the same target is now a property of the constructor rather than an invariant to validate — and since links carry no type, there is no cardinality rule to enforce either.

#### 9.3.5 Device

A **device** is one machine that references the repository. It is metadata; links reference it by fingerprint.

| Field | Description |
|------|------|
| `fingerprint` | stable machine id, `sha256("<GOOS>|<raw>")` hex — the device's primary key |
| `name` | user-visible name, defaults to the hostname, renameable |
| `hostname`, `os` | display metadata only |
| `last_seen_at` | updated on every `apply` |

Fingerprint sources, in order:

| OS | Source | Fallback |
|------|------|------|
| Linux | `/etc/machine-id`, then `/var/lib/dbus/machine-id` | — |
| macOS | `ioreg -rd1 -c IOPlatformExpertDevice` → `IOPlatformUUID` | — |
| Windows | `reg query HKLM\SOFTWARE\Microsoft\Cryptography /v MachineGuid` | — |
| any | — | `sha256(hostname + "|" + username)` |

Only the hash is stored. It is cached at `~/.config/backup-manager/device.json`.

#### 9.3.6 Repository Layout and Manifest

```
<repo-root>/
├── .backup-manager/
│   └── manifest.json      # devices + entries + links (git-tracked, source of truth)
├── data/                  # real content, git-tracked — the only content store
└── .git/
```

The manifest is grouped **by entry**, which is what makes R-2 and R-3 structurally evident — every link of an entry lives next to that entry, so a sub-path link cannot even be expressed.

```json
{
  "version": 1,
  "updated_at": "2026-09-18T10:00:00Z",
  "devices": [
    { "fingerprint": "9f2c1a4b7d8e0f31", "name": "MacBook Pro",
      "hostname": "mbp.local", "os": "darwin",
      "last_seen_at": "2026-09-18T10:00:00Z" }
  ],
  "entries": [
    {
      "id": "e1a2b3c4d5e6f708",
      "repo_path": "opencode/opencode.json",
      "kind": "file",
      "created_at": "2026-09-01T08:12:00Z",
      "links": [
        { "id": "l1a2b3c4d5e6f708",
          "device": "9f2c1a4b7d8e0f31",
          "local_path": "/Users/x/.config/opencode/opencode.json",
          "enabled": true, "created_at": "2026-09-01T08:12:00Z" },
        { "id": "l2b3c4d5e6f70819",
          "device": "9f2c1a4b7d8e0f31",
          "local_path": "/Users/x/Desktop/opencode.json",
          "enabled": true, "created_at": "2026-09-10T12:00:00Z" }
      ]
    },
    {
      "id": "e2b3c4d5e6f70819",
      "repo_path": "docs/notes",
      "kind": "dir",
      "created_at": "2026-09-02T09:00:00Z",
      "links": [
        { "id": "l3c4d5e6f7081920",
          "device": "9f2c1a4b7d8e0f31",
          "local_path": "/Users/x/Documents/notes",
          "enabled": true, "created_at": "2026-09-02T09:00:00Z" }
      ]
    }
  ]
}
```

| Rule | Detail |
|------|------|
| Source of truth | The manifest is authoritative. SQLite holds only `repos`, `repo_configs`, `repo_auths` — things inherently local to one machine |
| Write | Every change rewrites the file and commits it immediately (`link: add opencode/opencode.json`). Writes are atomic: `manifest.json.tmp` → `fsync` → `os.Rename` |
| Read | Loaded on repo open, cached in memory keyed by `repoID` + file mtime |
| Transport | Living inside the repo, it travels with `git clone` / `git push`. A new machine learns every device's links at once |
| Hand editing | Supported. Missing `id`s are assigned on load; unknown fields are preserved; an unparsable file blocks all writes instead of being silently rewritten |
| Conflict | Git is the conflict resolver. A merge conflict surfaces as a normal Git conflict on this file; the app refuses to write until it is resolved |

### 9.4 Creation Flows

#### 9.4.1 Adopt — creates an entry and its first link

```
1. Validate local_path: exists, is NOT a symlink, parent writable, NOT inside repo.Path
2. repo_path = user input, default filepath.Base(local_path)
3. R-2 check: repo_path must not be an ancestor or descendant of any existing entry → 409
4. data/<repo_path> must not already exist → 409
5. If local_path is a directory, scan it for symlinks.
     Any symlink found → refuse (it would drag content in from outside the repo and
     break "content lives only in data/"), listing the offending paths.
     Option `follow_symlinks: true` dereferences them instead.
6. MOVE local_path → <repo>/data/<repo_path>
     - try os.Rename; on EXDEV degrade to CopyFile + verify size + Remove
7. Create the link: local_path → <repo>/data/<repo_path>
8. Append {entry, link} to the manifest, commit
```

Failure rollback:

| Failure point | Compensation |
|------|------|
| 6 fails | nothing moved — return the error |
| 7 fails | move `data/<repo_path>` back to `local_path` |
| 8 fails | remove the symlink, move `data/<repo_path>` back to `local_path` |

#### 9.4.2 Add Link — an additional view of an existing entry

```
1. Resolve the entry; it must exist. Having a link already is NOT required — this
   flow is how a freshly initialised device binds an entry that has none
2. Validate local_path: does not exist, or is an empty directory
3. R-3 check: local_path must not be inside any directory entry's local_path → 409
4. local_path must not be inside repo.Path
5. Ensure the parent directory of local_path exists
6. Create the symlink: local_path → <repo>/data/<entry.repo_path>
7. Append the link to the entry, commit
```

The same `repo_path` may carry many links, on the same device or on different ones — that is the Issue's "distribute one backup to many locations".

#### 9.4.3 Bulk Link — bringing a repository onto a machine

```
POST /api/v1/repos/:id/links/bulk  { local_root, entry_ids? }
```

Select one or more entries (or all) plus a local root directory; the service creates one link per entry at `<local_root>/<repo_path>`. The R-3 check applies to `local_root`. This is the one-click "I just cloned this backup repo onto a new machine, give me my files back" operation.

#### 9.4.4 Forbidden Shapes

| Shape | Why forbidden | Error |
|------|------|------|
| A link to a single file inside a tracked directory | Breaks the Issue's complete-consistency rule (R-1, R-3) | 409 |
| A nested entry whose `repo_path` is inside another entry's | Two owners for one subtree (R-2); this replaces the old `AddNestedSymlink` feature | 409 |
| A link whose `local_path` is inside a directory entry's `local_path` | Ambiguous ownership (R-3) | 409 |

> An entry **without** any links is not a forbidden shape — it is how content arriving from a clone or an import exists in the repository before any local path is bound to it.

> Note the deliberate removal: the old `AddNestedSymlink` let a directory symlink contain links to unrelated locations. Under R-2 that is expressed by adopting the sub-content as its **own non-overlapping entry** instead (e.g. `projects/vendor` rather than `docs/vendor`).

### 9.5 Link Equality

Because links carry no type, there is no "tracked" link and nothing to hand over when a machine is replaced. Everything that used to hang off the tracked link is now either derived or explicit:

| Question | Answer |
|------|------|
| Which local path does the UI open for an entry? | The first link **on the current device** (`is_current`); if the entry has none here, it is shown as having no local view |
| Where does `move_back` put the content? | An explicitly chosen link (`link_id`); if omitted, the first link on the current device, then the first link — both deterministic |
| Which link is the entry's "home"? | There is no such concept. Each machine has its own links; the entry is identified by `repo_path` |

#### 9.5.1 Device handover

```
Device A is retired:
  1. On device B, add a link wherever the content should live
     POST /entries/:id/links { local_path: ~/dev/opencode/opencode.json }
  2. Delete device A (or detach it)
     → A's link is removed from the manifest and its symlink is removed from A
  The entry, its content in data/, and every other link are untouched.
```

Nothing needs to be "promoted": on device B the new link is already the only local view, and the UI picks it up as soon as it exists.

### 9.6 Device Lifecycle

#### 9.6.1 Register

```
1. Compute the current machine fingerprint
2. Open the repo → parse the manifest
3. If a device with this fingerprint exists → update last_seen_at
   else → append a new device (name = hostname), commit
4. Set it as the current device (in-memory session state; not persisted in the manifest)
```

Registration is implicit: it happens on repo open, on `apply`, and on any link creation. The UI never asks the user to "create a device" as a first step.

#### 9.6.2 Apply — converging the machine

```
POST /api/v1/repos/:id/devices/:fingerprint/apply   { dry_run: true }

1. Collect every link whose device is this fingerprint
2. Diagnose each one against the filesystem (§9.7)
3. Build a plan:
     create  — missing      → create the symlink
     repair  — wrong_target → recreate the symlink
     skip    — ok / disabled / replaced
     conflict— occupied     → report, never overwrite
     orphan  — dangling     → report only (the entry content is missing from the repo)
4. Return the plan (dry run) → the UI shows it in a confirmation dialog
5. On confirmation, execute under the repo-level mutex and report per-item results
```

`apply` is idempotent and never touches content: it only creates, fixes, or reports symlinks.

#### 9.6.3 Detach / Handover

```
POST /api/v1/repos/:id/devices/:fingerprint/detach  { mode: "unlink" | "keep" }
```

| Mode | Behavior |
|------|------|
| `unlink` | Remove every local symlink of the device. `data/` is untouched, so all content is preserved |
| `keep` | Leave the filesystem alone and just stop managing (used when handing a machine over) |

Detach does **not** delete the device entry, and it leaves the link definitions enabled — that is what makes re-attaching a single `Apply`. The consequence is that after an `unlink` detach those links read as `link_missing` in the audit until the machine is re-attached.

Deleting a device (`DELETE /devices/:fingerprint`) removes its link definitions. Entries keep existing — an entry left with no links is perfectly legal, its content stays in the repository. A device's filesystem is never touched by its deletion. The current device cannot be deleted.

### 9.7 Link State & Repair

State is computed on demand from the filesystem; it is never persisted.

| State | Condition | Offered action |
|------|------|------|
| `ok` | `local_path` is a symlink whose resolved target is `data/<entry.repo_path>` | — |
| `missing` | `Lstat(local_path)` returns `ErrNotExist` | Create |
| `wrong_target` | Symlink exists but points elsewhere | Repair |
| `replaced` | `local_path` exists and is a **real** file/directory | Re-adopt (move the content into `data/` and recreate the link) or Remove the link |
| `dangling` | Symlink exists but `data/<repo_path>` is missing from the repository | Roll back from Git history, or Remove |
| `occupied` | Path is taken by an unrelated object and cannot be safely replaced | Manual resolution |
| `disabled` | `enabled == false` | — |
| `not_current` | Belongs to another device | Read-only display |

> `replaced` deserves emphasis: applications that write configuration atomically (temp file + `rename`) replace the symlink with a real file. The UI surfaces this as `replaced` with a one-click **Re-adopt** that moves the new content into `data/` and restores the link. Guidance: prefer tracking **directories** rather than individual files when an application manages the file itself.

> `dangling` is safe by construction: because content lives in Git, `git checkout <commit> -- data/<repo_path>` restores it. The UI links directly to the rollback view.

Note that link state is per link, and an entry with no links has no per-link state at all — which is not a problem, it simply has no local view.

### 9.8 Consistency Audit

```
GET /api/v1/repos/:id/consistency            # entry-level + link-level findings
POST /api/v1/repos/:id/consistency/repair     # converge everything that can be converged
```

| Check | Finding code | Severity |
|------|------|------|
| An entry lacks `id` / `repo_path`, has a duplicate id, or an illegal `repo_path` | `invalid_entry` | error |
| A link lacks `local_path` / `device`, or has a duplicate id | `invalid_link` | error |
| A link references a device that is not registered | `unknown_device` | error |
| Two entries overlap (hand-edited manifest) | `overlapping_entries` | error |
| A link's `local_path` sits inside a directory entry's `local_path` | `nested_link` | error |
| A local symlink is missing, points elsewhere, or was replaced by a real file | `link_missing` / `link_wrong_target` / `link_replaced` (the §9.7 state) | warning |
| A local path is taken by an unrelated object | `link_occupied` | error |
| The content of a live entry is missing from `data/` | `content_missing` | error |
| A symlink exists **inside** `data/` | `symlink_in_data` | error — breaks "content lives only in `data/`" |
| An unmanaged symlink pointing into `data/` is found while scanning the parent directories of registered links | `unmanaged_link` | warning |

`unmanaged_link` is the direct detector for the Issue's forbidden shape: a link to a sub-path that was created outside the application. It cannot scan the whole filesystem, so it is scoped to the parent directories of registered links and reported as a warning rather than an error.

#### 9.8.1 What repair can and cannot fix

| Finding | Repair action |
|------|------|
| `nested_link` | Disable the offending link — a disabled link is not active, so it no longer violates R-3 |
| `link_missing` / `link_wrong_target` | Recreate the local symlink |
| `link_replaced` | Report only — recovering the content requires an explicit re-adopt decision |
| `link_occupied`, `content_missing`, `symlink_in_data`, `overlapping_entries`, `invalid_entry`, `invalid_link`, `unknown_device`, `unmanaged_link` | Report only — there is no safe automatic action |

#### 9.8.2 Where validation happens

| Stage | Behaviour | Why |
|------|------|------|
| `Load` | Parses and assigns missing ids; does **not** validate | A hand-edited manifest must stay readable. If loading failed, the repository would become completely unusable and the user could not even see what is wrong |
| `Save` | Validates; refuses any write that would introduce an error-level violation | The application never persists an invalid state |
| `saveConverging` | Skips validation | Used only by repair and removal, which can only reduce the number of violations. Without it, a manifest that was hand-edited into an invalid state could never be corrected through the app |

Note that R-3 is evaluated over **enabled** links only. That is what makes "disable the offending link" a legal convergence step rather than another violation.

### 9.9 Removing

**Link level** — always safe:

```
POST /api/v1/repos/:id/entries/:entryId/links/:linkId/remove
```

Removes the local symlink and the link record. `data/` is untouched, so every other link of the entry keeps working, and removing the last link leaves a perfectly valid entry with no local view. The entry-level operations below are for the cases where the content itself should no longer be tracked.

**Entry level:**

```
DELETE /api/v1/repos/:id/entries/:entryId?mode=...
```

| Mode | Behavior | Guard |
|------|------|------|
| `unlink` | Remove only this device's local symlinks; the entry and its content stay | none — safe |
| `move_back` | Move `data/<repo_path>` to the `local_path` of a chosen link, then remove the whole entry | Warn that the other links will dangle; requires confirmation |
| `purge` | Delete `data/<repo_path>` **and** the whole entry | Requires typing the `repo_path` to confirm; warns that other devices' links will dangle and will be reported by their next `apply` |

Every removal is preceded by a commit, so `git revert` is always available — **Git is the recycle bin**. There is no separate trash mechanism to design.

### 9.10 Service & Store Layout

```
internal/
├── entry/                        # new package — the whole subsystem
│   ├── manifest.go               # load / save / atomic write / R-1..R-3 validation
│   ├── service.go                # Service wiring, repo mutex, manifest commit, helpers
│   ├── entry_service.go          # adopt, list, readopt, remove (unlink / move_back / purge)
│   ├── link_service.go           # add link, bulk link, repair, remove
│   ├── device_service.go         # register, rename, delete, apply
│   ├── entry_state.go            # per-link state diagnosis + views (§9.7)
│   ├── consistency.go            # audit + repair (§9.8)
│   └── entry_service_test.go
├── model/
│   ├── repo.go                   # unchanged
│   ├── auth.go                   # unchanged
│   └── link.go                   # new — Entry, Link, Device, Manifest, LinkState
└── util/
    ├── device.go                 # new — MachineFingerprint()
    └── repo_mutex.go             # moved from service — shared by backup/rollback/links
```

SQLite tables after the redesign — all of them live in `~/.config/backup-manager/backup-manager.db`; the entry/link/device definitions live in the repo manifest instead (§4.2):

```sql
repos         — id, name, path, created_at, updated_at, last_backup_at, status   (unchanged)
repo_configs  — repo_id(FK), remote_url, branch, auto_backup, ...                (unchanged)
repo_auths    — repo_id(FK), auth_type, ssh_private_key, ...                      (unchanged)
-- symlinks table: DROPPED
```

### 9.11 API

**Entries**

| Method | Path | Purpose |
|------|------|------|
| GET | `/api/v1/repos/:id/entries` | Entry list; every link carries its computed state, so device/state grouping is done client-side (server-side filters deferred, §9.19) |
| GET | `/api/v1/repos/:id/entries/:entryId` | Entry detail (includes its links and their states) |
| POST | `/api/v1/repos/:id/entries/adopt` | Create an entry together with its first link: `{local_path, repo_path?, follow_symlinks?}` |
| DELETE | `/api/v1/repos/:id/entries/:entryId?mode=&link_id=` | `unlink` (with an optional `link_id` to remove a single link) / `move_back` / `purge` |
| PATCH | `/api/v1/repos/:id/entries/:entryId` | `{repo_path?}` — rename; R-2 re-validated (deferred, §9.19) |

**Links**

| Method | Path | Purpose |
|------|------|------|
| POST | `/api/v1/repos/:id/entries/:entryId/links` | Add a link: `{local_path, device?}` |
| POST | `/api/v1/repos/:id/links/bulk` | Bulk links: `{local_root, entry_ids?}` |
| POST | `/api/v1/repos/:id/entries/:entryId/links/:linkId/repair` | Recreate the symlink |
| POST | `/api/v1/repos/:id/entries/:entryId/links/:linkId/readopt` | `replaced` → move the new content into `data/`, recreate the link |
| POST | `/api/v1/repos/:id/entries/:entryId/links/:linkId/remove` | Remove one link |
| GET | `/api/v1/repos/:id/entries/:entryId/links` | Links of an entry with states — already returned by the entry detail (deferred, §9.19) |
| PATCH | `/api/v1/repos/:id/entries/:entryId/links/:linkId` | `{local_path?, enabled?}` (deferred, §9.19) |

**Devices**

| Method | Path | Purpose |
|------|------|------|
| GET | `/api/v1/devices/current` | Fingerprint / hostname / suggested name of the running machine |
| GET | `/api/v1/repos/:id/devices` | Device list (`fingerprint`, name, `last_seen_at`, link count, `is_current`) |
| POST | `/api/v1/repos/:id/devices` | Register / claim a device (`name`, optional `fingerprint`) |
| PATCH | `/api/v1/repos/:id/devices/:fingerprint` | Rename a device |
| DELETE | `/api/v1/repos/:id/devices/:fingerprint` | Delete a device and its link definitions |
| POST | `/api/v1/repos/:id/devices/:fingerprint/apply` | Converge the machine (`dry_run` supported) |
| POST | `/api/v1/repos/:id/devices/:fingerprint/detach` | Detach the machine (`mode`: `unlink` / `keep`) |
| GET | `/api/v1/repos/:id/devices/:fingerprint/links` | This device's links with states — derivable from the entry list (deferred, §9.19) |

**Consistency**

| Method | Path | Purpose |
|------|------|------|
| GET | `/api/v1/repos/:id/consistency` | Audit findings (§9.8) |
| POST | `/api/v1/repos/:id/consistency/repair` | Converge everything convergable |

**Content** (unchanged shape, simplified semantics — see §7)

| Method | Path | Purpose |
|------|------|------|
| GET | `/api/v1/repos/:id/tree?path=` | Entries under `data/`, each with an entry badge and link count |
| GET | `/api/v1/repos/:id/preview?path=` | File preview |
| PUT | `/api/v1/repos/:id/save` | Save to `data/` |
| GET | `/api/v1/repos/:id/changes` | `git status --porcelain data/` |

Payloads:

```go
type EntryView struct {
    ID        string      `json:"id"`
    RepoPath  string      `json:"repo_path"`
    Kind      string      `json:"kind"` // file | dir
    CreatedAt time.Time   `json:"created_at"`
    Links     []*LinkView `json:"links"`
}

type LinkView struct {
    ID         string    `json:"id"`
    EntryID    string    `json:"entry_id"`
    Device     string    `json:"device"` // device fingerprint
    DeviceName string    `json:"device_name,omitempty"`
    LocalPath  string    `json:"local_path"`
    Enabled    bool      `json:"enabled"`
    IsCurrent  bool      `json:"is_current"` // derived: device == current fingerprint
    State      string    `json:"state"`
    StateNote  string    `json:"state_note,omitempty"`
    CreatedAt  time.Time `json:"created_at"`
}

type ApplyRequest struct {
    DryRun bool `json:"dry_run"`
}

type ApplyAction struct {
    EntryID   string `json:"entry_id"`
    LinkID    string `json:"link_id"`
    RepoPath  string `json:"repo_path"`
    LocalPath string `json:"local_path"`
    Action    string `json:"action"` // create / repair / skip / conflict / orphan
    Reason    string `json:"reason,omitempty"`
}

type ApplyResult struct {
    Device      string        `json:"device"`
    Created     []ApplyAction `json:"created"`
    Repaired    []ApplyAction `json:"repaired"`
    Skipped     []ApplyAction `json:"skipped"`
    Conflicts   []ApplyAction `json:"conflicts"`
    Orphans     []ApplyAction `json:"orphans"`
    DryRun      bool          `json:"dry_run"`
    CompletedAt time.Time     `json:"completed_at"`
}
```

### 9.12 Frontend

Repository detail tabs: **Browse** · **Entries** · **Backup** · **Config**.

```
components/entry/
├── EntriesPanel.tsx          # tab root: toolbar (device, New Entry, Apply, Audit, Detach),
│                             #   entry list with expandable link rows, add-link / apply-plan /
│                             #   audit / detach / remove-entry modals
└── AdoptModal.tsx            # create an entry: local picker + repo_path editor
                              #   + "content will be moved into the repository" warning
```

The list view is entry-centric, because that is what the invariants are about:

```
[Device: MacBook Pro (current)]  [+ New Entry]  [Apply]  [Audit]  [Detach]
────────────────────────────────────────────────────────────────────
▾ opencode/opencode.json                             file   ok
    ● ~/.config/opencode/opencode.json        MacBook Pro     [re-adopt?] [remove]
    ● ~/Desktop/opencode.json                 MacBook Pro     [remove]
    ○ ~/work/opencode/opencode.json           MacBook-Pro-2   other device
▸ docs/notes                                          dir   missing — [repair] [remove]
▸ projects/vendor                                     dir   0 links — [add link]
```

Links are listed uniformly: there is no in/out badge and no "set as tracked" action, because all links are equal (§9.3.3). The only per-link actions are **repair** (for `missing` / `wrong_target`), **re-adopt** (for `replaced`) and **remove**. `Add Link` is always available, including for an entry with no links.

`components/files/FilesPanel.tsx` (Browse) renders the `data/` tree and badges each node: has entry / not an entry / has link drift. The `symlink/` components are deleted.

Type additions in `frontend/src/types/index.ts`:

```typescript
export type EntryKind = 'file' | 'dir';
export type LinkState =
  | 'ok' | 'missing' | 'wrong_target' | 'replaced'
  | 'dangling' | 'occupied' | 'disabled' | 'not_current';

export interface Device {
  fingerprint: string;
  name: string;
  hostname?: string;
  os?: string;
  is_current: boolean;
  last_seen_at?: string | null;
  link_count: number;
}

export interface Link {
  id: string;
  entry_id: string;
  device: string;
  device_name?: string;
  local_path: string;
  enabled: boolean;
  is_current: boolean;
  created_at: string;
  state: LinkState;
  state_note?: string;
}

export interface Entry {
  id: string;
  repo_path: string;
  kind: EntryKind;
  created_at: string;
  links: Link[];
}
```

`api/client.ts` drops every symlink function and gains the entry, link, device and consistency functions.

### 9.13 What This Deletes

| Deleted | Replaced by |
|------|------|
| `internal/service/symlink_service.go` (~720 lines) | `internal/entry/entry_service.go` + `link_service.go` |
| `internal/service/backup_service.go` sync half (`syncChangedFiles`, `syncOneFile`, `syncDirectoryFiles`, `walkSourceDir`, `cleanEmptyDataDirs`) | nothing — content is already in `data/` |
| `SymlinkService.SyncDeletedSource` | nothing — the same class of data-loss bug cannot occur, because existence is decided by `data/`, not by a local path |
| `internal/resolver/symlink_resolver.go` | nothing — rollback writes `data/` directly; every link reflects it automatically |
| `internal/store/symlink_store.go`, `model/symlink.go` | `internal/entry/manifest.go`, `model/link.go` |
| `.links/` and all mirror-consistency code | nothing — one representation |
| `SymlinkHandler` (8 endpoints), including `nested` | `EntryHandler` + `DeviceHandler` |
| `is_new` computation | `GET /repos/:id/changes` (`git status`) |
| `PreviewService.ResolveSource` prefix matching | `path` is always repo-relative |
| Preview dual write (source **and** `data/`) | a single write to `data/` |
| `.gitignore` generation in `repo_service.go` | nothing — nothing needs ignoring |

`BackupService.Trigger` reduces to:

```
1. repo-level mutex
2. status = backing_up
3. write the manifest (flush any pending definition change) and commit it if dirty
4. git add -A
5. git commit -m <message>
6. optional git push  (failure does not block the local commit)
7. last_backup_at = now; status = active (error on failure)
```

### 9.14 Security

| Risk | Protection |
|------|------|
| `adopt` destroys the original file | Explicit confirmation: "the file will be moved into the repository and this location replaced by a symlink"; full rollback on every failure path; in the cross-filesystem path the source is removed only after the size is verified |
| A link overwrites unrelated data | Adding a link refuses an existing non-empty path; `apply` never overwrites — `occupied` items are reported, not forced |
| `purge` deletes content | Requires typing the `repo_path`; the previous commit restores it; every other link is reported before the operation |
| Path traversal | `util.SafeResolve` / `util.SafeJoin` on every user path; local paths limited to AllowedRoots (`$HOME` + repo roots) |
| Self reference | `local_path` inside `repo.Path` is rejected |
| Dangerous targets | `/`, `$HOME`, and the repository root itself are rejected as `local_path` |
| Symlink loops | A candidate chain is resolved with `util.ResolveNestedSymlink` before any link is created; cycles are rejected |
| Adopting a tree that contains symlinks | Refused unless `follow_symlinks` is set (§9.4.1), so `data/` never contains a symlink pointing outside |
| Hand-edited manifest | Validated before every write against R-1..R-3; every path re-checked by `SafeResolve`; a malformed file blocks writes, and an invalid one is reported by the audit |
| Concurrent changes | All filesystem-mutating operations hold the repo-level mutex from `RepoMutexManager` |
| Manifest write tearing | `manifest.json.tmp` → `fsync` → `os.Rename` |

### 9.15 Boundary & Exceptions

| Scenario | Handling |
|------|------|
| `adopt` source is already a symlink | 400 — reject (avoids chained links) |
| `adopt` source is inside `repo.Path` | 400 — reject |
| `adopt` source is the repo itself or an ancestor | 400 — reject |
| `adopt` source tree contains symlinks | 409 — refuse, listing them; `follow_symlinks: true` to dereference |
| `repo_path` overlaps an existing entry | 409 (R-2) |
| `data/<repo_path>` already exists | 409 |
| `os.Rename` crosses filesystems | Degrade to `CopyFile` + size verify + `Remove` |
| `mv` succeeded but link creation failed | Roll back: move the content back |
| Link target holds a real file | 409 — ask the user to move it first |
| Link target is a non-empty directory | 409 |
| Link `local_path` inside a directory entry's `local_path` | 409 (R-3) |
| Removing the last link of an entry | Allowed — the entry and its content stay; it simply has no local view |
| Deleting a device | Its link definitions go away; entries left with no links are legal |
| `data/<repo_path>` missing from the repo | `dangling`; `apply` reports it, never creates the link |
| Adding a link to an entry that has none | Allowed — this is how a new device binds content that already exists in the repository |
| Manifest has a structural problem (bad id, unknown device, overlap) | Reported by the audit; writes are refused until it is fixed |
| Fingerprint unavailable | Degrade to `sha256(hostname+username)` with a warning; the user may name the device manually |
| Manifest unparsable (Git conflict) | 409 with the raw error; all writes blocked until resolved |
| Two machines edit the manifest | Git conflict, resolved by Git; the app never auto-merges |
| Same `local_path` used by two links on one device | 409 |

### 9.16 Testing

**Unit** (`internal/entry/`):

- Manifest: load / save / atomic write / missing file / malformed file / unknown fields preserved / id auto-assignment
- Manifest validation: overlapping entries (R-2) and links nested inside a directory entry (R-3) rejected; a linkless entry accepted
- `MachineFingerprint`: each platform branch + fallback determinism
- Adopt: happy path, R-2 conflict, `data/` conflict, cross-filesystem fallback, rollback at each failure point, source tree containing a symlink
- Add link: happy path, occupied target, non-empty directory, self-reference, R-3 violation, several links for one entry
- **Link equality**: two links of the same entry both read and write the same content
- Removing the last link leaves the entry and its content intact
- Manifest round-trip of a linkless entry survives save/load unchanged
- State diagnosis: all 8 states
- Re-adopt after a simulated atomic write (replace the symlink with a real file)
- Detach: `unlink` removes the local symlinks and a following `apply` restores them; `keep` leaves the filesystem alone
- Remove: link-level removal, entry-level `unlink` / `move_back` / `purge`, plus the guards
- Consistency audit: one fixture per finding code

**Integration**:

- Adopt → edit through the first link → the change is immediately visible in `data/` → `git commit` captures it
- Adopt → delete a link's symlink → `data/` survives → state is `missing` → `apply` recreates it
- One entry with three links on one device → editing any of them produces exactly one change in `data/`
- Two-device simulation: build the manifest on A → clone → register device B → `bulk` → B's links match B's layout while `data/` is unchanged
- Device A deleted → its links go away; entries that had only A's links remain valid and the audit stays clean
- R-2/R-3 rejection paths at the HTTP layer (409 with a readable message)
- `purge` then `git revert` → content restored
- Concurrent `apply` and backup → no corruption

### 9.17 Upgrade Behaviour (No Migration)

Because backward compatibility is explicitly out of scope:

| Item | Behaviour |
|------|------|
| `symlinks` table | Dropped on first start of the new version |
| `.links/` directory | Deleted from existing repos on first open (a single `os.RemoveAll`) |
| `data/` content | **Preserved untouched** — nothing holding user data is deleted |
| `manifest.json` | Created empty (`{"version":1,"devices":[],"entries":[]}`) |
| Re-establishing entries and links | `adopt` for each tracked file/directory, then `bulk` for distribution |
| Recovering the old source paths | Not possible automatically — the old `target_path` values are discarded with the table |
| Repos registered in SQLite | Kept; `repos` / `repo_configs` / `repo_auths` are unchanged |
| Repo `.gitignore` | Left alone if present; no longer generated for new repos |

### 9.18 Milestones

| Milestone | Scope |
|------|------|
| M1 | `model/link.go`, `entry/manifest.go`, atomic write, R-1..R-3 validation, id assignment |
| M2 | `util/device.go` (fingerprint) + device register/list/rename/delete |
| M3 | Entry creation: adopt (mv, R-2 check, symlink scan, cross-filesystem degrade, rollback) |
| M4 | Link creation: add link, R-3 check, bulk link, path safety |
| M5 | State diagnosis + `apply` (dry run + execution) + repair + re-adopt |
| M6 | Removal: link-level, entry-level `unlink` / `move_back` / `purge`, guards; device `detach` |
| M7 | Consistency audit + repair |
| M8 | Strip the old subsystem: delete `.links/`, `symlinks` table, sync machinery, resolver, symlink API; simplify `BackupService.Trigger` |
| M9 | Content API simplification (§7) + `changes` endpoint |
| M10 | Frontend: Entries tab, modals, badges, Browse integration |
| M11 | Docs sync (all docs and READMEs, EN + ZH) |

### 9.19 Deferred Items (not yet implemented)

The main line above is implemented and verified. The following are deliberately postponed — all of them are additive and none of them changes the model:

| Item | Why deferred |
|------|------|
| **Entry rename** (`PATCH /entries/:entryId`) | Requires re-pointing every existing symlink of the entry, so it is more than a metadata change |
| **Per-link edit** (`PATCH /entries/:entryId/links/:linkId`, `{local_path?, enabled?}`) | `enabled` is already honoured by `apply` and by the R-3 check; only the endpoint is missing |
| **`GET /entries/:entryId/links`, `GET /devices/:fingerprint/links`** | Convenience projections — the entry list already returns every link with its computed state |
| **Server-side entry filters** (`?device=&state=`) | The entry list is small; the front end groups and filters client-side |
| Bulk-link UI, device rename UI | Front-end conveniences; the APIs (`POST /links/bulk`, `PATCH /devices/:fingerprint`) already exist |
| Recovering a `replaced` link in bulk | `readopt` works per link (§9.7); a batch variant is not written |

