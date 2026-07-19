import axios from 'axios';

const BASE_URL = import.meta.env.VITE_API_URL || '';

export const api = axios.create({
  baseURL: BASE_URL,
  timeout: 30000,
});

// Request interceptor — attach JWT token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Response interceptor — handle 401 (token expired)
api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const original = error.config;
    if (error.response?.status === 401 && !original._retry) {
      original._retry = true;
      const refreshToken = localStorage.getItem('refresh_token');
      if (refreshToken) {
        try {
          const { data } = await axios.post(`${BASE_URL}/auth/refresh`, {
            refresh_token: refreshToken,
          });
          localStorage.setItem('access_token', data.access_token);
          localStorage.setItem('refresh_token', data.refresh_token);
          original.headers.Authorization = `Bearer ${data.access_token}`;
          return api(original);
        } catch {
          localStorage.clear();
          window.location.href = '/login';
        }
      }
    }
    return Promise.reject(error);
  }
);

// ── Auth ──────────────────────────────────────────────────────────────────

export interface TokenResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  token_type: string;
}

export const login = (username: string, password: string) =>
  api.post<TokenResponse>('/auth/token', { username, password }).then((r) => r.data);

// ── Config ────────────────────────────────────────────────────────────────

export interface ResourceTypeInfo {
  name: string;
  allowedExtensions: string[];
  maxSizeMb: number;
  publicRead: boolean;
  url: string;
}

export interface AppConfig {
  resourceTypes: ResourceTypeInfo[];
  maxUploadMb: number;
}

export const fetchConfig = () =>
  api.get<AppConfig>('/api/config').then((r) => r.data);

// ── Folders ───────────────────────────────────────────────────────────────

export interface FolderInfo {
  name: string;
  path: string;
  hasChildren: boolean;
  acl: number;
}

export interface FoldersResponse {
  resourceType: string;
  currentFolder: { path: string; url: string; acl: number };
  folders: FolderInfo[];
}

export const fetchFolders = (type: string, path: string) =>
  api.get<FoldersResponse>('/api/folders', { params: { type, path } }).then((r) => r.data);

export const createFolder = (type: string, path: string, name: string) =>
  api.post('/api/folder', { type, path, name }).then((r) => r.data);

export const deleteFolder = (type: string, path: string) =>
  api.delete('/api/folder', { data: { type, path } }).then((r) => r.data);

export const renameFolder = (type: string, path: string, newName: string) =>
  api.patch('/api/folder/rename', { type, path, newName }).then((r) => r.data);

// ── Files ─────────────────────────────────────────────────────────────────

export interface FileInfo {
  name: string;
  date: string;
  size: number;
  url: string;
  thumb: string;
}

export interface FilesResponse {
  resourceType: string;
  currentFolder: { path: string; url: string; acl: number };
  files: FileInfo[];
}

export const fetchFiles = (type: string, path: string) =>
  api.get<FilesResponse>('/api/files', { params: { type, path } }).then((r) => r.data);

export const deleteFiles = (type: string, path: string, files: string[]) =>
  api.delete('/api/files', { data: { type, path, files } }).then((r) => r.data);

export const renameFile = (type: string, path: string, name: string, newName: string) =>
  api.patch('/api/file/rename', { type, path, name, newName }).then((r) => r.data);

export interface FileRef { type: string; path: string; name: string; }

export const moveFiles = (files: FileRef[], destination: FileRef) =>
  api.post('/api/files/move', { files, destination }).then((r) => r.data);

export const copyFiles = (files: FileRef[], destination: FileRef) =>
  api.post('/api/files/copy', { files, destination }).then((r) => r.data);

export const downloadFile = (type: string, path: string, name: string) =>
  api.get<{ url: string }>('/api/file/download', { params: { type, path, name } }).then((r) => r.data);

export const uploadFile = (
  type: string,
  path: string,
  file: File,
  onProgress?: (percent: number) => void
) => {
  const form = new FormData();
  form.append('type', type);
  form.append('path', path);
  form.append('file', file);
  return api.post<{ fileName: string; uploaded: number; url: string }>('/api/upload', form, {
    onUploadProgress: (e) => {
      if (e.total) onProgress?.(Math.round((e.loaded * 100) / e.total));
    },
  }).then((r) => r.data);
};

export const thumbnailUrl = (type: string, path: string, name: string, w = 150, h = 150) => {
  const token = localStorage.getItem('access_token') || '';
  return `${BASE_URL}/api/thumbnail?type=${type}&path=${encodeURIComponent(path)}&name=${encodeURIComponent(name)}&w=${w}&h=${h}&token=${encodeURIComponent(token)}`;
};

// ── Stats & ZIP ────────────────────────────────────────────────────────────

export interface StorageStats {
  totalCount: number;
  totalSize: number;
  breakdown: Record<string, { count: number; size: number }>;
}

export const compressFiles = (type: string, path: string, files: string[], zipName: string) =>
  api.post('/api/files/compress', { type, path, files, zipName }).then((r) => r.data);

export const extractZip = (type: string, path: string, fileName: string) =>
  api.post('/api/files/extract', { type, path, fileName }).then((r) => r.data);

export const fetchStats = () =>
  api.get<StorageStats>('/api/stats').then((r) => r.data);
