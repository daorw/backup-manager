export type BackupRepoStatus = 'active' | 'error' | 'backing_up';

export interface BackupRepo {
  id: string;
  name: string;
  path: string;
  created_at: string;
  updated_at: string;
  last_backup_at: string | null;
  status: BackupRepoStatus;
  // Config fields — backend returns them flat in the repo response
  remote_url?: string;
  branch?: string;
  auto_backup?: boolean;
  auto_backup_interval?: string;
  git_user_name?: string;
  git_user_email?: string;
}

export interface CreateRepoRequest {
  name: string;
  path: string;
}

export interface UpdateConfigRequest {
  remote_url?: string;
  branch?: string;
  auto_backup?: boolean;
  auto_backup_interval?: string;
  git_user_name?: string;
  git_user_email?: string;
}

export interface Symlink {
  id: string;
  repo_id: string;
  relative_path: string;
  target_path: string;
  type: 'file' | 'directory';
  file_size: number;
  modified_at: string | null;
  created_at: string;
}

export interface CreateSymlinkRequest {
  target_path: string;
  relative_path: string;
}

export interface UpdateSymlinkRequest {
  target_path: string;
}

export interface BatchSymlinkRequest {
  targets: Array<{
    target_path: string;
    relative_path: string;
  }>;
}

export interface BrowseEntry {
  name: string;
  path: string;
  type: 'file' | 'directory';
  size: number;
  modified_at: string;
}

export interface PreviewResult {
  content?: string;
  mime_type: string;
  size: number;
  text: boolean;
  truncated?: boolean;
}

export interface BackupResult {
  repo_id: string;
  completed_at: string;
  files_changed: number;
  files_removed: number;
  commit_hash?: string;
  commit_message?: string;
  pushed: boolean;
}

export interface CommitEntry {
  hash: string;
  author: string;
  email: string;
  date: string;
  message: string;
}

export type GitAuthType = 'none' | 'ssh_key' | 'password';

export interface GitAuth {
  repo_id: string;
  auth_type: GitAuthType;
  ssh_private_key_path?: string;
  username?: string;
  updated_at: string;
}

export interface SetAuthRequest {
  auth_type: GitAuthType;
  ssh_private_key?: string;
  ssh_private_key_path?: string;
  username?: string;
  password?: string;
}

export interface SymlinkTreeNode {
  key: string;
  title: string;
  isLeaf: boolean;
  symlink?: Symlink;
  children?: SymlinkTreeNode[];
}

// Rollback types
export interface CommitFileChange {
  change_type: string;
  relative_path: string;
  symlink_id: string | null;
  symlink_type: string | null;
}

export interface RollbackRequest {
  commit_hash: string;
  symlink_ids?: string[];
}

export interface RollbackFailure {
  relative_path: string;
  error: string;
}

export interface RollbackResult {
  repo_id: string;
  commit_hash: string;
  total: number;
  success: number;
  skipped: number;
  failed: number;
  failures?: RollbackFailure[];
  completed_at: string;
}
