import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { FileManager } from '../FileManager';
import * as api from '../../api/filemanager';
import { useFileManagerStore } from '../../store/fileManagerStore';

vi.mock('../../api/filemanager', () => ({
  fetchConfig: vi.fn(),
  fetchFiles: vi.fn(),
  fetchFolders: vi.fn(),
  fetchStats: vi.fn(),
  deleteFiles: vi.fn(),
  thumbnailUrl: vi.fn().mockReturnValue('http://example.com/thumb.jpg'),
  copyFiles: vi.fn(),
  moveFiles: vi.fn(),
  extractZip: vi.fn(),
  uploadFile: vi.fn(),
  logout: vi.fn(),
}));

describe('FileManager Component', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.mocked(api.fetchStats).mockResolvedValue({ totalCount: 0, totalSize: 0, breakdown: {} });
    useFileManagerStore.setState({
      activeResourceType: 'Images',
      resourceTypes: [
        { name: 'Images', allowedExtensions: ['jpg', 'png'], maxSizeMb: 10, publicRead: true, url: '' },
        { name: 'Files', allowedExtensions: ['pdf', 'doc'], maxSizeMb: 20, publicRead: false, url: '' },
      ],
      currentPath: '/',
      files: [],
      selectedFiles: new Set(),
      viewMode: 'grid',
      sidebarOpen: true,
      sidebarWidth: 240,
    });
  });

  it('renders header, toolbar, sidebar, and file grid view', async () => {
    vi.mocked(api.fetchConfig).mockResolvedValueOnce({
      resourceTypes: [
        { name: 'Images', allowedExtensions: ['jpg', 'png'], maxSizeMb: 10, publicRead: true, url: '' },
      ],
      maxUploadMb: 50,
    });

    vi.mocked(api.fetchFiles).mockResolvedValueOnce({
      resourceType: 'Images',
      currentFolder: { path: '/', url: '', acl: 255 },
      files: [
        { name: 'pic1.png', date: '2026-01-01', size: 500, url: 'http://example.com/pic1.png', thumb: '' },
        { name: 'pic2.png', date: '2026-01-02', size: 1200, url: 'http://example.com/pic2.png', thumb: '' },
      ],
    });

    vi.mocked(api.fetchFolders).mockResolvedValue({
      resourceType: 'Images',
      currentFolder: { path: '/', url: '', acl: 255 },
      folders: [],
    });

    render(<FileManager />);

    expect(screen.getByText('Media Manager')).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('pic1.png')).toBeInTheDocument();
      expect(screen.getByText('pic2.png')).toBeInTheDocument();
    });
  });

  it('switches between grid and list views', async () => {
    vi.mocked(api.fetchFiles).mockResolvedValue({
      resourceType: 'Images',
      currentFolder: { path: '/', url: '', acl: 255 },
      files: [{ name: 'file1.png', date: '2026-01-01', size: 500, url: '', thumb: '' }],
    });
    vi.mocked(api.fetchFolders).mockResolvedValue({
      resourceType: 'Images',
      currentFolder: { path: '/', url: '', acl: 255 },
      folders: [],
    });

    render(<FileManager />);

    await waitFor(() => {
      expect(screen.getByText('file1.png')).toBeInTheDocument();
    });

    const listViewBtn = screen.getByTitle('List view');
    fireEvent.click(listViewBtn);

    expect(useFileManagerStore.getState().viewMode).toBe('list');

    const gridViewBtn = screen.getByTitle('Grid view');
    fireEvent.click(gridViewBtn);

    expect(useFileManagerStore.getState().viewMode).toBe('grid');
  });

  it('filters files via search input', async () => {
    vi.mocked(api.fetchFiles).mockResolvedValue({
      resourceType: 'Images',
      currentFolder: { path: '/', url: '', acl: 255 },
      files: [
        { name: 'apple.png', date: '2026-01-01', size: 500, url: '', thumb: '' },
        { name: 'banana.png', date: '2026-01-01', size: 500, url: '', thumb: '' },
      ],
    });
    vi.mocked(api.fetchFolders).mockResolvedValue({
      resourceType: 'Images',
      currentFolder: { path: '/', url: '', acl: 255 },
      folders: [],
    });

    render(<FileManager />);

    await waitFor(() => {
      expect(screen.getByText('apple.png')).toBeInTheDocument();
      expect(screen.getByText('banana.png')).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText('Tìm kiếm file...');
    fireEvent.change(searchInput, { target: { value: 'apple' } });

    expect(screen.getByText('apple.png')).toBeInTheDocument();
    expect(screen.queryByText('banana.png')).toBeNull();
  });

  it('opens upload modal when upload button is clicked', async () => {
    vi.mocked(api.fetchFiles).mockResolvedValue({
      resourceType: 'Images',
      currentFolder: { path: '/', url: '', acl: 255 },
      files: [],
    });
    vi.mocked(api.fetchFolders).mockResolvedValue({
      resourceType: 'Images',
      currentFolder: { path: '/', url: '', acl: 255 },
      folders: [],
    });

    render(<FileManager />);

    const uploadButtons = screen.getAllByRole('button', { name: /Upload/i });
    fireEvent.click(uploadButtons[0]);

    expect(useFileManagerStore.getState().showUpload).toBe(true);
  });
});
