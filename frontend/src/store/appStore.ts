import { create } from 'zustand';
import type {
  BackupRepo,
  Symlink,
  GitAuth,
  CommitEntry,
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

  triggerBackup: (repoId: string) => Promise<void>;
  fetchBackupHistory: (repoId: string, limit?: number, offset?: number) => Promise<void>;

  fetchAuth: (repoId: string) => Promise<void>;
  setAuth: (repoId: string, auth: Parameters<typeof api.setAuth>[1]) => Promise<void>;
  clearAuth: (repoId: string) => Promise<void>;

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

  triggerBackup: async (repoId: string) => {
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
      const result = await api.triggerBackup(repoId);
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
}));
