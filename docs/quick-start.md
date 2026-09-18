# Backup Manager - Quick Start Guide

> 🇨🇳 [中文文档](zh/quick-start.md)

## Overview

Backup Manager is a visual management tool for file/directory aggregated backup. Based on Git's reverse tracking mode (whitelist mechanism), it allows users to manage backups intuitively by "specifying what to back up" rather than "what to exclude".

## Dashboard

The main interface shows the status and basic information of all backup repositories.

![Dashboard](assets/repository-dashboard.png)

- View all backup repositories
- View repository status (active/inactive)
- Quick actions (Open, Delete)
- Create a new repository

## Repository Management

### Creating a Repository

1. Click the "+ Create Repository" button in the top right of the Dashboard
2. Enter the repository name and path
3. Configure basic settings

### Repository Detail View

After opening a repository, you can see four main tabs: **Browse**, **Entries**, **Backup**, **Config**.

## Browse Tab

The Browse tab shows the repository's real content under `data/`, and lets you preview and edit it.

![Browse Tab](assets/preview.png)

### Features:
- Browse the `data/` tree
- Each node carries a badge: is an entry / not backed up / has link drift
- Preview file content
- Click "Edit" to edit, "Save" to write in place
- View file metadata

### File Operations:
1. Select a file in the left tree view
2. Preview the file content on the right
3. Click "Edit" to enter edit mode
4. Click "Save" — because every link is a symlink to this same file, all of the entry's local paths update instantly

## Entries Tab

Every backed-up file or directory is an **entry**. An **entry** is a whitelist member: its existence in the manifest is what makes it backed up. A **link** binds a local path to that entry as a symlink into `data/<repo_path>`. An entry may have **0..N links**, and all of them are **completely equal** — same target, same semantics.

An entry with no links is perfectly valid: the content is in the repository, there is just no local view of it yet.

```
   ● in   ~/.config/opencode/opencode.json   MacBook Pro   [tracked]
   ● out  ~/Desktop/opencode.json            MacBook Pro   [track]  [remove]
   ○ out  ~/work/opencode/opencode.json      MacBook-Pro-2  other device
```

### Features:
- Entries grouped by `repo_path`, expandable to show every link
- Per-link state: `ok` / `missing` / `wrong_target` / `replaced` / `dangling` / `occupied`
- Link status per device, with which link is currently tracked
- Consistency audit with one-click repair

### Creating an Entry (adopt):

Click "+ New Entry" in the Entries tab:

![Add Entry Dialog](assets/add-symlink.jpeg)

1. **Source Path**: Enter or browse to the file or directory you want to back up (e.g. `~/.config/opencode/opencode.json`)
2. **Repo Path**: the logical path inside the repository (e.g. `opencode/opencode.json`). It defaults to the file name and must not overlap another entry's path.
3. Read the warning: **the content will be moved into the repository and this location replaced by a symlink.**
4. Click **Create**. The original location now holds the entry's first link.

### Distributing an Entry (add an out link):

Select an entry → "Add Link" → choose a local path. A symlink to `data/<repo_path>` is created there; nothing is copied. The same entry may have several links, on this machine or on others, and `Add Link` is available even for an entry that has none.

### All Links Are Equal:

There is no "tracked" link and nothing to switch: every link points at the same `data/<repo_path>`, so editing through one is the same as editing through another. The UI simply opens the first link that lives on the current device.

### Removing:

| Action | Effect |
|------|------|
| Remove a link | Deletes just that local symlink. `data/` keeps the content |
| `unlink` | Removes only this device's symlinks; the entry and its content stay |
| `move_back` | Moves the content back to a chosen local path, then removes the entry |
| `purge` | Deletes the content as well. Requires typing the `repo_path`; the previous commit can restore it |

## Multi-Device

Devices, entries and links are stored inside the repository in `.backup-manager/manifest.json`, tracked by Git. That is what makes multi-device work: the database is per machine, but the manifest travels with `git clone` / `git push`.

![Devices](assets/repository-dashboard.png)

### Bringing a Repository onto a New Machine:
1. Clone the repository (or point a new repo entry at the existing directory)
2. Open it — the current machine's device is registered automatically, and every device's links are listed
3. Click **Apply** — a dry-run plan appears (create / repair / skip / conflict / orphan)
4. Confirm. Missing links are created, drifted ones repaired, occupied paths are only reported
5. To put the files somewhere else, use **Bulk Link**: pick entries plus a local root directory

### Handing an Entry Over:
1. On the new machine, add a link wherever you want the files
2. Delete or detach the old device — nothing needs to be promoted: the new link is already the only local view

### Consistency Rules

The system refuses anything that would break entry-level consistency:

- A tracked directory forbids a link to a single file inside it
- Entries never overlap — `docs` and `docs/vendor` cannot both be entries
- A link's local path may not sit inside a directory entry's local path

Choose a non-overlapping repo path instead (e.g. `projects/vendor` rather than `docs/vendor`).

## Backup Tab

The Backup tab displays backup history and backup control buttons.

![Backup Tab](assets/backup.png)

### Features:
- View the last backup time
- View the total number of backups
- Monitor backup status
- Execute backup operations

### Backup Controls:
- **Trigger Backup**: Manually trigger a backup operation
- **Push to Remote**: Push committed changes to the remote repository
- **Force Push**: Force push (use with caution)

### Rollback:
- Select a commit from backup history to view changed files
- Choose specific files or roll back all files to the historical version
- Preview file contents at a specific commit before restoring
- Rollback overwrites `data/` (requires user confirmation). Because every link is a symlink into `data/`, all local paths reflect the rollback immediately — no separate sync step
- The tab also shows the number of uncommitted changes under `data/` as the "there is an update" indicator

### Backup History:
- View the commit hash
- View the commit author
- View the commit date
- View the commit message

## Config Tab

The Config tab manages repository configuration settings.

![Config Tab](assets/git-remote-config.png)

### Configuration Options:
- **Remote URL**: Git remote repository address
- **Branch**: Target branch for backup
- **Git User Name**: Commit author name
- **Git User Email**: Commit author email
- **Automatic Backup**: Enable/disable scheduled automatic backup

## Git Authentication & Danger Zone

Configure Git authentication information, and the danger zone.

![Git Authentication & Danger Zone](assets/git-auth-config.png)

### Authentication Types:
- **SSH Key**: Authenticate using an SSH private key
- **HTTPS**: Authenticate using username/password

### SSH Key Configuration:
1. Select "SSH Key" from the Authentication Type dropdown
2. Enter the SSH private key path (e.g. `~/.ssh/id_ed25519`)
3. Click "Save Authentication"

### Clear Authentication:
- Click "Clear" to delete saved authentication information

### Danger Zone ⚠️
- **Delete Repository**: Remove the repository from database records. All filesystem data (the manifest, backup data, Git history) is preserved and can be recovered by re-creating a repo pointing to the same directory. Scheduled tasks are unregistered.
- **Back to Dashboard**: Return to the repository list

## Getting Started Workflow

1. **Install & Run**: Download and start Backup Manager — a system tray icon appears
2. **Open UI**: Click the tray icon and select "Open UI" to open the web interface
3. **Create Repository**: Set up your first backup repository
4. **Create Entries**: Specify the files/directories to back up — their content moves into the repository and the original locations become their first links
5. **Distribute (optional)**: Add more links to make the same content available at further local paths
6. **Configure Git**: Set up remote repository and authentication information
7. **Run Backup**: Execute the first backup
8. **Monitor Status**: View backup status and history
9. **On another machine (optional)**: Clone the repo, open it, and click **Apply** to recreate that machine's links

## Best Practices

- Start with a few important files
- Prefer tracking a **directory** rather than an individual file when an application manages that file itself: apps that write config atomically (temp file + rename) replace the symlink with a real file. The UI flags this as `replaced` and offers one-click **Re-adopt**
- Keep repository paths non-overlapping so entries stay unambiguous
- Use `unlink` instead of `purge` when you only want to stop tracking on this machine
- Use meaningful commit messages
- Configure automatic backup for critical data
- Regularly verify backup integrity
- Safely store authentication credentials

## Troubleshooting

### Common Issues:
- **Backup Failed**: Check Git configuration and authentication information
- **A Link Shows `missing`**: The local symlink was deleted. Click **Apply** to recreate it — the content in `data/` is safe
- **A Link Shows `replaced`**: Something replaced the symlink with a real file. Use **Re-adopt** to move the new content into the repository and restore the link
- **A Link Shows `dangling`**: The content is missing from the repository. Restore it from Git history via the Backup tab, or remove the link
- **"Entry overlaps another entry"**: Two entries cannot nest. Pick a non-overlapping repo path (e.g. `projects/vendor` instead of `docs/vendor`)
- **"Local path is inside a directory entry"**: A link may not point inside a tracked directory. Move the target outside it, or track it as its own entry
- **An entry shows 0 links**: Legal — the content is in the repository with no local view. Click **Add Link** to bind it to a local path
- **You removed the last link**: No content is lost. Add a link again, or use an entry-level action (`unlink` / `move_back` / `purge`) to stop tracking the content itself
- **Remote Push Failed**: Ensure the remote repository exists and credentials are correct

### Getting Help:
- Check the application logs for detailed error information
- Ensure all dependencies are properly installed
- Ensure file permissions are correct
