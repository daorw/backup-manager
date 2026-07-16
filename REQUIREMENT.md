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

**Symlink Aggregation**: Inside a backup repository (repo) created by the user, symlinks are used to "point to" source files, centrally managed under the `.links/` directory. During actual backup, the system syncs the content of the source files pointed to by the symlinks to the `data/` directory, which is then version-controlled via Git.

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

### 3.2 Symlink Management (.links/ directory)

| ID | Feature | Description | Priority |
|----|------|------|--------|
| FR-5 | Add Symlink | User selects source files/directories via UI, automatically creates corresponding symlinks under `.links/` and copies source files to `data/` | P0 |
| FR-6 | View Symlink List | Display all symlinks under `.links/` directory in a tree structure | P0 |
| FR-7 | Delete Symlink | Delete a specified symlink and synchronously remove the corresponding file in `data/` | P0 |
| FR-8 | Modify Symlink Target | Change the source file/directory path that the symlink points to | P1 |
| FR-9 | Batch Import Symlinks | Support batch selection of multiple source files/directories to add symlinks, with automatic rollback on partial failure | P1 |
| FR-10 | Clean Up Deleted Source Files | Detect symlinks whose source files have been deleted, synchronously clean up corresponding files in `data/` and generate a Git commit | P2 |
| FR-11 | Nested Symlinks | Add child symlinks within directory-type symlinks, with cycle detection and depth limiting | P2 |

### 3.3 File Preview and Editing

| ID | Feature | Description | Priority |
|----|------|------|--------|
| FR-12 | Plain Text File Preview and Edit | View and edit plain text source file contents in the UI (e.g., .txt, .log, .json, .yaml, .py, etc.), with save-to-source-file support. The operation targets the source file pointed to by the symlink. | P0 |
| FR-13 | Markdown Rendered Preview and Edit | Render and display Markdown source files (.md) using react-markdown + remark-gfm. Supports toggling between edit mode and preview mode, saving edits to the source file. The operation targets the source file pointed to by the symlink. Local references (images/docs) are rendered by browser default without path rewriting. | P0 |
| FR-14 | Binary File Identification | Display file type information and size for non-text files, without attempting to preview content | P2 |

### 3.4 Backup Execution

| ID | Feature | Description | Priority |
|----|------|------|--------|
| FR-15 | Execute Backup | Manually trigger a backup operation: incremental detection (mtime+size) → sync changes to `data/` → `git add` → `git commit` → (optional) `git push`, with progress display | P0 |
| FR-16 | Scheduled/Auto Backup | Automatically execute backups at scheduled times based on configured cron expression, auto-load enabled repositories on application startup | P1 |
| FR-17 | View Backup History | View the repository's Git commit history with pagination support | P1 |
| FR-18 | Source File Rollback | Select a historical commit version and restore source files to the version in the specified commit. Supports full rollback, selective rollback by symlink, and single-file restore. Commit file content can be previewed before rollback. | P1 |

### 3.5 Configuration Management

| ID | Feature | Description | Priority |
|----|------|------|--------|
| FR-19 | Git Remote Repository Config | Visually configure the Git remote repository URL and target branch | P0 |
| FR-20 | Git Authentication Config | Configure authentication information required for Git operations (SSH private key or HTTPS username/password), stored encrypted in SQLite | P1 |
| FR-21 | Application Global Settings | Application-level basic settings (port number, theme, whether to auto-open browser) | P1 |

### 3.6 System Management

| ID | Feature | Description | Priority |
|----|------|------|--------|
| FR-22 | Application Start/Stop | One-click start and stop of the entire application, auto-open browser after startup. System tray icon provides "Open UI", "Start/Stop Server", and "Quit" controls. | P0 |
| FR-23 | Local File Browser | Safely browse the local filesystem for selecting symlink source files and previewing files, limited to the user's home directory and repo root directory | P0 |
| FR-24 | Health Check | Provide `/health` endpoint returning application running status, startup time, and version information | P2 |

---

## 4. Non-Functional Requirements

| ID | Requirement | Description |
|----|------|------|
| NFR-1 | **Unified Frontend and Backend Architecture** | Frontend UI and backend service integrated into a single application, running as a single process |
| NFR-2 | **Cross-Platform Support** | Support at least macOS and Linux |
| NFR-3 | **Responsive UI** | Interface adapts to different screen sizes |
| NFR-4 | **Security** | Require confirmation before deleting symlinks and repositories; path safety checks required when previewing and editing source files |
| NFR-5 | **Data Consistency** | `.links/` and `data/` directory structures must be mirror-consistent |
| NFR-6 | **Backup Atomicity** | Failed backups should have clear prompts and error status |
| NFR-7 | **Usability** | Core features should be completable within 3 clicks |
| NFR-8 | **Startup Behavior** | Auto-open browser after startup |
| NFR-9 | **Path Safety** | Four-layer path validation (Clean→Abs→EvalSymlinks→Prefix) to prevent path traversal |
| NFR-10 | **Concurrency Safety** | Independent mutex per repository to prevent concurrent backups; preview/edit API rate-limited (max 5 concurrent) |
| NFR-11 | **Sensitive Information Encryption** | SSH private keys and HTTPS passwords encrypted with AES-256-GCM before storage in SQLite, key file permissions 0600 |

---

## 5. Core Concepts / Data Model

### 5.1 Directory Structure

```
<repo-root>/
├── .links/              # Symlink directory, structure mirrors data/ exactly
│   ├── documents/
│   │   ├── report.docx -> /Users/xxx/Documents/report.docx
│   │   └── notes.txt -> /Users/xxx/Documents/notes.txt
│   └── config/
│       └── settings.json -> /Users/xxx/.config/settings.json
├── data/                # Actual backup data directory, structure mirrors .links/ exactly
│   ├── documents/
│   │   ├── report.docx
│   │   └── notes.txt
│   └── config/
│       └── settings.json
└── .git/                # Git repository
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
└── symlinks: Symlink[]

Symlink
├── id: string
├── repoId: string
├── relativePath: string   # Relative path under .links/ (including filename)
├── targetPath: string     # Absolute path to the source file/directory (target for preview and edit operations)
├── type: 'file' | 'directory'
├── size: number           # Source file size
├── modifiedAt: timestamp  # Source file's last modification time
└── createdAt: timestamp
```

### 5.3 Database Schema

4 core tables with foreign key cascading deletes:

```sql
repos         — Repository: id, name, path, created_at, updated_at, last_backup_at, status
repo_configs  — Config: repo_id(FK), remote_url, branch, auto_backup, auto_backup_interval, git_user_name, git_user_email
repo_auths    — Auth: repo_id(FK), auth_type, ssh_private_key(BLOB), ssh_private_key_path, username, password_encrypted(BLOB)
symlinks      — Symlink: id, repo_id(FK), relative_path(UNIQUE), target_path, type, file_size, modified_at, created_at
```

---

## 6. Key Decisions Confirmed by User

| Issue | Decision |
|------|------|
| Git Remote Repository | `git push` is optional. When remote is not configured, only local commits are made without push |
| Strategy After Source File Deletion | Corresponding files in `data/` are deleted synchronously, and a Git commit is generated |
| File Conflict | If two symlinks point to the same source file, no special handling is needed — proceed normally |
| Backup Granularity | Incremental sync, only sync modified files (compare mtime+size) |
| Frontend Technology Stack | React 18 + TypeScript + Vite + Ant Design 5 |
| Startup Behavior | Auto-open browser after startup |
| Markdown Images | Support local image display in Markdown |
| Multiple Repositories | Support parallel management of multiple repositories |
| Backend Framework | Gin (Go lightweight high-performance HTTP framework) |
| Database | SQLite (pure Go implementation via modernc.org/sqlite, no CGO required) |
| Authentication Encryption | SSH private keys and HTTPS passwords stored encrypted with AES-256-GCM |
| Source File Rollback | Rollback overwrites source files (requires user confirmation), displays list of changed files before rollback |
| Repository Deletion | Deleting a repository only removes database records and scheduled tasks, preserving filesystem data without loss |
| **Preview/Edit Target** | Preview and edit operations target the **source file pointed to by the symlink** (target_path), not the copy in the data/ directory |

---

## 7. Preview and Edit Feature Details

### 7.1 Feature Description

In the Preview tab of the repository detail page, users can select symlink files through the file tree to:
- **Plain text files**: View file contents (read-only preview) and switch to edit mode to modify and save to the source file
- **Markdown files**: Toggle between rendered preview mode and raw text edit mode, save changes to the source file after editing
- **Binary files**: Only display file type information, not editable

### 7.2 Operation Target Description

| Operation | Target | Description |
|------|------|------|
| Preview (Read) | Source file pointed to by the symlink (`symlink.target_path`) | Read the current content of the source file and display it to the user |
| Edit (Save) | Source file pointed to by the symlink (`symlink.target_path`) | Write the edited content back to the source file |
| Backup | Copy in the data/ directory | Git version management operates on the copy in data/, unrelated to preview/edit |

### 7.3 Relationship with Backup

Editing the source file does not automatically trigger a backup. The user's modifications to the source file will be detected by incremental checking (mtime+size changes) during the next manual or scheduled backup, synced to the `data/` directory, and then tracked by Git version management. This is a reasonable design — editing source files is an independent action, and the backup timing is controlled by the user.

---

**Document Version**: v1.3  
**Status**: Confirmed  
**Date Prepared**: 2026-07-16
