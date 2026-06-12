import axios from 'axios';
import type {
  BackupRepo,
  CreateRepoRequest,
  UpdateConfigRequest,
  Symlink,
  CreateSymlinkRequest,
  UpdateSymlinkRequest,
  BatchSymlinkRequest,
  BrowseEntry,
  PreviewResult,
  BackupResult,
  CommitEntry,
  GitAuth,
  SetAuthRequest,
  CommitFileChange,
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

// Response interceptor: unwrap { data: ... } wrapper from backend
api.interceptors.response.use(
  (response) => {
    // If the response has a data wrapper, unwrap it
    if (response.data && typeof response.data === 'object' && 'data' in response.data) {
      response.data = response.data.data;
    }
    return response;
  },
  (error) => {
    const message = error.response?.data?.error || error.message || 'Request failed';
    return Promise.reject(new Error(message));
  }
);

// Repo APIs
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

export async function updateRepoConfig(
  id: string,
  config: UpdateConfigRequest
): Promise<void> {
  await api.put(`/repos/${id}/config`, config);
}

// Symlink APIs
export async function fetchSymlinks(repoId: string): Promise<Symlink[]> {
  const { data } = await api.get<Symlink[]>(`/repos/${repoId}/symlinks`);
  return data;
}

export async function createSymlink(
  repoId: string,
  req: CreateSymlinkRequest
): Promise<Symlink> {
  const { data } = await api.post<Symlink>(`/repos/${repoId}/symlinks`, req);
  return data;
}

export async function deleteSymlink(
  repoId: string,
  linkId: string
): Promise<void> {
  await api.delete(`/repos/${repoId}/symlinks/${linkId}`);
}

export async function updateSymlink(
  repoId: string,
  linkId: string,
  req: UpdateSymlinkRequest
): Promise<Symlink> {
  const { data } = await api.put<Symlink>(
    `/repos/${repoId}/symlinks/${linkId}`,
    req
  );
  return data;
}

export async function batchImportSymlinks(
  repoId: string,
  req: BatchSymlinkRequest
): Promise<Symlink[]> {
  const { data } = await api.post<Symlink[]>(
    `/repos/${repoId}/symlinks/batch`,
    req
  );
  return data;
}

// Directory symlink browsing API
export async function fetchDirEntries(
  repoId: string,
  linkId: string,
  subPath?: string
): Promise<BrowseEntry[]> {
  const { data } = await api.get<BrowseEntry[]>(
    `/repos/${repoId}/symlinks/${linkId}/entries`,
    { params: { sub_path: subPath || '' } }
  );
  return data;
}

// Browse API
export async function browsePath(path: string): Promise<BrowseEntry[]> {
  const { data } = await api.get<BrowseEntry[]>('/browse', {
    params: { path },
  });
  return data;
}

// Allowed roots (browse root directories)
export async function fetchAllowedRoots(): Promise<string[]> {
  const { data } = await api.get<string[]>('/browse/allowed-roots');
  return data;
}

// Preview API
export async function previewFile(
  repoId: string,
  path: string
): Promise<PreviewResult> {
  const { data } = await api.get<PreviewResult>(
    `/repos/${repoId}/preview`,
    { params: { path } }
  );
  return data;
}

// Backup APIs
export async function triggerBackup(repoId: string): Promise<BackupResult> {
  const { data } = await api.post<BackupResult>(
    `/repos/${repoId}/backup`
  );
  return data;
}

export async function fetchBackupHistory(
  repoId: string,
  limit = 20,
  offset = 0
): Promise<CommitEntry[]> {
  const { data } = await api.get<CommitEntry[]>(
    `/repos/${repoId}/backup/history`,
    { params: { limit, offset } }
  );
  return data;
}

// Auth APIs
export async function fetchAuth(repoId: string): Promise<GitAuth> {
  const { data } = await api.get<GitAuth>(`/repos/${repoId}/auth`);
  return data;
}

export async function setAuth(
  repoId: string,
  req: SetAuthRequest
): Promise<GitAuth> {
  const { data } = await api.put<GitAuth>(`/repos/${repoId}/auth`, req);
  return data;
}

export async function clearAuth(repoId: string): Promise<void> {
  await api.delete(`/repos/${repoId}/auth`);
}

// Git init API
export async function gitInitRepo(repoId: string): Promise<void> {
  await api.post(`/repos/${repoId}/git-init`);
}

// Push API
export async function pushRepo(repoId: string, force = false): Promise<void> {
  await api.post(`/repos/${repoId}/push`, { force });
}

// Health API
export async function healthCheck(): Promise<{ status: string }> {
  const { data } = await api.get<{ status: string }>('/health');
  return data;
}

// Rollback APIs
export async function fetchCommitChangedFiles(
  repoId: string,
  commitHash: string
): Promise<CommitFileChange[]> {
  const { data } = await api.get<CommitFileChange[]>(
    `/repos/${repoId}/commits/${commitHash}/changed-files`
  );
  return data;
}

export async function rollbackSourceFiles(
  repoId: string,
  req: RollbackRequest
): Promise<RollbackResult> {
  const { data } = await api.post<RollbackResult>(
    `/repos/${repoId}/rollback`,
    req
  );
  return data;
}

export default api;
