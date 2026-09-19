# Requirements Analysis Document — Backup Manager

> 🇨🇳 [中文文档](docs/zh/REQUIREMENT.md)

## 1. Product Overview

| Field | Content |
|------|------|
| Product Name | Backup Manager |
| Product Positioning | File/directory aggregated backup visual management software |
| One-line Description | A backup management tool based on Git's reverse tracking mode (whitelist mechanism), aggregating and managing source files through symlinks, with a visual interface. |
| Core Value | Enables users to manage backups intuitively by "specifying what to back up" (rather than "what to exclude"), while providing a visual operation interface to lower the barrier to backup management. |

### 1.1 Core Design Principles

**Reverse Tracking Model**: Contrary to `.gitignore`'s "exclusion mode", users proactively specify which files/directories need to be tracked and backed up; unspecified files are automatically ignored. Similar to a whitelist system.

**Entry & Link Aggregation**: A backup repository (repo) holds the real content under `data/`. An **entry** is one backed-up file or directory at `data/<repo_path>` — it is the whitelist member. A **link** is a local path bound to that entry as a symlink into `data/`. All links are completely equivalent: editing through any of them is the same operation on the same content, so the model stores no in/out distinction.

**Unified Frontend and Backend**: The system adopts a unified frontend-backend architecture, running as a single process with one-click startup, eliminating the need to deploy frontend and backend services separately.

---

## 2. User Roles

| Role | Description | Core Needs |
|------|------|----------|
| **Regular User** | Individual users who want to back up their own files/directories | Simple and easy to use, visual operations, quick file content preview |
| **Advanced User** | Users with some technical background who understand Git concepts | Fine-grained control over backup strategies, view Git commit history, manually manage symlinks, roll back source files |

---

## 3. Functional Requirements

### 3.1 Backup Repository Management

| ID | Feature | Description | Priority |
|----|------|------|--------|
| FR-1 | Create Backup Repository | User creates a new backup repository via UI, selecting a local path as the repo root directory | P0 |
| FR-2 | View Repository List | Display all created backup repositories with basic information | P0 |
| FR-3 | Delete Backup Repository | Delete an existing backup repository (only removes database records, preserves filesystem data) | P1 |
| FR-4 | Edit Repository Config | Visually edit repository configuration: remote URL, branch, Git username/email, scheduled backup toggle and interval | P0 |

### 3.2 Entry & Link Management (data/)

An **entry** is one backed-up file/directory at `data/<repo_path>`. A **link** binds a local path to an entry; an entry may have **0..N links**. Because every link is a symlink to the same `data/<repo_path>`, all links are equal — there is no primary link, no tracked link, and nothing to switch. An entry with no links is legal: the content is in the repository, there is just no local view of it.

| ID | Feature | Description | Priority |
|----|------|------|--------|
| FR-5 | Create Entry (Adopt) | User selects a local file/directory via UI; its content is **moved** into `data/<repo_path>` and the original location is replaced by a symlink. Entry and link are created together | P0 |
| FR-6 | View Entry & Link List | Display all entries grouped by `repo_path`, each expandable to show its links, the owning device, and each link's state | P0 |
| FR-7 | Add Link | Distribute an entry to another local path by creating a symlink to `data/<repo_path>`; any number of links per entry is allowed, including for an entry that has none yet (this is how a new device binds content that already exists in the repository) | P0 |
| FR-9 | Bulk Link | Pick multiple entries plus one local root directory and create one link per entry at `<local_root>/<repo_path>` | P1 |
| FR-10 | Link State Diagnosis & Repair | Diagnose each link (`ok` / `missing` / `wrong_target` / `replaced` / `dangling` / `occupied` / `disabled`) and offer repair / re-adopt | P0 |
| FR-11 | Consistency Audit | Verify the invariants (links bind whole entries, never sub-paths; entries never overlap; no link inside a directory entry; no symlink inside `data/`, no link referencing an unregistered device) and report unmanaged links; one-click repair for everything convergable | P1 |
| FR-25 | Remove Link / Entry | Remove a single link (safe; the entry and its content stay), or remove an entry via `unlink` (this device's links only) / `move_back` (content returns to a chosen local path) / `purge` (delete the content) | P0 |

**Consistency rule (must hold)**: links stay completely consistent with the backed-up file/directory. If a directory is tracked, no link may point at a single file inside it. Consequently entries never overlap, and a link is always bound to a whole entry.

### 3.3 Multi-Device Management

A **device** is one machine referencing the repository, identified by a stable machine fingerprint. Device is metadata; links are its child data.

| ID | Feature | Description | Priority |
|----|------|------|--------|
| FR-26 | Device Registration | Detect the current machine's fingerprint, register it on the repo automatically (name defaults to the hostname, renameable); list all devices with their link counts | P0 |
| FR-27 | Apply Device | Converge this machine: show a dry-run plan (create / repair / skip / conflict / orphan) and execute it after confirmation. Never overwrites an occupied path | P0 |
| FR-28 | Detach Device | Remove this device's local symlinks (`unlink`) or just stop managing them (`keep`). `data/` is never touched. Definitions stay, so re-attaching is a single Apply | P1 |
| FR-29 | Delete Device | Delete a device's link definitions. Entries keep existing; one left with no links is legal | P1 |

### 3.4 File Preview and Editing

| ID | Feature | Description | Priority |
|----|------|------|--------|
| FR-12 | Plain Text File Preview and Edit | View and edit plain text content in the UI (e.g., .txt, .log, .json, .yaml, .py, etc.), with save support. The operation targets `data/<repo_path>`; every link reflects the change immediately because it is a symlink to that same file. | P0 |
| FR-13 | Markdown Rendered Preview and Edit | Render and display Markdown files (.md) using react-markdown + remark-gfm. Supports toggling between edit mode and preview mode, saving edits to `data/<repo_path>`. Local references (images/docs) are rendered by browser default without path rewriting. | P0 |
| FR-14 | Binary File Identification | Display file type information and size for non-text files, without attempting to preview content | P2 |

### 3.5 Backup Execution

| ID | Feature | Description | Priority |
|----|------|------|--------|
| FR-15 | Execute Backup | Manually trigger a backup: flush any pending manifest change → `git add -A` → `git commit` → (optional) `git push`. No incremental sync is involved, because content already lives in `data/` | P0 |
| FR-16 | Scheduled/Auto Backup | Automatically execute backups at scheduled times based on configured cron expression, auto-load enabled repositories on application startup | P1 |
| FR-17 | View Backup History | View the repository's Git commit history with pagination support | P1 |
| FR-18 | Content Rollback | Select a historical commit and restore `data/` to that version. Supports full rollback, selective rollback by entry, and single-file restore; commit file content can be previewed before rollback. Because every link points into `data/`, all local paths reflect the rollback immediately | P1 |
| FR-30 | Uncommitted Change Indicator | Show the number of uncommitted changes under `data/` (from `git status`) as the "this entry has an update" signal | P2 |

### 3.6 Configuration Management

| ID | Feature | Description | Priority |
|----|------|------|--------|
| FR-19 | Git Remote Repository Config | Visually configure the Git remote repository URL and target branch | P0 |
| FR-20 | Git Authentication Config | Configure authentication information required for Git operations (SSH private key or HTTPS username/password), stored encrypted in SQLite | P1 |
| FR-21 | Application Global Settings | Manage application-level basics (port number, theme, whether to auto-open the browser) and provide an in-UI language switch for English (`en`) and Simplified Chinese (`zh-CN`). English is the default; the selected language is persisted by the backend as an app-wide setting | P1 |

Settings API contract: `GET /api/v1/settings` returns `{"data":{"language":"en"}}`; `PUT /api/v1/settings` accepts `{"language":"en"}` or `{"language":"zh-CN"}` and returns the same `data.language` shape.

### 3.7 System Management

| ID | Feature | Description | Priority |
|----|------|------|--------|
| FR-22 | Application Start/Stop | One-click start and stop of the entire application, auto-open browser after startup. System tray icon provides "Open UI", "Start/Stop Server", and "Quit" controls. | P0 |
| FR-23 | Local File Browser | Safely browse the local filesystem for selecting entry sources and link target paths, limited to the user's home directory and repo root directory | P0 |
| FR-24 | Health Check | Provide `/health` endpoint returning application running status, startup time, and version information | P2 |

---

## 4. Non-Functional Requirements

| ID | Requirement | Description |
|----|------|------|
| NFR-1 | **Unified Frontend and Backend Architecture** | Frontend UI and backend service integrated into a single application, running as a single process |
| NFR-2 | **Cross-Platform Support** | Support at least macOS and Linux |
| NFR-3 | **Responsive UI** | Interface adapts to different screen sizes |
| NFR-4 | **Security** | Require confirmation before removing links/entries and repositories; path safety checks on every user-supplied path; deleting content requires typed confirmation |
| NFR-5 | **Link Consistency** | All links of an entry point at the same content object and are completely equivalent; once a directory is tracked, no link may bind a single file inside it — links always bind whole entries and entries never overlap (R-1..R-3) |
| NFR-6 | **Backup Atomicity** | Failed backups should have clear prompts and error status |
| NFR-7 | **Usability** | Core features should be completable within 3 clicks |
| NFR-8 | **Startup Behavior** | Auto-open browser after startup |
| NFR-9 | **Path Safety** | Four-layer path validation (Clean→Abs→EvalSymlinks→Prefix) to prevent path traversal |
| NFR-10 | **Concurrency Safety** | Independent mutex per repository to prevent concurrent backups; preview/edit API rate-limited (max 5 concurrent) |
| NFR-11 | **Sensitive Information Encryption** | SSH private keys and HTTPS passwords encrypted with AES-256-GCM before storage in SQLite, key file permissions 0600 |
| NFR-12 | **Localization Consistency** | Switching language updates application text, Ant Design components, and dayjs formatting together; a persisted app-wide choice is restored before the UI is shown |

---

## 5. Core Concepts / Data Model

### 5.1 Directory Structure

```
<repo-root>/
├── .backup-manager/
│   └── manifest.json    # devices + entries + links (git-tracked, single source of truth)
├── data/                # the real content — the only content store
│   ├── documents/
│   │   ├── report.docx
│   │   └── notes.txt
│   └── config/
│       └── settings.json
└── .git/                # Git repository

Local machine — every link is a symlink into the repo:
  ~/Documents/report.docx  ->  <repo>/data/documents/report.docx
  ~/Desktop/notes.txt      ->  <repo>/data/documents/notes.txt
  ~/Desktop/report.docx    ->  <repo>/data/documents/report.docx
```

### 5.2 Core Entities

```
BackupRepo
├── id: string
├── name: string
├── path: string          # Absolute path to the repo root directory
├── createdAt: timestamp
├── updatedAt: timestamp
├── lastBackupAt: timestamp|null
├── status: 'active' | 'error' | 'backing_up'
├── config:
│   ├── remoteUrl: string
│   ├── branch: string (default: main)
│   ├── autoBackup: boolean
│   ├── autoBackupInterval: string (cron expression)
│   ├── gitUserName: string
│   └── gitUserEmail: string
└── entries: Entry[]
    └── links: Link[]

Entry                          # one backed-up file/directory
├── id: string
├── repoPath: string           # path under data/ — the identity of the content
├── kind: 'file' | 'dir'
├── createdAt: timestamp
└── links: Link[]              # 0..N links, all completely equal (no in/out type)

Link                           # a local path bound to an entry
├── id: string
├── entryId: string
├── device: string             # device fingerprint
├── localPath: string          # absolute local path of the symlink
├── enabled: boolean
└── createdAt: timestamp

Device                         # metadata about one machine
├── fingerprint: string        # stable machine id (sha256 hex) — primary key
├── name: string               # defaults to the hostname, renameable
├── hostname, os: string
└── lastSeenAt: timestamp
```

### 5.3 Persistence

There are **three separate stores** with different files, formats and reasons to exist:

| Data | File | Format | Why there |
|------|------|------|------|
| `repos`, `repo_configs`, `repo_auths` | `~/.config/backup-manager/backup-manager.db` | SQLite (binary) | Machine-private: encrypted credentials, local paths, schedules. Never committed |
| entries, links, devices | `<repo-root>/.backup-manager/manifest.json` | JSON, Git-tracked | Must travel across machines. The SQLite file is per machine and cannot |
| app settings | `~/.config/backup-manager/config.json` | JSON | Application-level settings, including the app-wide `language` (`en` by default) |

**SQLite database** — `~/.config/backup-manager/backup-manager.db`, three tables, unchanged:

```sql
repos         — Repository: id, name, path, created_at, updated_at, last_backup_at, status
repo_configs  — Config: repo_id(FK), remote_url, branch, auto_backup, auto_backup_interval, git_user_name, git_user_email
repo_auths    — Auth: repo_id(FK), auth_type, ssh_private_key(BLOB), ssh_private_key_path, username, password_encrypted(BLOB)
```

The old `symlinks` table is removed and no table replaces it — entries, links and devices are **not** stored in SQLite.

**Repository manifest** — `<repo-root>/.backup-manager/manifest.json`, a JSON file inside the repo:

```json
{
  "version": 1,
  "updated_at": "2026-09-18T10:00:00Z",
  "devices": [
    { "fingerprint": "9f2c…", "name": "MacBook Pro", "hostname": "mbp.local",
      "os": "darwin", "last_seen_at": "2026-09-18T10:00:00Z" }
  ],
  "entries": [
    { "id": "e1a2…", "repo_path": "documents/notes.txt", "kind": "file",
      "created_at": "2026-09-01T08:12:00Z",
      "links": [
        { "id": "l1a2…", "device": "9f2c…",
          "local_path": "/Users/x/Documents/notes.txt", "enabled": true,
          "created_at": "2026-09-01T08:12:00Z" },
        { "id": "l2b3…", "device": "9f2c…",
          "local_path": "/Users/x/Desktop/notes.txt", "enabled": true,
          "created_at": "2026-09-10T12:00:00Z" }
      ] }
  ]
}
```

Entries, links and devices live inside the repository rather than in SQLite because the database is per-machine. Storing them in the repo (where Git versions and transports them) is what lets a new machine discover every device's links with a plain `git clone`.

---

## 6. Key Decisions Confirmed by User

| Issue | Decision |
|------|------|
| Git Remote Repository | `git push` is optional. When remote is not configured, only local commits are made without push |
| **Content Ownership** | Content lives **only** in `data/<repo_path>`. A local path is never a second copy but a symlink view onto it |
| **Link Equality** | An entry has 0..N links and all of them are equal — same target, same semantics. There is no `in`/`out` type, no tracked link, and nothing to switch |
| **Link Consistency** | Links always bind a whole entry. A tracked directory forbids a link to a file inside it; entries never overlap |
| **Entry = Whitelist Member** | An entry's existence in the manifest is what makes it backed up. An entry with no links is legal — the content is in the repository with no local view |
| **Device Metadata Location** | Devices, entries and links are stored in `<repo>/.backup-manager/manifest.json` inside the repository (git-tracked), not in the per-machine SQLite database, so a new machine learns them with a plain `git clone` |
| **Adopt Semantics** | Creating an entry **moves** the source into the repo and replaces the original location with a symlink. The original file is never left behind as a second copy |
| Content Removal | Removing content requires a typed `repo_path` confirmation; `unlink` / `move_back` are offered as non-destructive alternatives; a commit precedes every removal so `git revert` always works |
| Frontend Technology Stack | React 18 + TypeScript + Vite + Ant Design 5 + i18next/react-i18next; Ant Design and dayjs use the active UI locale |
| UI Language | Support `en` and `zh-CN`, default to `en`, and persist the selected app-wide default in `config.json` |
| Startup Behavior | Auto-open browser after startup |
| Markdown Images | Support local image display in Markdown |
| Multiple Repositories | Support parallel management of multiple repositories |
| Backend Framework | Gin (Go lightweight high-performance HTTP framework) |
| Database | SQLite (pure Go implementation via modernc.org/sqlite, no CGO required) |
| Authentication Encryption | SSH private keys and HTTPS passwords stored encrypted with AES-256-GCM |
| Content Rollback | Rollback overwrites `data/` (requires user confirmation) and displays the changed-file list first. All local links reflect it immediately |
| Repository Deletion | Deleting a repository only removes database records and scheduled tasks, preserving filesystem data without loss |
| **Preview/Edit Target** | Preview and edit operate on `data/<repo_path>` — a single write, no dual write to a separate source file |
| **Backward Compatibility** | Not required. The old `symlinks` table and the `.links/` directory are dropped; `data/` content is preserved |

---

## 7. Preview and Edit Feature Details

### 7.1 Feature Description

In the Browse tab of the repository detail page, users select a node in the `data/` tree to:
- **Plain text files**: View contents (read-only preview) and switch to edit mode to modify and save
- **Markdown files**: Toggle between rendered preview mode and raw text edit mode, then save
- **Binary files**: Only display file type information, not editable

### 7.2 Operation Target Description

| Operation | Target | Description |
|------|------|------|
| Preview (Read) | `data/<repo_path>` | Read the entry's content and display it |
| Edit (Save) | `data/<repo_path>` | Write in place. Every link of that entry reflects the change immediately, because each one is a symlink to that same file |
| Backup | `data/` | `git add -A` captures the edit — no incremental sync step exists |

### 7.3 Relationship with Backup

Editing does not automatically trigger a backup. The change sits in `data/` as an uncommitted working-tree modification and is captured by the next manual or scheduled backup. The Backup tab shows the uncommitted change count as the "there is an update" indicator. This is a reasonable design — editing is an independent action, and the backup timing stays under user control.

Because the same file backs every link, editing through any link or through the Browse tab is the same operation on the same content.

---

**Document Version**: v2.1  
**Status**: Confirmed  
**Date Prepared**: 2026-07-16 (v2.1 revised 2026-09-18 — links carry no `in`/`out` type; the cardinality rule is dropped; see §3.2, §3.3, §5)
