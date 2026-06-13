import { create } from 'zustand';
import type {
  BackupRepo,
  BackupResult,
  Symlink,
  GitAuth,
  SymlinkDirEntry,
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
  symlinks: Symlink[];
  backupHistory: CommitEntry[];
  currentAuth: GitAuth | null;
  backupProgress: BackupProgressState | null;
  loading: boolean;
  error: string | null;

  // Directory browsing cache
  dirEntriesCache: Record<string, SymlinkDirEntry[]>;

  // Rollback state
  commitFilesByHash: Record<string, CommitFileChange[]>;
  rollbackResult: RollbackResult | null;
  rollbackLoading: boolean;

  // Commit file preview state
  commitFileContent: CommitFileContent | null;
  commitFileContentLoading: boolean;
  restoreFileLoading: boolean;

  fetchRepos: () => Promise<void>;
  fetchRepo: (id: string) => Promise<void>;
  createRepo: (name: string, path: string) => Promise<void>;
  deleteRepo: (id: string) => Promise<void>;
  updateRepoConfig: (id: string, config: Parameters<typeof api.updateRepoConfig>[1]) => Promise<void>;

  fetchSymlinks: (repoId: string) => Promise<void>;
  createSymlink: (repoId: string, targetPath: string, relativePath: string) => Promise<void>;
  deleteSymlink: (repoId: string, linkId: string) => Promise<void>;
  updateSymlink: (repoId: string, linkId: string, targetPath: string) => Promise<void>;
  batchImportSymlinks: (repoId: string, targets: Array<{ target_path: string; relative_path: string }>) => Promise<void>;

  triggerBackup: (repoId: string, commitMessage?: string) => Promise<BackupResult | void>;
  pushRepo: (repoId: string, force?: boolean) => Promise<void>;
  gitInitRepo: (repoId: string) => Promise<void>;
  fetchBackupHistory: (repoId: string, limit?: number, offset?: number) => Promise<void>;

  fetchAuth: (repoId: string) => Promise<void>;
  setAuth: (repoId: string, auth: Parameters<typeof api.setAuth>[1]) => Promise<void>;
  clearAuth: (repoId: string) => Promise<void>;

  // Directory browsing
  fetchDirEntries: (repoId: string, linkId: string, subPath?: string) => Promise<SymlinkDirEntry[]>;
  clearDirEntryCache: (linkId?: string) => void;

  // Rollback actions
  fetchCommitFiles: (repoId: string, commitHash: string) => Promise<void>;
  rollbackSourceFiles: (repoId: string, req: RollbackRequest) => Promise<RollbackResult>;
  clearRollbackResult: () => void;

  // Commit file preview actions
  fetchCommitFileContent: (repoId: string, commitHash: string, path: string) => Promise<CommitFileContent>;
  restoreCommitFile: (repoId: string, commitHash: string, path: string) => Promise<FileRestoreResult>;
  clearCommitFileContent: () => void;

  clearError: () => void;
}

export const useAppStore = create<AppState>((set, get) => ({
  repos: [],
  currentRepo: null,
  symlinks: [],
  backupHistory: [],
  currentAuth: null,
  backupProgress: null,
  loading: false,
  error: null,

  // Directory browsing initial state
  dirEntriesCache: {},

  // Rollback initial state
  commitFilesByHash: {},
  rollbackResult: null,
  rollbackLoading: false,

  // Commit file preview initial state
  commitFileContent: null,
  commitFileContentLoading: false,
  restoreFileLoading: false,

  clearError: () => set({ error: null }),

  fetchRepos: async () => {
    set({ loading: true, error: null });
    try {
      const repos = await api.fetchRepos();
      set({ repos, loading: false });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to fetch repos';
      set({ error: message, loading: false });
    }
  },

  fetchRepo: async (id: string) => {
    set({ loading: true, error: null });
    try {
      const repo = await api.fetchRepo(id);
      set({ currentRepo: repo, loading: false });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to fetch repo';
      set({ error: message, loading: false });
    }
  },

  createRepo: async (name: string, path: string) => {
    set({ loading: true, error: null });
    try {
      const repo = await api.createRepo({ name, path });
      const { repos } = get();
      set({ repos: [...repos, repo], loading: false });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to create repo';
      set({ error: message, loading: false });
      throw err;
    }
  },

  deleteRepo: async (id: string) => {
    set({ loading: true, error: null });
    try {
      await api.deleteRepo(id);
      const { repos } = get();
      set({ repos: repos.filter((r) => r.id !== id), loading: false });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to delete repo';
      set({ error: message, loading: false });
      throw err;
    }
  },

  updateRepoConfig: async (id, config) => {
    set({ error: null });
    try {
      await api.updateRepoConfig(id, config);
      const repo = await api.fetchRepo(id);
      set({ currentRepo: repo });
      const { repos } = get();
      set({
        repos: repos.map((r) => (r.id === id ? repo : r)),
      });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to update config';
      set({ error: message });
      throw err;
    }
  },

  fetchSymlinks: async (repoId: string) => {
    set({ error: null });
    try {
      const symlinks = await api.fetchSymlinks(repoId);
      set({ symlinks });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to fetch symlinks';
      set({ error: message });
    }
  },

  createSymlink: async (repoId, targetPath, relativePath) => {
    set({ error: null });
    try {
      await api.createSymlink(repoId, { target_path: targetPath, relative_path: relativePath });
      await get().fetchSymlinks(repoId);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to create symlink';
      set({ error: message });
      throw err;
    }
  },

  deleteSymlink: async (repoId, linkId) => {
    set({ error: null });
    try {
      await api.deleteSymlink(repoId, linkId);
      await get().fetchSymlinks(repoId);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to delete symlink';
      set({ error: message });
      throw err;
    }
  },

  updateSymlink: async (repoId, linkId, targetPath) => {
    set({ error: null });
    try {
      await api.updateSymlink(repoId, linkId, { target_path: targetPath });
      await get().fetchSymlinks(repoId);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to update symlink';
      set({ error: message });
      throw err;
    }
  },

  batchImportSymlinks: async (repoId, targets) => {
    set({ error: null });
    try {
      await api.batchImportSymlinks(repoId, { targets });
      await get().fetchSymlinks(repoId);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to batch import symlinks';
      set({ error: message });
      throw err;
    }
  },

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
          message: result.commit_message || (result.files_changed > 0 ? `Changed: ${result.files_changed}, Removed: ${result.files_removed}` : 'No changes'),
          progress: result.commit_hash ? 100 : 0,
          started_at: result.completed_at,
        },
      });
      const repo = await api.fetchRepo(repoId);
      set({ currentRepo: repo });
      return result;
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Backup failed';
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
      const history: CommitEntry[] = await api.fetchBackupHistory(repoId, limit, offset);
      set({ backupHistory: history });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to fetch backup history';
      set({ error: message });
    }
  },

  pushRepo: async (repoId: string, force = false) => {
    set({ error: null });
    try {
      await api.pushRepo(repoId, force);
      // Refresh repo to update status
      const repo = await api.fetchRepo(repoId);
      set({ currentRepo: repo });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Push failed';
      set({ error: message });
      throw err;
    }
  },

  gitInitRepo: async (repoId: string) => {
    set({ error: null });
    try {
      await api.gitInitRepo(repoId);
      // Refresh repo to get updated git_initialized flag
      const repo = await api.fetchRepo(repoId);
      set({ currentRepo: repo });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Git init failed';
      set({ error: message });
      throw err;
    }
  },

  fetchAuth: async (repoId: string) => {
    set({ error: null });
    try {
      const auth = await api.fetchAuth(repoId);
      set({ currentAuth: auth });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to fetch auth config';
      set({ error: message });
    }
  },

  setAuth: async (repoId, auth) => {
    set({ error: null });
    try {
      const result = await api.setAuth(repoId, auth);
      set({ currentAuth: result });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to set auth config';
      set({ error: message });
      throw err;
    }
  },

  clearAuth: async (repoId: string) => {
    set({ error: null });
    try {
      await api.clearAuth(repoId);
      set({ currentAuth: null });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to clear auth config';
      set({ error: message });
      throw err;
    }
  },

  // Directory browsing
  fetchDirEntries: async (repoId: string, linkId: string, subPath?: string) => {
    set({ error: null });
    const cacheKey = `${linkId}:${subPath || ''}`;
    const cached = get().dirEntriesCache[cacheKey];
    if (cached) {
      return cached;
    }
    try {
      const entries = await api.fetchDirEntries(repoId, linkId, subPath);
      set((state) => ({
        dirEntriesCache: { ...state.dirEntriesCache, [cacheKey]: entries },
      }));
      return entries;
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to fetch directory entries';
      set({ error: message });
      throw err;
    }
  },

  clearDirEntryCache: (linkId?: string) => {
    if (linkId) {
      set((state) => {
        const newCache = { ...state.dirEntriesCache };
        Object.keys(newCache).forEach((key) => {
          if (key.startsWith(`${linkId}:`)) {
            delete newCache[key];
          }
        });
        return { dirEntriesCache: newCache };
      });
    } else {
      set({ dirEntriesCache: {} });
    }
  },

  // Rollback actions
  fetchCommitFiles: async (repoId: string, commitHash: string) => {
    set({ error: null });
    try {
      const files = await api.fetchCommitChangedFiles(repoId, commitHash);
      set((state) => ({
        commitFilesByHash: { ...state.commitFilesByHash, [commitHash]: files },
      }));
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to fetch commit files';
      set({ error: message });
    }
  },

  rollbackSourceFiles: async (repoId: string, req: RollbackRequest) => {
    set({ rollbackLoading: true, error: null, rollbackResult: null });
    try {
      const result = await api.rollbackSourceFiles(repoId, req);
      set({ rollbackResult: result, rollbackLoading: false });
      return result;
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Rollback failed';
      set({ error: message, rollbackLoading: false });
      throw err;
    }
  },

  clearRollbackResult: () => set({ rollbackResult: null }),

  // Commit file preview actions
  fetchCommitFileContent: async (repoId: string, commitHash: string, path: string) => {
    set({ commitFileContentLoading: true, error: null, commitFileContent: null });
    try {
      const content = await api.fetchCommitFileContent(repoId, commitHash, path);
      set({ commitFileContent: content, commitFileContentLoading: false });
      return content;
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to fetch commit file content';
      set({ error: message, commitFileContentLoading: false });
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
      const message = err instanceof Error ? err.message : 'Failed to restore file';
      set({ error: message, restoreFileLoading: false });
      throw err;
    }
  },

  clearCommitFileContent: () => set({ commitFileContent: null, commitFileContentLoading: false }),
}));
