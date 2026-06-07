import { create } from 'zustand';
import type { FileInfo, FolderInfo, ResourceTypeInfo } from '../api/filemanager';

interface FileManagerState {
  // Auth
  isAuthenticated: boolean;
  setAuthenticated: (v: boolean) => void;

  // Resource types
  resourceTypes: ResourceTypeInfo[];
  setResourceTypes: (rts: ResourceTypeInfo[]) => void;
  activeResourceType: string;
  setActiveResourceType: (type: string) => void;

  // Navigation
  currentPath: string;
  setCurrentPath: (path: string) => void;

  // Folder tree
  folders: FolderInfo[];
  setFolders: (folders: FolderInfo[]) => void;
  folderRefreshKey: number;       // tăng lên mỗi khi cần reload folder tree
  refreshFolderTree: () => void;
  expandedPaths: Set<string>;
  togglePathExpanded: (path: string) => void;
  setExpandedPaths: (paths: Set<string>) => void;

  // Files
  files: FileInfo[];
  setFiles: (files: FileInfo[]) => void;
  selectedFiles: Set<string>;
  toggleSelectFile: (name: string) => void;
  selectAllFiles: () => void;
  clearSelection: () => void;

  // View mode
  viewMode: 'grid' | 'list';
  setViewMode: (mode: 'grid' | 'list') => void;

  // Modals
  showUpload: boolean;
  setShowUpload: (v: boolean) => void;
  showNewFolder: boolean;
  setShowNewFolder: (v: boolean) => void;
  renameTarget: { type: 'file' | 'folder'; name: string; path: string } | null;
  setRenameTarget: (t: FileManagerState['renameTarget']) => void;
  previewFile: FileInfo | null;
  setPreviewFile: (f: FileInfo | null) => void;

  // Upload progress
  uploadProgress: Record<string, number>; // filename → percent
  setUploadProgress: (filename: string, percent: number) => void;
  clearUploadProgress: (filename: string) => void;

  // Clipboard
  clipboard: { action: 'copy' | 'cut'; type: string; path: string; files: string[] } | null;
  setClipboard: (cb: FileManagerState['clipboard']) => void;
  clearClipboard: () => void;
}

export const useFileManagerStore = create<FileManagerState>((set, get) => ({
  // Auth
  isAuthenticated: !!localStorage.getItem('access_token'),
  setAuthenticated: (v) => set({ isAuthenticated: v }),

  // Resource types
  resourceTypes: [],
  setResourceTypes: (rts) => set({ resourceTypes: rts }),
  activeResourceType: 'Images',
  setActiveResourceType: (type) => set({ activeResourceType: type, currentPath: '/', selectedFiles: new Set(), expandedPaths: new Set() }),

  // Navigation
  currentPath: '/',
  setCurrentPath: (path) => set({ currentPath: path, selectedFiles: new Set() }),

  // Folder tree
  folders: [],
  setFolders: (folders) => set({ folders }),
  folderRefreshKey: 0,
  refreshFolderTree: () => set((s) => ({ folderRefreshKey: s.folderRefreshKey + 1 })),
  expandedPaths: new Set(),
  togglePathExpanded: (path) => set((s) => {
    const next = new Set(s.expandedPaths);
    if (next.has(path)) next.delete(path);
    else next.add(path);
    return { expandedPaths: next };
  }),
  setExpandedPaths: (expandedPaths) => set({ expandedPaths }),

  // Files
  files: [],
  setFiles: (files) => set({ files }),
  selectedFiles: new Set(),
  toggleSelectFile: (name) => {
    const sel = new Set(get().selectedFiles);
    if (sel.has(name)) sel.delete(name);
    else sel.add(name);
    set({ selectedFiles: sel });
  },
  selectAllFiles: () => set({ selectedFiles: new Set(get().files.map((f) => f.name)) }),
  clearSelection: () => set({ selectedFiles: new Set() }),

  // View mode
  viewMode: 'grid',
  setViewMode: (viewMode) => set({ viewMode }),

  // Modals
  showUpload: false,
  setShowUpload: (v) => set({ showUpload: v }),
  showNewFolder: false,
  setShowNewFolder: (v) => set({ showNewFolder: v }),
  renameTarget: null,
  setRenameTarget: (t) => set({ renameTarget: t }),
  previewFile: null,
  setPreviewFile: (f) => set({ previewFile: f }),

  // Upload progress
  uploadProgress: {},
  setUploadProgress: (filename, percent) =>
    set((s) => ({ uploadProgress: { ...s.uploadProgress, [filename]: percent } })),
  clearUploadProgress: (filename) =>
    set((s) => {
      const p = { ...s.uploadProgress };
      delete p[filename];
      return { uploadProgress: p };
    }),

  // Clipboard
  clipboard: null,
  setClipboard: (clipboard) => set({ clipboard }),
  clearClipboard: () => set({ clipboard: null }),
}));
