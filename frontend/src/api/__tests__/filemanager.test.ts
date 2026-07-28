import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  api,
  login,
  embedLogin,
  logout,
  fetchConfig,
  fetchFolders,
  createFolder,
  deleteFolder,
  renameFolder,
  fetchFiles,
  deleteFiles,
  renameFile,
  moveFiles,
  copyFiles,
  downloadFile,
  uploadFile,
  thumbnailUrl,
  compressFiles,
  extractZip,
  fetchStats,
} from '../filemanager';

describe('filemanager API functions', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('login posts credentials and returns token response', async () => {
    const mockData = { access_token: 'abc', refresh_token: 'ref', expires_in: 3600, token_type: 'Bearer' };
    vi.spyOn(api, 'post').mockResolvedValueOnce({ data: mockData });

    const res = await login('admin', 'password');
    expect(api.post).toHaveBeenCalledWith('/auth/token', { username: 'admin', password: 'password' });
    expect(res).toEqual(mockData);
  });

  it('embedLogin posts ticket', async () => {
    const mockData = { access_token: 'abc', refresh_token: 'ref', expires_in: 3600, token_type: 'Bearer' };
    vi.spyOn(api, 'post').mockResolvedValueOnce({ data: mockData });

    const res = await embedLogin('ticket123');
    expect(api.post).toHaveBeenCalledWith('/auth/embed/login', { ticket: 'ticket123' });
    expect(res).toEqual(mockData);
  });

  it('logout posts to logout endpoint', async () => {
    vi.spyOn(api, 'post').mockResolvedValueOnce({ data: { message: 'ok' } });

    const res = await logout();
    expect(api.post).toHaveBeenCalledWith('/auth/logout');
    expect(res).toEqual({ message: 'ok' });
  });

  it('fetchConfig gets configuration', async () => {
    const mockConfig = { resourceTypes: [], maxUploadMb: 50 };
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: mockConfig });

    const res = await fetchConfig();
    expect(api.get).toHaveBeenCalledWith('/api/config');
    expect(res).toEqual(mockConfig);
  });

  it('fetchFolders fetches folder structure', async () => {
    const mockRes = { resourceType: 'Images', currentFolder: { path: '/', url: '', acl: 255 }, folders: [] };
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: mockRes });

    const res = await fetchFolders('Images', '/');
    expect(api.get).toHaveBeenCalledWith('/api/folders', { params: { type: 'Images', path: '/' } });
    expect(res).toEqual(mockRes);
  });

  it('createFolder sends folder details', async () => {
    vi.spyOn(api, 'post').mockResolvedValueOnce({ data: { success: true } });

    const res = await createFolder('Images', '/', 'new_folder');
    expect(api.post).toHaveBeenCalledWith('/api/folder', { type: 'Images', path: '/', name: 'new_folder' });
    expect(res).toEqual({ success: true });
  });

  it('deleteFolder sends delete payload', async () => {
    vi.spyOn(api, 'delete').mockResolvedValueOnce({ data: { success: true } });

    const res = await deleteFolder('Images', '/old_folder');
    expect(api.delete).toHaveBeenCalledWith('/api/folder', { data: { type: 'Images', path: '/old_folder' } });
    expect(res).toEqual({ success: true });
  });

  it('renameFolder sends patch payload', async () => {
    vi.spyOn(api, 'patch').mockResolvedValueOnce({ data: { success: true } });

    const res = await renameFolder('Images', '/old', 'new');
    expect(api.patch).toHaveBeenCalledWith('/api/folder/rename', { type: 'Images', path: '/old', newName: 'new' });
    expect(res).toEqual({ success: true });
  });

  it('fetchFiles fetches file list', async () => {
    const mockRes = { resourceType: 'Images', currentFolder: { path: '/', url: '', acl: 255 }, files: [] };
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: mockRes });

    const res = await fetchFiles('Images', '/');
    expect(api.get).toHaveBeenCalledWith('/api/files', { params: { type: 'Images', path: '/' } });
    expect(res).toEqual(mockRes);
  });

  it('deleteFiles sends files array for deletion', async () => {
    vi.spyOn(api, 'delete').mockResolvedValueOnce({ data: { success: true } });

    const res = await deleteFiles('Images', '/', ['a.png', 'b.png']);
    expect(api.delete).toHaveBeenCalledWith('/api/files', { data: { type: 'Images', path: '/', files: ['a.png', 'b.png'] } });
    expect(res).toEqual({ success: true });
  });

  it('renameFile sends patch request', async () => {
    vi.spyOn(api, 'patch').mockResolvedValueOnce({ data: { success: true } });

    const res = await renameFile('Images', '/', 'old.png', 'new.png');
    expect(api.patch).toHaveBeenCalledWith('/api/file/rename', { type: 'Images', path: '/', name: 'old.png', newName: 'new.png' });
    expect(res).toEqual({ success: true });
  });

  it('moveFiles and copyFiles post correct payloads', async () => {
    vi.spyOn(api, 'post').mockResolvedValue({ data: { success: true } });

    const files = [{ type: 'Images', path: '/', name: 'pic.png' }];
    const destination = { type: 'Images', path: '/dest', name: '' };

    await moveFiles(files, destination);
    expect(api.post).toHaveBeenCalledWith('/api/files/move', { files, destination });

    await copyFiles(files, destination);
    expect(api.post).toHaveBeenCalledWith('/api/files/copy', { files, destination });
  });

  it('downloadFile gets download url', async () => {
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: { url: 'http://example.com/dl' } });

    const res = await downloadFile('Images', '/', 'pic.png');
    expect(api.get).toHaveBeenCalledWith('/api/file/download', { params: { type: 'Images', path: '/', name: 'pic.png' } });
    expect(res.url).toBe('http://example.com/dl');
  });

  it('uploadFile formats FormData and triggers onProgress', async () => {
    vi.spyOn(api, 'post').mockImplementation((_url, _data, config) => {
      if (config?.onUploadProgress) {
        config.onUploadProgress({ loaded: 50, total: 100 } as any);
      }
      return Promise.resolve({ data: { fileName: 'test.jpg', uploaded: 1, url: '/test.jpg' } }) as any;
    });

    const progressFn = vi.fn();
    const mockFile = new File(['dummy content'], 'test.jpg', { type: 'image/jpeg' });

    const res = await uploadFile('Images', '/', mockFile, progressFn);

    expect(api.post).toHaveBeenCalled();
    expect(progressFn).toHaveBeenCalledWith(50);
    expect(res.fileName).toBe('test.jpg');
  });

  it('thumbnailUrl generates correct URL string', () => {
    const url = thumbnailUrl('Images', '/my folder', 'my photo.png', 200, 200);
    expect(url).toContain('/api/thumbnail?type=Images&path=%2Fmy%20folder&name=my%20photo.png&w=200&h=200');
  });

  it('compressFiles, extractZip, and fetchStats post and get data correctly', async () => {
    vi.spyOn(api, 'post').mockResolvedValue({ data: { success: true } });
    vi.spyOn(api, 'get').mockResolvedValue({ data: { totalCount: 10, totalSize: 1024, breakdown: {} } });

    await compressFiles('Files', '/', ['doc.pdf'], 'archive.zip');
    expect(api.post).toHaveBeenCalledWith('/api/files/compress', { type: 'Files', path: '/', files: ['doc.pdf'], zipName: 'archive.zip' });

    await extractZip('Files', '/', 'archive.zip');
    expect(api.post).toHaveBeenCalledWith('/api/files/extract', { type: 'Files', path: '/', fileName: 'archive.zip' });

    const stats = await fetchStats();
    expect(api.get).toHaveBeenCalledWith('/api/stats');
    expect(stats.totalCount).toBe(10);
  });
});
