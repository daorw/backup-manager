export type AppLanguage = 'en' | 'zh-CN';

export interface AppSettings {
  language: AppLanguage;
}

export type BackupRepoStatus = 'active' | 'error' | 'backing_up';

export interface BackupRepo {
  id: string;
  name: string;
  path: string;
  created_at: string;
  updated_at: string;
  last_backup_at: string | null;
  status: BackupRepoStatus;
  git_initialized: boolean;
  has_remote: boolean;
  // 配置字段 —— 后端在仓库响应里平铺返回
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

// ── 条目 / 链接 / 设备 ─────────────────────────────────────────────────

/** 条目类型。 */
export type EntryKind = 'file' | 'dir';

/** 链接在本机的实际状态（后端按需计算）。 */
export type LinkState =
  | 'ok'
  | 'missing'
  | 'wrong_target'
  | 'replaced'
  | 'dangling'
  | 'occupied'
  | 'disabled'
  | 'not_current';

/** 一条链接：把本机路径绑定到条目。所有链接完全等价，不区分 in/out。 */
export interface Link {
  id: string;
  entry_id: string;
  device: string;
  device_name?: string;
  local_path: string;
  enabled: boolean;
  /** 派生字段：是否属于当前设备。 */
  is_current: boolean;
  state: LinkState;
  state_note?: string;
  created_at: string;
}

export interface Entry {
  id: string;
  repo_path: string;
  kind: EntryKind;
  created_at: string;
  links: Link[];
}

export interface Device {
  fingerprint: string;
  name: string;
  hostname?: string;
  os?: string;
  is_current: boolean;
  last_seen_at?: string | null;
  link_count: number;
}

export interface CurrentDeviceInfo {
  fingerprint: string;
  hostname: string;
  os: string;
  name: string;
}

export interface AdoptRequest {
  local_path: string;
  repo_path?: string;
  follow_symlinks?: boolean;
}

export interface AddLinkRequest {
  local_path: string;
  device?: string;
}

export interface BulkLinkRequest {
  local_root: string;
  entry_ids?: string[];
}

/** 设备卸载模式。 */
export type DetachMode = 'unlink' | 'keep';

export interface DetachResult {
  device: string;
  mode: DetachMode;
  removed: ApplyAction[];
  completed_at: string;
}

export type ApplyActionName = 'create' | 'repair' | 'skip' | 'conflict' | 'orphan';

export interface ApplyAction {
  entry_id: string;
  link_id: string;
  repo_path: string;
  local_path: string;
  action: ApplyActionName;
  reason?: string;
}

export interface ApplyResult {
  device: string;
  created: ApplyAction[];
  repaired: ApplyAction[];
  skipped: ApplyAction[];
  conflicts: ApplyAction[];
  orphans: ApplyAction[];
  dry_run: boolean;
  completed_at: string;
}

// ── 一致性巡检 ────────────────────────────────────────────────────────

export type FindingSeverity = 'error' | 'warning';

export interface AuditFinding {
  code: string;
  severity: FindingSeverity;
  repo_path?: string;
  link_id?: string;
  local_path?: string;
  /** 是否可由一键修复处理。 */
  repairable: boolean;
  message: string;
}

export interface AuditResult {
  repo_id: string;
  device: string;
  clean: boolean;
  errors: number;
  warnings: number;
  entry_count: number;
  link_count: number;
  findings: AuditFinding[];
  audited_at: string;
}

export interface RepairResult {
  repo_id: string;
  repaired: ApplyAction[];
  /** 无法自动修复的结论。 */
  skipped: AuditFinding[];
  repaired_count: number;
  remaining_errors: number;
  completed_at: string;
}

// ── 仓库内容 ──────────────────────────────────────────────────────────

export interface ContentEntry {
  name: string;
  /** 相对 data/ 的斜杠路径。 */
  path: string;
  type: 'file' | 'directory';
  size?: number;
  modified_at?: string;
}

export interface PreviewResult {
  content?: string;
  mime_type: string;
  size: number;
  text: boolean;
  truncated?: boolean;
}

export interface SaveFileRequest {
  path: string;
  content: string;
}

export interface SaveFileResult {
  file_size: number;
  modified_at: string;
}

export interface ChangesResult {
  dirty: boolean;
  changes: string[];
}

// ── 本机文件浏览 ──────────────────────────────────────────────────────

export interface BrowseEntry {
  name: string;
  path: string;
  type: 'file' | 'directory';
  size: number;
  modified_at: string;
}

// ── 备份 / 回滚 ───────────────────────────────────────────────────────

export interface BackupResult {
  repo_id: string;
  completed_at: string;
  files_changed: number;
  files_added: number;
  files_removed: number;
  commit_hash?: string;
  commit_message?: string;
}

export interface CommitEntry {
  hash: string;
  author: string;
  email: string;
  date: string;
  message: string;
}

export interface CommitFileChange {
  change_type: string;
  relative_path: string;
}

export interface RollbackRequest {
  commit_hash: string;
  /** 为空表示回滚该提交的全部变更。 */
  paths?: string[];
}

export interface RollbackFailure {
  relative_path: string;
  error: string;
}

export interface CommitFileContent {
  content?: string;
  mime_type: string;
  size: number;
  text: boolean;
  truncated?: boolean;
}

export interface FileRestoreResult {
  relative_path: string;
  success: boolean;
  restored_at: string;
}

export interface RollbackResult {
  repo_id: string;
  commit_hash: string;
  total: number;
  success: number;
  failed: number;
  failures?: RollbackFailure[];
  completed_at: string;
}

// ── Git 认证 ──────────────────────────────────────────────────────────

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
