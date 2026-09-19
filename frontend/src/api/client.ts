import axios from 'axios';
import i18n from '../i18n';
import type {
  AppSettings,
  BackupRepo,
  CreateRepoRequest,
  UpdateConfigRequest,
  Entry,
  Device,
  CurrentDeviceInfo,
  AdoptRequest,
  AddLinkRequest,
  BulkLinkRequest,
  ApplyResult,
  DetachMode,
  DetachResult,
  AuditResult,
  RepairResult,
  ContentEntry,
  BrowseEntry,
  PreviewResult,
  SaveFileRequest,
  SaveFileResult,
  ChangesResult,
  BackupResult,
  CommitEntry,
  GitAuth,
  SetAuthRequest,
  CommitFileChange,
  CommitFileContent,
  FileRestoreResult,
  RollbackRequest,
  RollbackResult,
} from '../types';

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// 响应拦截器：解开后端的 { data: ... } 包装
api.interceptors.response.use(
  (response) => {
    if (response.data && typeof response.data === 'object' && 'data' in response.data) {
      response.data = response.data.data;
    }
    return response;
  },
  (error) => {
    const message = error.response?.data?.error || i18n.t('errors.requestFailed');
    return Promise.reject(new Error(message));
  }
);

// ── 应用设置 ──────────────────────────────────────────────────────────

export async function fetchSettings(): Promise<AppSettings> {
  const { data } = await api.get<AppSettings>('/settings');
  return data;
}

export async function updateSettings(settings: AppSettings): Promise<AppSettings> {
  const { data } = await api.put<AppSettings>('/settings', settings);
  return data;
}

// ── 仓库 ──────────────────────────────────────────────────────────────

export async function fetchRepos(): Promise<BackupRepo[]> {
  const { data } = await api.get<BackupRepo[]>('/repos');
  return data;
}

export async function fetchRepo(id: string): Promise<BackupRepo> {
  const { data } = await api.get<BackupRepo>(`/repos/${id}`);
  return data;
}

export async function createRepo(req: CreateRepoRequest): Promise<BackupRepo> {
  const { data } = await api.post<BackupRepo>('/repos', req);
  return data;
}

export async function deleteRepo(id: string): Promise<void> {
  await api.delete(`/repos/${id}`);
}

export async function updateRepoConfig(id: string, config: UpdateConfigRequest): Promise<void> {
  await api.put(`/repos/${id}/config`, config);
}

export async function gitInitRepo(repoId: string): Promise<void> {
  await api.post(`/repos/${repoId}/git-init`);
}

// ── 条目 ──────────────────────────────────────────────────────────────

export async function fetchEntries(repoId: string): Promise<Entry[]> {
  const { data } = await api.get<Entry[]>(`/repos/${repoId}/entries`);
  return data;
}

/** 创建条目：内容移入仓库，原位置替换为指向它的软链接（条目的第一条链接）。 */
export async function adoptEntry(repoId: string, req: AdoptRequest): Promise<Entry> {
  const { data } = await api.post<Entry>(`/repos/${repoId}/entries/adopt`, req);
  return data;
}

export type EntryRemoveMode = 'unlink' | 'move_back' | 'purge';

export async function removeEntry(
  repoId: string,
  entryId: string,
  mode: EntryRemoveMode,
  linkId?: string
): Promise<void> {
  await api.delete(`/repos/${repoId}/entries/${entryId}`, {
    params: { mode, link_id: linkId },
  });
}

// ── 链接 ──────────────────────────────────────────────────────────────

export async function addLink(
  repoId: string,
  entryId: string,
  req: AddLinkRequest
): Promise<Entry> {
  const { data } = await api.post<Entry>(`/repos/${repoId}/entries/${entryId}/links`, req);
  return data;
}

/** 把一个本地根目录下的多个条目批量链接到本机。 */
export async function bulkLink(repoId: string, req: BulkLinkRequest): Promise<Entry[]> {
  const { data } = await api.post<Entry[]>(`/repos/${repoId}/links/bulk`, req);
  return data;
}

export async function repairLink(
  repoId: string,
  entryId: string,
  linkId: string
): Promise<Entry> {
  const { data } = await api.post<Entry>(
    `/repos/${repoId}/entries/${entryId}/links/${linkId}/repair`
  );
  return data;
}

/** replaced 状态：把本机的真实文件/目录移入仓库，并恢复软链接。 */
export async function readoptLink(
  repoId: string,
  entryId: string,
  linkId: string
): Promise<Entry> {
  const { data } = await api.post<Entry>(
    `/repos/${repoId}/entries/${entryId}/links/${linkId}/readopt`
  );
  return data;
}

export async function removeLink(
  repoId: string,
  entryId: string,
  linkId: string
): Promise<Entry> {
  const { data } = await api.post<Entry>(
    `/repos/${repoId}/entries/${entryId}/links/${linkId}/remove`
  );
  return data;
}

// ── 一致性巡检 ────────────────────────────────────────────────────────

/** 巡检仓库：清单不变量 + 本机链接实际状态。 */
export async function fetchConsistency(repoId: string): Promise<AuditResult> {
  const { data } = await api.get<AuditResult>(`/repos/${repoId}/consistency`);
  return data;
}

/** 收敛所有可自动修复的问题。 */
export async function repairConsistency(repoId: string): Promise<RepairResult> {
  const { data } = await api.post<RepairResult>(`/repos/${repoId}/consistency/repair`);
  return data;
}

// ── 设备 ──────────────────────────────────────────────────────────────

export async function fetchCurrentDevice(): Promise<CurrentDeviceInfo> {
  const { data } = await api.get<CurrentDeviceInfo>('/devices/current');
  return data;
}

export async function fetchDevices(repoId: string): Promise<Device[]> {
  const { data } = await api.get<Device[]>(`/repos/${repoId}/devices`);
  return data;
}

export async function registerDevice(repoId: string, name?: string): Promise<Device> {
  const { data } = await api.post<Device>(`/repos/${repoId}/devices`, { name });
  return data;
}

export async function renameDevice(
  repoId: string,
  fingerprint: string,
  name: string
): Promise<void> {
  await api.patch(`/repos/${repoId}/devices/${fingerprint}`, { name });
}

export async function deleteDevice(repoId: string, fingerprint: string): Promise<void> {
  await api.delete(`/repos/${repoId}/devices/${fingerprint}`);
}

/** 卸载本机：unlink 删除本机软链接，keep 只停止管理。 */
export async function detachDevice(
  repoId: string,
  fingerprint: string,
  mode: DetachMode
): Promise<DetachResult> {
  const { data } = await api.post<DetachResult>(
    `/repos/${repoId}/devices/${fingerprint}/detach`,
    { mode }
  );
  return data;
}

/** 让本机与清单收敛。dry_run 时只返回计划。 */
export async function applyDevice(
  repoId: string,
  fingerprint: string,
  dryRun = false
): Promise<ApplyResult> {
  const { data } = await api.post<ApplyResult>(
    `/repos/${repoId}/devices/${fingerprint}/apply`,
    { dry_run: dryRun }
  );
  return data;
}

// ── 仓库内容 ──────────────────────────────────────────────────────────

export async function fetchTree(
  repoId: string,
  path?: string,
  includeHidden = false
): Promise<ContentEntry[]> {
  const { data } = await api.get<ContentEntry[]>(`/repos/${repoId}/tree`, {
    params: { path: path || '', include_hidden: includeHidden || undefined },
  });
  return data;
}

export async function previewFile(repoId: string, path: string): Promise<PreviewResult> {
  const { data } = await api.get<PreviewResult>(`/repos/${repoId}/preview`, {
    params: { path },
  });
  return data;
}

export async function saveFile(repoId: string, req: SaveFileRequest): Promise<SaveFileResult> {
  const { data } = await api.put<SaveFileResult>(`/repos/${repoId}/save`, req);
  return data;
}

export async function fetchChanges(repoId: string): Promise<ChangesResult> {
  const { data } = await api.get<ChangesResult>(`/repos/${repoId}/changes`);
  return data;
}

// ── 本机文件浏览 ──────────────────────────────────────────────────────

/** 浏览本机目录。无白名单限制：任意服务端可见路径均可浏览。 */
export async function browsePath(path: string, includeHidden = false): Promise<BrowseEntry[]> {
  const { data } = await api.get<BrowseEntry[]>('/browse', {
    params: { path, include_hidden: includeHidden },
  });
  return data;
}

/** 浏览对话框的默认起始目录（服务端进程的家目录）。 */
export async function fetchHomeDir(): Promise<string> {
  const { data } = await api.get<string>('/browse/home');
  return data;
}

// ── 备份 ──────────────────────────────────────────────────────────────

export async function triggerBackup(repoId: string, commitMessage?: string): Promise<BackupResult> {
  const body = commitMessage ? { commit_message: commitMessage } : {};
  const { data } = await api.post<BackupResult>(`/repos/${repoId}/backup`, body);
  return data;
}

export async function fetchBackupHistory(
  repoId: string,
  limit = 20,
  offset = 0
): Promise<CommitEntry[]> {
  const { data } = await api.get<CommitEntry[]>(`/repos/${repoId}/backup/history`, {
    params: { limit, offset },
  });
  return data;
}

export async function pushRepo(repoId: string, force = false): Promise<void> {
  await api.post(`/repos/${repoId}/push`, { force });
}

// ── 回滚 ──────────────────────────────────────────────────────────────

export async function fetchCommitChangedFiles(
  repoId: string,
  commitHash: string
): Promise<CommitFileChange[]> {
  const { data } = await api.get<CommitFileChange[]>(
    `/repos/${repoId}/commits/${commitHash}/changed-files`
  );
  return data;
}

export async function fetchCommitFileContent(
  repoId: string,
  commitHash: string,
  path: string
): Promise<CommitFileContent> {
  const { data } = await api.get<CommitFileContent>(
    `/repos/${repoId}/commits/${commitHash}/files`,
    { params: { path } }
  );
  return data;
}

export async function restoreCommitFile(
  repoId: string,
  commitHash: string,
  path: string
): Promise<FileRestoreResult> {
  const { data } = await api.post<FileRestoreResult>(
    `/repos/${repoId}/commits/${commitHash}/restore`,
    { path }
  );
  return data;
}

export async function rollbackFiles(
  repoId: string,
  req: RollbackRequest
): Promise<RollbackResult> {
  const { data } = await api.post<RollbackResult>(`/repos/${repoId}/rollback`, req);
  return data;
}

// ── Git 认证 ──────────────────────────────────────────────────────────

export async function fetchAuth(repoId: string): Promise<GitAuth> {
  const { data } = await api.get<GitAuth>(`/repos/${repoId}/auth`);
  return data;
}

export async function setAuth(repoId: string, req: SetAuthRequest): Promise<GitAuth> {
  const { data } = await api.put<GitAuth>(`/repos/${repoId}/auth`, req);
  return data;
}

export async function clearAuth(repoId: string): Promise<void> {
  await api.delete(`/repos/${repoId}/auth`);
}

// ── 系统 ──────────────────────────────────────────────────────────────

export async function healthCheck(): Promise<{ status: string }> {
  const { data } = await api.get<{ status: string }>('/health');
  return data;
}

export default api;
