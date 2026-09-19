import { create } from 'zustand';
import i18n from '../i18n';
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
  DetachMode,
  DetachResult,
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
  bulkLink: (repoId: string, req: BulkLinkRequest) => Promise<Entry[]>;

  repairLink: (repoId: string, entryId: string, linkId: string) => Promise<void>;
  readoptLink: (repoId: string, entryId: string, linkId: string) => Promise<void>;
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
  fetchCurrentDevice: () => Promise<CurrentDeviceInfo | null>;
  fetchDevices: (repoId: string) => Promise<Device[]>;
  registerDevice: (repoId: string, name?: string) => Promise<void>;
  renameDevice: (repoId: string, fingerprint: string, name: string) => Promise<void>;
  deleteDevice: (repoId: string, fingerprint: string) => Promise<void>;
  applyDevice: (repoId: string, fingerprint: string, dryRun?: boolean) => Promise<ApplyResult>;
  detachDevice: (repoId: string, fingerprint: string, mode: DetachMode) => Promise<DetachResult>;

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
      set({ error: errMsg(err, 'errors.repos.fetch'), loading: false });
    }
  },

  fetchRepo: async (id: string) => {
    set({ loading: true, error: null });
    try {
      set({ currentRepo: await api.fetchRepo(id), loading: false });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.repo.fetch'), loading: false });
    }
  },

  createRepo: async (name: string, path: string) => {
    set({ loading: true, error: null });
    try {
      const repo = await api.createRepo({ name, path });
      set({ repos: [...get().repos, repo], loading: false });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.repo.create'), loading: false });
      throw err;
    }
  },

  deleteRepo: async (id: string) => {
    set({ loading: true, error: null });
    try {
      await api.deleteRepo(id);
      set({ repos: get().repos.filter((r) => r.id !== id), loading: false });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.repo.delete'), loading: false });
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
      set({ error: errMsg(err, 'errors.repo.updateConfig') });
      throw err;
    }
  },

  // ── 条目与链接 ──────────────────────────────────────────────────────

  fetchEntries: async (repoId: string) => {
    set({ error: null });
    try {
      set({ entries: await api.fetchEntries(repoId) });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.entries.fetch') });
    }
  },

  adoptEntry: async (repoId, req) => {
    set({ error: null });
    try {
      await api.adoptEntry(repoId, req);
      await get().fetchEntries(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.entry.create') });
      throw err;
    }
  },

  addLink: async (repoId, entryId, req) => {
    set({ error: null });
    try {
      await api.addLink(repoId, entryId, req);
      await get().fetchEntries(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.link.add') });
      throw err;
    }
  },

  bulkLink: async (repoId, req) => {
    set({ error: null });
    try {
      const views = await api.bulkLink(repoId, req);
      await get().fetchEntries(repoId);
      return views;
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.link.bulk') });
      throw err;
    }
  },

  repairLink: async (repoId, entryId, linkId) => {
    set({ error: null });
    try {
      await api.repairLink(repoId, entryId, linkId);
      await get().fetchEntries(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.link.repair') });
      throw err;
    }
  },

  readoptLink: async (repoId, entryId, linkId) => {
    set({ error: null });
    try {
      await api.readoptLink(repoId, entryId, linkId);
      await get().fetchEntries(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.link.readopt') });
      throw err;
    }
  },

  removeLink: async (repoId, entryId, linkId) => {
    set({ error: null });
    try {
      await api.removeLink(repoId, entryId, linkId);
      await get().fetchEntries(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.link.remove') });
      throw err;
    }
  },

  removeEntry: async (repoId, entryId, mode, linkId) => {
    set({ error: null });
    try {
      await api.removeEntry(repoId, entryId, mode, linkId);
      await get().fetchEntries(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.entry.remove') });
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
      set({ error: errMsg(err, 'errors.audit.fetch'), auditLoading: false });
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
      set({ error: errMsg(err, 'errors.audit.repair'), auditLoading: false });
      throw err;
    }
  },

  // ── 设备 ─────────────────────────────────────────────────────────────

  fetchCurrentDevice: async () => {
    try {
      const info = await api.fetchCurrentDevice();
      set({ currentDevice: info });
      return info;
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.device.current') });
      return null;
    }
  },

  fetchDevices: async (repoId: string) => {
    set({ error: null });
    try {
      const devices = await api.fetchDevices(repoId);
      set({ devices });
      return devices;
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.device.fetch') });
      return [];
    }
  },

  registerDevice: async (repoId, name) => {
    set({ error: null });
    try {
      await api.registerDevice(repoId, name);
      await get().fetchDevices(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.device.register') });
      throw err;
    }
  },

  renameDevice: async (repoId, fingerprint, name) => {
    set({ error: null });
    try {
      await api.renameDevice(repoId, fingerprint, name);
      await get().fetchDevices(repoId);
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.device.rename') });
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
      set({ error: errMsg(err, 'errors.device.delete') });
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
      set({ error: errMsg(err, 'errors.device.apply') });
      throw err;
    }
  },

  detachDevice: async (repoId, fingerprint, mode) => {
    set({ error: null });
    try {
      const result = await api.detachDevice(repoId, fingerprint, mode);
      await get().fetchEntries(repoId);
      return result;
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.device.detach') });
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
          message: i18n.t('backup.progress.starting'),
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
              ? i18n.t('backup.progress.changed', {
                  changed: result.files_changed,
                  removed: result.files_removed,
                })
              : i18n.t('backup.progress.noChanges')),
          progress: result.commit_hash ? 100 : 0,
          started_at: result.completed_at,
        },
      });
      set({ currentRepo: await api.fetchRepo(repoId) });
      return result;
    } catch (err: unknown) {
      const message = errMsg(err, 'errors.backup.failed');
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
      set({ error: errMsg(err, 'errors.backup.history') });
    }
  },

  pushRepo: async (repoId: string, force = false) => {
    set({ error: null });
    try {
      await api.pushRepo(repoId, force);
      set({ currentRepo: await api.fetchRepo(repoId) });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.backup.push') });
      throw err;
    }
  },

  gitInitRepo: async (repoId: string) => {
    set({ error: null });
    try {
      await api.gitInitRepo(repoId);
      set({ currentRepo: await api.fetchRepo(repoId) });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.backup.gitInit') });
      throw err;
    }
  },

  // ── Git 认证 ─────────────────────────────────────────────────────────

  fetchAuth: async (repoId: string) => {
    set({ error: null });
    try {
      set({ currentAuth: await api.fetchAuth(repoId) });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.auth.fetch') });
    }
  },

  setAuth: async (repoId, auth) => {
    set({ error: null });
    try {
      set({ currentAuth: await api.setAuth(repoId, auth) });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.auth.set') });
      throw err;
    }
  },

  clearAuth: async (repoId: string) => {
    set({ error: null });
    try {
      await api.clearAuth(repoId);
      set({ currentAuth: null });
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.auth.clear') });
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
      set({ error: errMsg(err, 'errors.rollback.files') });
    }
  },

  rollbackFiles: async (repoId: string, req: RollbackRequest) => {
    set({ rollbackLoading: true, error: null, rollbackResult: null });
    try {
      const result = await api.rollbackFiles(repoId, req);
      set({ rollbackResult: result, rollbackLoading: false });
      return result;
    } catch (err: unknown) {
      set({ error: errMsg(err, 'errors.rollback.execute'), rollbackLoading: false });
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
      set({ error: errMsg(err, 'errors.rollback.preview'), commitFileContentLoading: false });
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
      set({ error: errMsg(err, 'errors.rollback.restore'), restoreFileLoading: false });
      throw err;
    }
  },

  clearCommitFileContent: () => set({ commitFileContent: null, commitFileContentLoading: false }),
}));

/** 统一提取后端详情；缺少详情时使用当前语言的操作级错误文案。 */
function errMsg(err: unknown, fallbackKey: string): string {
  return err instanceof Error ? err.message : i18n.t(fallbackKey);
}
