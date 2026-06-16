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
| Database | SQLite (mattn/go-sqlite3) | No additional database service needed, single file storage, suitable for desktop-grade applications |
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
   ┌──────────┐           ┌──────────────────┐
    │  Source  │           │  Repository Root │
    │  Files   │◀─symlink──│  ├─ .links/       │
    │  (Any    │           │  ├─ data/         │
    │   Path)  │────copy──▶│  └─ .git/         │
                          └──────────────────┘
```

### Architecture Core Principles

| Principle | Description |
|------|----------|
| **Full-stack Integrated** | All code compiled into a single binary, frontend embedded via embed.FS |
| **Responsive API** | RESTful JSON API, stateless design (state persisted by SQLite) |
| **Path Safety First** | All user-input paths must pass through SafeResolve security validation function |
| **Mirror Consistency** | `.links/` and `data/` always mirror each other (synchronized when adding/deleting symlinks) |
| **Auth Isolation** | Git auth info stored encrypted, only injected as environment variables during git operations |

## 3. Detailed Design

### 3.1 Route Registration

```
POST   /api/v1/repos                          → RepoHandler.Create
GET    /api/v1/repos                          → RepoHandler.List
GET    /api/v1/repos/:id                      → RepoHandler.Get
DELETE /api/v1/repos/:id                      → RepoHandler.Delete
PUT    /api/v1/repos/:id/config               → RepoHandler.UpdateConfig  // ★ P0-1: Config Editing

POST   /api/v1/repos/:id/symlinks             → SymlinkHandler.Create
GET    /api/v1/repos/:id/symlinks             → SymlinkHandler.List
GET    /api/v1/repos/:id/symlinks/:linkId     → SymlinkHandler.Get
DELETE /api/v1/repos/:id/symlinks/:linkId     → SymlinkHandler.Delete
PUT    /api/v1/repos/:id/symlinks/:linkId     → SymlinkHandler.UpdateTarget
POST   /api/v1/repos/:id/symlinks/batch       → SymlinkHandler.BatchImport

GET    /api/v1/browse         ?path=...&root=...  → BrowseHandler.Browse   // ★ P0-2: Security Fix
GET    /api/v1/repos/:id/preview ?path=...        → PreviewHandler.Preview // ★ P0-2: Security Fix

POST   /api/v1/repos/:id/backup               → BackupHandler.Trigger
GET    /api/v1/repos/:id/backup/history?limit=&offset= → BackupHandler.History

GET    /api/v1/repos/:id/auth                 → AuthHandler.Get
PUT    /api/v1/repos/:id/auth                 → AuthHandler.Set
DELETE /api/v1/repos/:id/auth                 → AuthHandler.Clear

GET    /api/v1/health                         → SystemHandler.Health
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

### 3.4 Symlink Mirror Consistency (★ P0-5 Fix)

When adding a symlink, **synchronously copy source file to data/**, ensuring `.links/` and `data/` are always consistent:

```
CREATE Symlink:
  1. Validate sourcePath → SafeResolve
  2. Compute relative path
  3. os.Symlink(sourcePath → .links/<relPath>)
  4. copyFile/copyDir(sourcePath → data/<relPath>)  ← sync copy
  5. INSERT symlinks table

DELETE Symlink:
  1. Delete .links/<relPath>
  2. Delete data/<relPath>
  3. Clean up empty directories
  4. DELETE FROM symlinks

UPDATE Symlink Target:
  1. Delete old .links/ + data/ files
  2. Create new symlink → copy new source file
  3. Update database
```

Backup operation reduces to: `incremental detect source file changes → sync to data/ → git add -A → git commit → git push`

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

### SQLite Table Schema

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

CREATE TABLE symlinks (
    id              TEXT PRIMARY KEY,
    repo_id         TEXT NOT NULL,
    relative_path   TEXT NOT NULL,
    target_path     TEXT NOT NULL,
    type            TEXT NOT NULL,
    file_size       INTEGER,
    modified_at     DATETIME,
    created_at      DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE CASCADE,
    UNIQUE(repo_id, relative_path)
);
```

### App Configuration

Path: `~/.config/backup-manager/config.json`
```json
{
  "port": 9800,
  "openBrowser": true,
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
│   │   ├── middleware/
│   │   └── handler/
│   │       ├── repo.go
│   │       ├── symlink.go
│   │       ├── browse.go
│   │       ├── preview.go
│   │       ├── backup.go
│   │       ├── auth.go
│   │       └── system.go
│   ├── model/
│   │   ├── repo.go
│   │   ├── symlink.go
│   │   └── auth.go
│   ├── service/
│   │   ├── repo_service.go
│   │   ├── symlink_service.go
│   │   ├── backup_service.go
│   │   └── auth_service.go
│   ├── store/
│   │   ├── db.go
│   │   ├── repo_store.go
│   │   ├── repo_config_store.go
│   │   ├── repo_auth_store.go
│   │   └── symlink_store.go
│   ├── git/
│   │   └── git.go
│   ├── scheduler/
│   │   └── scheduler.go
│   └── util/
│       ├── path.go          # SafeResolve security function
│       ├── crypto.go        # AES-GCM encryption
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

## 7. Preview & Edit Feature Design

### 7.1 Overview

On the Preview page, users can directly preview and edit the **source file** (not the `data/` copy) pointed to by the symlink. When saving edits, it writes to both the source file and syncs to `data/` to maintain mirror consistency.

**Core change**: Preview reads from source file path (`symlink.TargetPath`) instead of `data/`.

### 7.2 Backend Design

#### 7.2.1 Store Layer — New Query Method

Add a method to query symlink by `relative_path` in `internal/store/symlink_store.go`:

```go
func (s *Store) GetSymlinkByRelativePath(repoID, relativePath string) (*model.Symlink, error)
```

Utilizes the existing `UNIQUE(repo_id, relative_path)` constraint, no new index needed.

#### 7.2.2 Service Layer — New PreviewService

Create `internal/service/preview_service.go`, containing the following core methods:

**ResolveSource** — resolves `relative_path` to the source file absolute path:

```
Input: repoID, relPath = "docs/notes/readme.md"

Match strategy (by priority):
  1. Exact match: sym.RelativePath == relPath → return sym.TargetPath
  2. Longest prefix: iterate directory-type symlinks,
      check strings.HasPrefix(relPath, sym.RelativePath+"/")
      → return filepath.Join(sym.TargetPath, suffix)
  3. No match: return error
```

**SaveFile** — saves edited file content to source file and syncs to `data/`:

```
Flow:
  1. ResolveSource to get source file path
  2. Verify it's not directory-type symlink
  3. os.Stat to get original file permission mode
  4. os.WriteFile(sourcePath, content, mode) → write to source file
  5. os.Chmod(sourcePath, mode) → ensure permission not affected by umask
  6. os.MkdirAll(filepath.Dir(dataPath), 0755)
     os.WriteFile(dataPath, content, mode) → sync to data/
     os.Chmod(dataPath, mode) → preserve permission in data/ too
  7. store.UpdateSymlink → update file_size, modified_at
```

**API Contract**:

```go
// PUT /api/v1/repos/:id/save
// Content-Type: application/json
type SaveRequest struct {
    Path    string `json:"path" binding:"required"`    // symlink relative path
    Content string `json:"content" binding:"required"` // file content (UTF-8 text)
}
// Response: {"data": {"file_size": 1234, "modified_at": "2026-06-12T10:30:00Z"}}
```

Constraints:
| Condition | Handling |
|------|------|
| Content > 10MB | 413 Request Entity Too Large |
| Directory symlink | 400 Bad Request |
| No matching symlink | 404 Not Found |
| Source file has been deleted | 404 Not Found |

#### 7.2.3 Handler Layer — Refactor PreviewHandler

Refactor `internal/api/handler/preview.go`:

```go
type PreviewHandler struct {
    previewSvc *service.PreviewService
    semaphore  chan struct{}     // Max 5 concurrent, shared between Preview and Save
}
```

- **Preview method**: Rewritten to read from source file via `PreviewService.Preview(repoID, relPath)`
- **Save method**: New, calls `PreviewService.SaveFile(repoID, path, content)`

#### 7.2.4 Route Registration

```go
v1.GET("/repos/:id/preview", previewHandler.Preview)  // unchanged
v1.PUT("/repos/:id/save", previewHandler.Save)         // added
```

#### 7.2.5 main.go Dependency Injection

```go
previewSvc := service.NewPreviewService(dataStore)      // added
previewHandler := handler.NewPreviewHandler(previewSvc)  // refactored
```

### 7.3 Frontend Design

#### 7.3.1 Type Definitions

```typescript
export interface SaveFileRequest {
  path: string;
  content: string;
}

export interface SaveFileResult {
  file_size: number;
  modified_at: string;
}
```

#### 7.3.2 API Client

```typescript
export async function saveFile(repoId: string, req: SaveFileRequest): Promise<SaveFileResult> {
  const { data } = await api.put<SaveFileResult>(`/repos/${repoId}/save`, req);
  return data;
}
```

#### 7.3.3 TextPreview Refactor

- Added `editable`, `onContentChange`, `onSave`, `saving` props
- View mode: existing `<pre>` read-only display + "Edit" button
- Edit mode: `<textarea>` + "Save"/"Cancel" buttons
- Truncated files disable edit button

#### 7.3.4 MarkdownPreview Refactor

- Added `editable`, `onContentChange`, `onSave`, `saving` props
- Use Ant Design Tabs for Preview/Edit mode switching
- Preview mode: render Markdown using react-markdown + remark-gfm; local references (images/docs) are rendered by browser default without path rewriting
- Edit mode: Markdown source textarea
- Switching tabs does not lose edit content

#### 7.3.5 PreviewPanel Refactor

- Added `editingContent` state for content being edited
- Added `saving` state for saving in progress
- `handleSave` calls `saveFile` API → refreshes preview → shows success

### 7.4 Data Flow

**Preview flow**:
```
User clicks file node → GET /preview?path=<relPath>
  → PreviewService.ResolveSource → resolve to source file path
  → Read content from source file → return {content, mime_type, size, text, truncated}
  → Frontend renders TextPreview / MarkdownPreview / BinaryInfo
```

**Save flow**:
```
User edits content → clicks save → PUT /repos/:id/save {path, content}
  → Validate: content ≤ 10MB, path not empty
  → ResolveSource → resolve source file path
  → os.WriteFile(sourcePath, content, origMode) → write to source file
  → os.WriteFile(dataPath, content, origMode) → sync to data/
  → UpdateSymlink → update metadata
  → Return {file_size, modified_at}
  → Frontend refreshes preview → shows "File saved successfully"
```

### 7.5 Boundary & Exception Handling

| Scenario | Handling | Status Code |
|------|----------|--------|
| path is empty | Return error | 400 |
| content is empty | Return error | 400 |
| content > 10MB | Return size limit prompt | 413 |
| No matching symlink | Return "no matching symlink" | 404 |
| Target is directory symlink | Return "cannot save directory" | 400 |
| Source file externally deleted | os.Stat fails | 404 |
| Write source succeeds but data/ sync fails | Log error, source already updated no rollback | 200 (log warning) |
| No write permission | os.WriteFile fails | 500 |

### 7.6 Security Considerations

| Risk | Protection |
|------|----------|
| Path traversal | Path comes from database (WHERE repo_id = ?), not directly user-constructed |
| Writing binary files | Frontend truncated disables editing; backend content size validation |
| Large content OOM | Content length ≤ 10MB |
| Permission loss | Read original mode before write, Chmod to restore after write |

### 7.7 Change Impact

| Impact Point | Change | Impact Level |
|--------|------|----------|
| Preview read path | data/ → source file | ✅ Behavior change, meets requirements |
| Preview response format | Unchanged | ✅ Compatible |
| ListDirEntries | Still reads data/ | ✅ No modification needed |
| Backup flow | Unchanged. After save mtime changes, next backup auto-detects | ✅ Compatible |
| Rollback service | Unchanged. SymlinkResolver already handles correctly | ✅ Compatible |
| SafeResolveFile(dataDir) | Preview no longer uses it | ⚠️ Remove one call |

### 7.8 Testing Strategy

**Unit tests**:
- `PreviewService.ResolveSource`: exact match, prefix match, no match
- `PreviewService.SaveFile`: metadata update after save, permission preservation, source file does not exist
- Save Handler: content exceeds limit, directory symlink, invalid request format

**Integration tests**:
- File symlink preview → source file content correct
- Directory symlink internal file preview → content correct
- Edit save → re-preview → content updated
- Concurrent save and backup → no data corruption
