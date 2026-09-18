import { create } from 'zustand';
import type {
  BackupRepo,
  BackupResult,
  Entry,
  Device,
  CurrentDeviceInfo,
  AdoptRequest,
  AddLinkRequest,
  BulkLinkRequest,
  ApplyResult,
  AuditResult,
  RepairResult,
  GitAuth,
  CommitEntry,
  CommitFileChange,
  CommitFileContent,
  RollbackRequest,
  RollbackResult,
  FileRestoreResult,
} from '../types';
import * as api from '../api/client';

interface BackupProgressState {
  repo_id: string;
  status: 'running' | 'completed' | 'failed';
  message: string;
  progress: number;
  started_at: string;
}

interface AppState {
  repos: BackupRepo[];
  currentRepo: BackupRepo | null;
  entries: Entry[];
  devices: Device[];
  currentDevice: CurrentDeviceInfo | null;
  backupHistory: CommitEntry[];
  currentAuth: GitAuth | null;
  backupProgress: BackupProgressState | null;
  loading: boolean;
  error: string | null;

  // 回滚状态
  commitFilesByHash: Record<string, CommitFileChange[]>;
  rollbackResult: RollbackResult | null;
  rollbackLoading: boolean;

  // 提交文件预览状态
  commitFileContent: CommitFileContent | null;
  commitFileContentLoading: boolean;
  restoreFileLoading: boolean;

  fetchRepos: () => Promise<void>;
  fetchRepo: (id: string) => Promise<void>;
  createRepo: (name: string, path: string) => Promise<void>;
  deleteRepo: (id: string) => Promise<void>;
  updateRepoConfig: (id: string, config: Parameters<typeof api.updateRepoConfig>[1]) => Promise<void>;

  // 条目与链接
  fetchEntries: (repoId: string) => Promise<void>;
  adoptEntry: (repoId: string, req: AdoptRequest) => Promise<void>;
  addLink: (repoId: string, entryId: string, req: AddLinkRequest) => Promise<void>;
  bulkLink: (repoId: string, req: BulkLinkRequest) => Promise<void>;
  switchTrackedLink: (repoId: string, entryId: string, linkId: string) => Promise<void>;
  repairLink: (repoId: string, entryId: string, linkId: string) => Promise<void>;
  removeLink: (repoId: string, entryId: string, linkId: string) => Promise<void>;
  removeEntry: (
    repoId: string,
    entryId: string,
    mode: api.EntryRemoveMode,
    linkId?: string
  ) => Promise<void>;

  // 一致性巡检
  audit: AuditResult | null;
  auditLoading: boolean;
  runAudit: (repoId: string) => Promise<AuditResult>;
  repairConsistency: (repoId: string) => Promise<RepairResult>;

  // 设备
  fetchCurrentDevice: () => Promise<void>;
  fetchDevices: (repoId: string) => Promise<void>;
  registerDevice: (repoId: string, name?: string) => Promise<void>;
  renameDevice: (repoId: string, fingerprint: string, name: string) => Promise<void>;
  deleteDevice: (repoId: string, fingerprint: string) => Promise<void>;
  applyDevice: (repoId: string, fingerprint: string, dryRun?: boolean) => Promise<ApplyResult>;

  triggerBackup: (repoId: string, commitMessage?: string) => Promise<BackupResult | void>;
  pushRepo: (repoId: string, force?: boolean) => Promise<void>;
  gitInitRepo: (repoId: string) => Promise<void>;
  fetchBackupHistory: (repoId: string, limit?: number, offset?: number) => Promise<void>;

  fetchAuth: (repoId: string) => Promise<void>;
  setAuth: (repoId: string, auth: Parameters<typeof api.setAuth>[1]) => Promise<void>;
  clearAuth: (repoId: string) => Promise<void>;

  // 回滚
  fetchCommitFiles: (repoId: string, commitHash: string) => Promise<void>;
  rollbackFiles: (repoId: string, req: RollbackRequest) => Promise<RollbackResult>;
  clearRollbackResult: () => void;

  // 提交文件预览
  fetchCommitFileContent: (
    repoId: string,
    commitHash: string,
    path: string
  ) => Promise<CommitFileContent>;
  restoreCommitFile: (repoId: string, commitHash: string, path: string) => Promise<FileRestoreResult>;
  clearCommitFileContent: () => void;

  clearError: () => void;
}

export const useAppStore = create<AppState>((set, get) => ({
  repos: [],
  currentRepo: null,
  entries: [],
  devices: [],
  currentDevice: null,
  backupHistory: [],
  currentAuth: null,
  backupProgress: null,
  loading: false,
  error: null,

  commitFilesByHash: {},
  rollbackResult: null,
  rollbackLoading: false,

  commitFileContent: null,
  commitFileContentLoading: false,
  restoreFileLoading: false,

  clearError: () => set({ error: null }),

  fetchRepos: async () => {
    set({ loading: true, error: null });
    try {
      set({ repos: await api.fetchRepos(), loading: false });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to fetch repos'), loading: false });
    }
  },

  fetchRepo: async (id: string) => {
    set({ loading: true, error: null });
    try {
      set({ currentRepo: await api.fetchRepo(id), loading: false });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to fetch repo'), loading: false });
    }
  },

  createRepo: async (name: string, path: string) => {
    set({ loading: true, error: null });
    try {
      const repo = await api.createRepo({ name, path });
      set({ repos: [...get().repos, repo], loading: false });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to create repo'), loading: false });
      throw err;
    }
  },

  deleteRepo: async (id: string) => {
    set({ loading: true, error: null });
    try {
      await api.deleteRepo(id);
      set({ repos: get().repos.filter((r) => r.id !== id), loading: false });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to delete repo'), loading: false });
      throw err;
    }
  },

  updateRepoConfig: async (id, config) => {
    set({ error: null });
    try {
      await api.updateRepoConfig(id, config);
      const repo = await api.fetchRepo(id);
      set({ currentRepo: repo, repos: get().repos.map((r) => (r.id === id ? repo : r)) });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to update config') });
      throw err;
    }
  },

  // ── 条目与链接 ──────────────────────────────────────────────────────

  fetchEntries: async (repoId: string) => {
    set({ error: null });
    try {
      set({ entries: await api.fetchEntries(repoId) });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to fetch entries') });
    }
  },

  adoptEntry: async (repoId, req) => {
    set({ error: null });
    try {
      await api.adoptEntry(repoId, req);
      await get().fetchEntries(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to create entry') });
      throw err;
    }
  },

  addLink: async (repoId, entryId, req) => {
    set({ error: null });
    try {
      await api.addLink(repoId, entryId, req);
      await get().fetchEntries(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to add link') });
      throw err;
    }
  },

  bulkLink: async (repoId, req) => {
    set({ error: null });
    try {
      await api.bulkLink(repoId, req);
      await get().fetchEntries(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to bulk link') });
      throw err;
    }
  },

  switchTrackedLink: async (repoId, entryId, linkId) => {
    set({ error: null });
    try {
      await api.switchTrackedLink(repoId, entryId, linkId);
      await get().fetchEntries(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to switch the tracked link') });
      throw err;
    }
  },

  repairLink: async (repoId, entryId, linkId) => {
    set({ error: null });
    try {
      await api.repairLink(repoId, entryId, linkId);
      await get().fetchEntries(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to repair the link') });
      throw err;
    }
  },

  removeLink: async (repoId, entryId, linkId) => {
    set({ error: null });
    try {
      await api.removeLink(repoId, entryId, linkId);
      await get().fetchEntries(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to remove the link') });
      throw err;
    }
  },

  removeEntry: async (repoId, entryId, mode, linkId) => {
    set({ error: null });
    try {
      await api.removeEntry(repoId, entryId, mode, linkId);
      await get().fetchEntries(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to remove the entry') });
      throw err;
    }
  },

  // ── 一致性巡检 ───────────────────────────────────────────────────────

  audit: null,
  auditLoading: false,

  runAudit: async (repoId: string) => {
    set({ auditLoading: true, error: null });
    try {
      const audit = await api.fetchConsistency(repoId);
      set({ audit, auditLoading: false });
      return audit;
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to audit the repository'), auditLoading: false });
      throw err;
    }
  },

  repairConsistency: async (repoId: string) => {
    set({ auditLoading: true, error: null });
    try {
      const result = await api.repairConsistency(repoId);
      // 修复会改动本机软链接，重新拉一次条目与巡检结论
      await get().fetchEntries(repoId);
      set({ audit: await api.fetchConsistency(repoId), auditLoading: false });
      return result;
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to repair'), auditLoading: false });
      throw err;
    }
  },

  // ── 设备 ─────────────────────────────────────────────────────────────

  fetchCurrentDevice: async () => {
    try {
      set({ currentDevice: await api.fetchCurrentDevice() });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to fetch the current device') });
    }
  },

  fetchDevices: async (repoId: string) => {
    set({ error: null });
    try {
      set({ devices: await api.fetchDevices(repoId) });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to fetch devices') });
    }
  },

  registerDevice: async (repoId, name) => {
    set({ error: null });
    try {
      await api.registerDevice(repoId, name);
      await get().fetchDevices(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to register the device') });
      throw err;
    }
  },

  renameDevice: async (repoId, fingerprint, name) => {
    set({ error: null });
    try {
      await api.renameDevice(repoId, fingerprint, name);
      await get().fetchDevices(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to rename the device') });
      throw err;
    }
  },

  deleteDevice: async (repoId, fingerprint) => {
    set({ error: null });
    try {
      await api.deleteDevice(repoId, fingerprint);
      await get().fetchDevices(repoId);
      await get().fetchEntries(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to delete the device') });
      throw err;
    }
  },

  applyDevice: async (repoId, fingerprint, dryRun = false) => {
    set({ error: null });
    try {
      const result = await api.applyDevice(repoId, fingerprint, dryRun);
      if (!dryRun) {
        await get().fetchEntries(repoId);
        await get().fetchDevices(repoId);
      }
      return result;
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to apply the device') });
      throw err;
    }
  },

  // ── 备份 ─────────────────────────────────────────────────────────────

  triggerBackup: async (repoId: string, commitMessage?: string) => {
    set({ error: null, backupProgress: null });
    try {
      set({
        backupProgress: {
          repo_id: repoId,
          status: 'running',
          message: 'Starting backup...',
          progress: 0,
          started_at: new Date().toISOString(),
        },
      });
      const result = await api.triggerBackup(repoId, commitMessage);
      set({
        backupProgress: {
          repo_id: repoId,
          status: result.commit_hash ? 'completed' : 'failed',
          message:
            result.commit_message ||
            (result.files_changed > 0
              ? `Changed: ${result.files_changed}, Removed: ${result.files_removed}`
              : 'No changes'),
          progress: result.commit_hash ? 100 : 0,
          started_at: result.completed_at,
        },
      });
      set({ currentRepo: await api.fetchRepo(repoId) });
      return result;
    } catch (err: unknown) {
      const message = errMsg(err, 'Backup failed');
      set({
        backupProgress: {
          repo_id: repoId,
          status: 'failed',
          message,
          progress: 0,
          started_at: new Date().toISOString(),
        },
        error: message,
      });
    }
  },

  fetchBackupHistory: async (repoId, limit = 20, offset = 0) => {
    set({ error: null });
    try {
      set({ backupHistory: await api.fetchBackupHistory(repoId, limit, offset) });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to fetch backup history') });
    }
  },

  pushRepo: async (repoId: string, force = false) => {
    set({ error: null });
    try {
      await api.pushRepo(repoId, force);
      set({ currentRepo: await api.fetchRepo(repoId) });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Push failed') });
      throw err;
    }
  },

  gitInitRepo: async (repoId: string) => {
    set({ error: null });
    try {
      await api.gitInitRepo(repoId);
      set({ currentRepo: await api.fetchRepo(repoId) });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Git init failed') });
      throw err;
    }
  },

  // ── Git 认证 ─────────────────────────────────────────────────────────

  fetchAuth: async (repoId: string) => {
    set({ error: null });
    try {
      set({ currentAuth: await api.fetchAuth(repoId) });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to fetch auth config') });
    }
  },

  setAuth: async (repoId, auth) => {
    set({ error: null });
    try {
      set({ currentAuth: await api.setAuth(repoId, auth) });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to set auth config') });
      throw err;
    }
  },

  clearAuth: async (repoId: string) => {
    set({ error: null });
    try {
      await api.clearAuth(repoId);
      set({ currentAuth: null });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to clear auth config') });
      throw err;
    }
  },

  // ── 回滚 ─────────────────────────────────────────────────────────────

  fetchCommitFiles: async (repoId: string, commitHash: string) => {
    set({ error: null });
    try {
      const files = await api.fetchCommitChangedFiles(repoId, commitHash);
      set((state) => ({ commitFilesByHash: { ...state.commitFilesByHash, [commitHash]: files } }));
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to fetch commit files') });
    }
  },

  rollbackFiles: async (repoId: string, req: RollbackRequest) => {
    set({ rollbackLoading: true, error: null, rollbackResult: null });
    try {
      const result = await api.rollbackFiles(repoId, req);
      set({ rollbackResult: result, rollbackLoading: false });
      return result;
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Rollback failed'), rollbackLoading: false });
      throw err;
    }
  },

  clearRollbackResult: () => set({ rollbackResult: null }),

  // ── 提交文件预览 ─────────────────────────────────────────────────────

  fetchCommitFileContent: async (repoId: string, commitHash: string, path: string) => {
    set({ commitFileContentLoading: true, error: null, commitFileContent: null });
    try {
      const content = await api.fetchCommitFileContent(repoId, commitHash, path);
      set({ commitFileContent: content, commitFileContentLoading: false });
      return content;
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to fetch commit file content'), commitFileContentLoading: false });
      throw err;
    }
  },

  restoreCommitFile: async (repoId: string, commitHash: string, path: string) => {
    set({ restoreFileLoading: true, error: null });
    try {
      const result = await api.restoreCommitFile(repoId, commitHash, path);
      set({ restoreFileLoading: false });
      return result;
    } catch (err: unknown) {
      set({ error: errMsg(err, 'Failed to restore file'), restoreFileLoading: false });
      throw err;
    }
  },

  clearCommitFileContent: () => set({ commitFileContent: null, commitFileContentLoading: false }),
}));

/** 统一的错误消息提取。 */
function errMsg(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}
