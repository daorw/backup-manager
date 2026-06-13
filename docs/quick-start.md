# Backup Manager - Quick Start Guide

## Overview

Backup Manager is a file/directory aggregation backup visualization management tool based on Git's reverse tracking mechanism. It allows users to specify which files/directories need to be backed up through a whitelist approach, using symlinks for aggregated management.

## Dashboard

The main dashboard shows all your backup repositories with their status and basic information.

![Dashboard](assets/dashboard.png)

- View all backup repositories
- See repository status (active/inactive)
- Access quick actions (Open, Delete)
- Create new repositories

## Repository Management

### Creating a Repository

1. Click the "+ Create Repository" button on the dashboard
2. Provide a repository name and path
3. Configure basic settings

### Repository Detail View

Once you open a repository, you'll see four main tabs:

![Repository Detail](assets/repository-detail.png)

## Symlinks Tab

The Symlinks tab shows all files and directories that are backed up via symlinks.

![Symlinks Tab](assets/symlinks-tab.png)

### Features:
- View symlink hierarchy
- Add new symlinks with "+ Add Symlink" button
- Refresh symlink list
- See original file paths

### Adding a Symlink:
1. Click "+ Add Symlink"
2. Browse or enter the target file/directory path
3. Confirm the addition

## Preview Tab

The Preview tab allows you to view and edit backed-up files directly.

![Preview Tab](assets/preview-tab.png)

### Features:
- Browse file structure
- Preview file contents
- Edit files with the Edit button
- Save changes with the Save button
- View file metadata

### File Operations:
1. Select a file from the tree view
2. Preview its content
3. Click "Edit" to modify
4. Click "Save" to persist changes

## Backup Tab

The Backup tab shows backup history and provides backup controls.

![Backup Tab](assets/backup-tab.png)

### Features:
- View last backup time
- See total backup count
- Monitor backup status
- Access backup controls

### Backup Controls:
- **Git Init**: Initialize Git repository
- **Trigger Backup**: Manually start a backup
- **Push to Remote**: Push commits to remote repository
- **Force Push**: Force push to remote (use with caution)

### Backup History:
- View commit hashes
- See commit authors
- Check commit dates
- Read commit messages

## Config Tab

The Config tab manages repository configuration settings.

![Config Tab](assets/config-tab.png)

### Configuration Options:
- **Remote URL**: Git remote repository URL
- **Branch**: Target branch for backups
- **Git User Name**: Commit author name
- **Git User Email**: Commit author email
- **Automatic Backup**: Enable/disable scheduled backups

## Git Authentication

Configure Git authentication for remote repositories.

![Git Authentication](assets/git-authentication.png)

### Authentication Types:
- **SSH Key**: Use SSH private key for authentication
- **HTTPS**: Use username/password for HTTPS repositories

### SSH Key Configuration:
1. Select "SSH Key" from Authentication Type
2. Enter SSH private key path (e.g., `~/.ssh/id_ed25519`)
3. Click "Save Authentication"

### Clearing Authentication:
- Click "Clear" to remove stored authentication

## Danger Zone

⚠️ **Warning**: These actions are irreversible!

![Danger Zone](assets/danger-zone.png)

### Delete Repository:
- Permanently removes all data
- Includes symlinks, backup data, and Git history
- Cannot be undone

### Back to Dashboard:
- Returns to the main repository list

## Getting Started Workflow

1. **Install & Run**: Download and start Backup Manager
2. **Create Repository**: Set up your first backup repository
3. **Add Symlinks**: Specify which files/directories to back up
4. **Configure Git**: Set up remote repository and authentication
5. **Run Backup**: Execute your first backup
6. **Monitor**: Check backup status and history

## Best Practices

- Start with a small set of important files
- Use meaningful commit messages
- Configure automatic backups for critical data
- Regularly verify backup integrity
- Keep authentication credentials secure

## Troubleshooting

### Common Issues:
- **Backup fails**: Check Git configuration and authentication
- **Symlinks not showing**: Verify file permissions and paths
- **Remote push fails**: Ensure remote repository exists and credentials are correct

### Support:
- Check application logs for detailed error messages
- Verify all dependencies are installed
- Ensure proper file permissions