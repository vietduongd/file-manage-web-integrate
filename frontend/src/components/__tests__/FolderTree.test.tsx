import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { FolderTree } from '../FolderTree';
import * as api from '../../api/filemanager';
import { useFileManagerStore } from '../../store/fileManagerStore';

vi.mock('../../api/filemanager', () => ({
  fetchFolders: vi.fn(),
  moveFiles: vi.fn(),
}));

describe('FolderTree Component', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    useFileManagerStore.setState({
      activeResourceType: 'Images',
      currentPath: '/',
      folders: [],
      expandedPaths: new Set(),
    });
  });

  it('renders root folder and fetches top-level folders', async () => {
    vi.mocked(api.fetchFolders).mockResolvedValueOnce({
      resourceType: 'Images',
      currentFolder: { path: '/', url: '', acl: 255 },
      folders: [
        { name: 'photos', path: '/photos', hasChildren: true, acl: 255 },
        { name: 'documents', path: '/documents', hasChildren: false, acl: 255 },
      ],
    });

    render(<FolderTree />);

    expect(screen.getByText('/ (Gốc)')).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText('photos')).toBeInTheDocument();
      expect(screen.getByText('documents')).toBeInTheDocument();
    });
  });

  it('navigates to selected folder on click', async () => {
    vi.mocked(api.fetchFolders).mockResolvedValueOnce({
      resourceType: 'Images',
      currentFolder: { path: '/', url: '', acl: 255 },
      folders: [{ name: 'photos', path: '/photos', hasChildren: false, acl: 255 }],
    });

    render(<FolderTree />);

    await waitFor(() => {
      expect(screen.getByText('photos')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('photos'));

    expect(useFileManagerStore.getState().currentPath).toBe('/photos');
  });

  it('handles Expand All and Collapse All buttons', async () => {
    vi.mocked(api.fetchFolders).mockResolvedValue({
      resourceType: 'Images',
      currentFolder: { path: '/', url: '', acl: 255 },
      folders: [],
    });

    render(<FolderTree />);

    const expandBtn = screen.getByText('Mở rộng hết');
    const collapseBtn = screen.getByText('Thu gọn hết');

    fireEvent.click(collapseBtn);
    expect(useFileManagerStore.getState().expandedPaths.size).toBe(0);

    fireEvent.click(expandBtn);
    await waitFor(() => {
      expect(api.fetchFolders).toHaveBeenCalledWith('Images', '/');
    });
  });
});
