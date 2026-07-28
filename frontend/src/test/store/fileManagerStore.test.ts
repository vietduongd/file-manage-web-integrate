import { describe, it, expect, beforeEach } from 'vitest';
import {
  useFileManagerStore,
  clampSidebarWidth,
  SIDEBAR_MIN_WIDTH,
  SIDEBAR_MAX_WIDTH,
  SIDEBAR_DEFAULT_WIDTH,
} from '../../store/fileManagerStore';

describe('fileManagerStore helpers', () => {
  describe('clampSidebarWidth', () => {
    it('should return rounded width within min and max bounds', () => {
      expect(clampSidebarWidth(200)).toBe(200);
      expect(clampSidebarWidth(150.4)).toBe(SIDEBAR_MIN_WIDTH); // 150 < 180
      expect(clampSidebarWidth(500)).toBe(SIDEBAR_MAX_WIDTH); // 500 > 420
    });
  });
});

describe('useFileManagerStore state & actions', () => {
  beforeEach(() => {
    useFileManagerStore.setState({
      isAuthenticated: false,
      resourceTypes: [],
      activeResourceType: 'Images',
      currentPath: '/',
      folders: [],
      folderRefreshKey: 0,
      expandedPaths: new Set(),
      files: [],
      fileRefreshKey: 0,
      selectedFiles: new Set(),
      viewMode: 'grid',
      sidebarOpen: true,
      sidebarWidth: SIDEBAR_DEFAULT_WIDTH,
      showUpload: false,
      showNewFolder: false,
      renameTarget: null,
      deleteFolderTarget: null,
      previewFile: null,
      uploadProgress: {},
      clipboard: null,
    });
    localStorage.clear();
  });

  it('handles auth state', () => {
    expect(useFileManagerStore.getState().isAuthenticated).toBe(false);
    useFileManagerStore.getState().setAuthenticated(true);
    expect(useFileManagerStore.getState().isAuthenticated).toBe(true);
  });

  it('handles active resource type and resets navigation/selection state', () => {
    useFileManagerStore.setState({
      currentPath: '/subfolder',
      selectedFiles: new Set(['file1.jpg']),
      expandedPaths: new Set(['/subfolder']),
    });

    useFileManagerStore.getState().setActiveResourceType('Files');

    const state = useFileManagerStore.getState();
    expect(state.activeResourceType).toBe('Files');
    expect(state.currentPath).toBe('/');
    expect(state.selectedFiles.size).toBe(0);
    expect(state.expandedPaths.size).toBe(0);
  });

  it('handles currentPath changes and clears selected files', () => {
    useFileManagerStore.setState({ selectedFiles: new Set(['file1.jpg']) });
    useFileManagerStore.getState().setCurrentPath('/documents');

    const state = useFileManagerStore.getState();
    expect(state.currentPath).toBe('/documents');
    expect(state.selectedFiles.size).toBe(0);
  });

  it('handles folder tree refresh and expansion toggles', () => {
    const initialKey = useFileManagerStore.getState().folderRefreshKey;
    useFileManagerStore.getState().refreshFolderTree();
    expect(useFileManagerStore.getState().folderRefreshKey).toBe(initialKey + 1);

    useFileManagerStore.getState().togglePathExpanded('/photos');
    expect(useFileManagerStore.getState().expandedPaths.has('/photos')).toBe(true);

    useFileManagerStore.getState().togglePathExpanded('/photos');
    expect(useFileManagerStore.getState().expandedPaths.has('/photos')).toBe(false);

    useFileManagerStore.getState().setExpandedPaths(new Set(['/a', '/b']));
    expect(useFileManagerStore.getState().expandedPaths.size).toBe(2);
  });

  it('handles file selection actions', () => {
    const testFiles = [
      { name: 'a.png', date: '2026-01-01', size: 100, url: '/a.png', thumb: '/a.png' },
      { name: 'b.png', date: '2026-01-01', size: 200, url: '/b.png', thumb: '/b.png' },
    ];
    useFileManagerStore.getState().setFiles(testFiles);

    useFileManagerStore.getState().toggleSelectFile('a.png');
    expect(useFileManagerStore.getState().selectedFiles.has('a.png')).toBe(true);

    useFileManagerStore.getState().toggleSelectFile('b.png');
    expect(useFileManagerStore.getState().selectedFiles.size).toBe(2);

    useFileManagerStore.getState().toggleSelectFile('a.png');
    expect(useFileManagerStore.getState().selectedFiles.has('a.png')).toBe(false);

    useFileManagerStore.getState().selectOnlyFile('b.png');
    expect(useFileManagerStore.getState().selectedFiles.size).toBe(0);

    useFileManagerStore.getState().selectOnlyFile('a.png');
    expect(useFileManagerStore.getState().selectedFiles.has('a.png')).toBe(true);
    expect(useFileManagerStore.getState().selectedFiles.size).toBe(1);

    useFileManagerStore.getState().selectAllFiles();
    expect(useFileManagerStore.getState().selectedFiles.size).toBe(2);

    useFileManagerStore.getState().clearSelection();
    expect(useFileManagerStore.getState().selectedFiles.size).toBe(0);
  });

  it('handles sidebar actions and localStorage persistence', () => {
    useFileManagerStore.getState().toggleSidebar();
    expect(useFileManagerStore.getState().sidebarOpen).toBe(false);

    useFileManagerStore.getState().setSidebarWidth(300);
    expect(useFileManagerStore.getState().sidebarWidth).toBe(300);
    expect(localStorage.getItem('mm.sidebarWidth')).toBe('300');

    useFileManagerStore.getState().setSidebarWidth(500);
    expect(useFileManagerStore.getState().sidebarWidth).toBe(SIDEBAR_MAX_WIDTH);
  });

  it('handles modal visibility state', () => {
    useFileManagerStore.getState().setShowUpload(true);
    expect(useFileManagerStore.getState().showUpload).toBe(true);

    useFileManagerStore.getState().setShowNewFolder(true);
    expect(useFileManagerStore.getState().showNewFolder).toBe(true);

    useFileManagerStore.getState().setRenameTarget({ type: 'file', name: 'old.txt', path: '/' });
    expect(useFileManagerStore.getState().renameTarget?.name).toBe('old.txt');

    useFileManagerStore.getState().setDeleteFolderTarget({ name: 'docs', path: '/docs' });
    expect(useFileManagerStore.getState().deleteFolderTarget?.name).toBe('docs');
  });

  it('handles upload progress tracking', () => {
    useFileManagerStore.getState().setUploadProgress('img.png', 45);
    expect(useFileManagerStore.getState().uploadProgress['img.png']).toBe(45);

    useFileManagerStore.getState().clearUploadProgress('img.png');
    expect(useFileManagerStore.getState().uploadProgress['img.png']).toBeUndefined();
  });

  it('handles clipboard state', () => {
    const cb = { action: 'copy' as const, type: 'Images', path: '/', files: ['test.jpg'] };
    useFileManagerStore.getState().setClipboard(cb);
    expect(useFileManagerStore.getState().clipboard).toEqual(cb);

    useFileManagerStore.getState().clearClipboard();
    expect(useFileManagerStore.getState().clipboard).toBeNull();
  });
});
