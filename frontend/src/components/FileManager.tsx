import { useEffect, useState, useCallback } from 'react';
import {
  File, Image, Film, FileText, Archive,
  LayoutGrid, List, Upload, FolderPlus, Trash2,
  RefreshCw, ChevronRight, LogOut,
  Images, Folder, Video,
} from 'lucide-react';
import {
  fetchConfig, fetchFiles, deleteFiles,
  thumbnailUrl, type FileInfo, copyFiles, moveFiles, extractZip,
} from '../api/filemanager';
import { useFileManagerStore } from '../store/fileManagerStore';
import { FolderTree } from './FolderTree';
import { UploadModal } from './UploadModal';
import { RenameModal, NewFolderModal, ConfirmModal, CompressModal } from './Modals';
import { ContextMenu, PreviewModal } from './ContextMenu';
import { StatsWidget } from './StatsWidget';

function formatBytes(bytes: number) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(2) + ' MB';
}

function getFileIcon(name: string, size = 24) {
  const ext = name.split('.').pop()?.toLowerCase() ?? '';
  if (['jpg','jpeg','png','gif','webp','bmp','svg'].includes(ext)) return <Image size={size} color="var(--accent)" />;
  if (['mp4','webm','ogg','avi','mov','mkv'].includes(ext)) return <Film size={size} color="#a78bfa" />;
  if (['pdf','doc','docx','txt'].includes(ext)) return <FileText size={size} color="#34d399" />;
  if (['zip','rar','7z'].includes(ext)) return <Archive size={size} color="var(--warning)" />;
  return <File size={size} color="var(--text-muted)" />;
}

function getResourceIcon(name: string) {
  switch (name.toLowerCase()) {
    case 'images': return <Images size={15} />;
    case 'videos': return <Video size={15} />;
    default: return <Folder size={15} />;
  }
}

export function FileManager() {
  const store = useFileManagerStore();
  const {
    activeResourceType, setActiveResourceType,
    resourceTypes, setResourceTypes,
    currentPath, setCurrentPath,
    files, setFiles,
    selectedFiles, toggleSelectFile, selectAllFiles, clearSelection,
    viewMode, setViewMode,
    showUpload, setShowUpload,
    showNewFolder, setShowNewFolder,
    renameTarget, setRenameTarget,
    previewFile, setPreviewFile,
    setAuthenticated,
    refreshFolderTree,
    folderRefreshKey,
    clipboard, setClipboard, clearClipboard,
  } = store;

  const [loading, setLoading] = useState(false);
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; file: FileInfo } | null>(null);
  const [deleteConfirmTarget, setDeleteConfirmTarget] = useState<string[] | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [pasting, setPasting] = useState(false);
  const [compressTarget, setCompressTarget] = useState<string[] | null>(null);
  const [extractTarget, setExtractTarget] = useState<string | null>(null);

  // Integration detection (e.g. opened as a popup or iframe by CKEditor / TinyMCE)
  const urlParams = new URLSearchParams(window.location.search);
  const isIntegrationMode = urlParams.has('CKEditorFuncNum') || urlParams.get('mode') === 'popup' || urlParams.has('CKEditor');

  const loadFiles = useCallback(() => {
    setLoading(true);
    clearSelection();
    fetchFiles(activeResourceType, currentPath)
      .then((res) => setFiles(res.files))
      .catch(() => setFiles([]))
      .finally(() => setLoading(false));
  }, [activeResourceType, currentPath, folderRefreshKey]);

  const handleDeleteSelected = () => {
    if (selectedFiles.size === 0) return;
    setDeleteConfirmTarget(Array.from(selectedFiles));
  };

  const handleChooseFiles = useCallback((fileUrls: string[]) => {
    const funcNum = urlParams.get('CKEditorFuncNum');
    if (funcNum) {
      if (window.opener) {
        try {
          fileUrls.forEach((url) => {
            window.opener.CKEDITOR.tools.callFunction(funcNum, url);
          });
        } catch (e) {
          console.error(e);
        }
        window.close();
        return;
      }
    }

    const msg = {
      sender: 'media-manager',
      action: 'select',
      url: fileUrls[0] || '', // fallback cho độ tương thích đơn
      urls: fileUrls,         // danh sách toàn bộ các file được chọn
    };
    if (window.opener) {
      window.opener.postMessage(msg, '*');
      window.close();
    } else if (window.parent && window.parent !== window) {
      window.parent.postMessage(msg, '*');
    }
  }, [urlParams]);

  const handlePaste = useCallback(async () => {
    if (!clipboard) return;
    setPasting(true);
    try {
      const payloadFiles = clipboard.files.map(name => ({
        type: clipboard.type,
        path: clipboard.path,
        name: name
      }));
      const destination = {
        type: activeResourceType,
        path: currentPath,
        name: ''
      };

      if (clipboard.action === 'copy') {
        await copyFiles(payloadFiles, destination);
      } else {
        await moveFiles(payloadFiles, destination);
      }
      clearClipboard();
      loadFiles();
    } catch (err) {
      console.error("Paste failed", err);
    } finally {
      setPasting(false);
    }
  }, [clipboard, activeResourceType, currentPath, clearClipboard, loadFiles]);

  // Load config (resource types)
  useEffect(() => {
    fetchConfig()
      .then((cfg) => {
        setResourceTypes(cfg.resourceTypes);
        if (cfg.resourceTypes.length > 0 && !resourceTypes.length) {
          setActiveResourceType(cfg.resourceTypes[0].name);
        }
      })
      .catch(() => {});
  }, []);

  useEffect(() => { loadFiles(); }, [loadFiles]);

  const [dragOverPath, setDragOverPath] = useState<string | null>(null);

  const handleBreadcrumbDrop = async (e: React.DragEvent, targetPath: string) => {
    e.preventDefault();
    e.stopPropagation();
    setDragOverPath(null);
    try {
      const rawData = e.dataTransfer.getData('application/json');
      if (!rawData) return;
      const payload = JSON.parse(rawData);
      if (payload.action === 'move') {
        if (payload.sourcePath === targetPath && payload.sourceResourceType === activeResourceType) {
          return;
        }
        const filesToMove = payload.files.map((name: string) => ({
          type: payload.sourceResourceType,
          path: payload.sourcePath,
          name: name
        }));
        const destination = {
          type: activeResourceType,
          path: targetPath,
          name: ''
        };
        await moveFiles(filesToMove, destination);
        refreshFolderTree();
      }
    } catch (err) {
      console.error("Breadcrumb drop failed", err);
    }
  };

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const activeEl = document.activeElement;
      if (activeEl && (activeEl.tagName === 'INPUT' || activeEl.tagName === 'TEXTAREA' || activeEl.getAttribute('contenteditable') === 'true')) {
        return;
      }

      // Ctrl + A: select all files
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'a') {
        e.preventDefault();
        selectAllFiles();
        return;
      }

      // Ctrl + C: copy selected files
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'c') {
        if (selectedFiles.size > 0) {
          e.preventDefault();
          setClipboard({
            action: 'copy',
            type: activeResourceType,
            path: currentPath,
            files: Array.from(selectedFiles),
          });
        }
        return;
      }

      // Ctrl + X: cut selected files
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'x') {
        if (selectedFiles.size > 0) {
          e.preventDefault();
          setClipboard({
            action: 'cut',
            type: activeResourceType,
            path: currentPath,
            files: Array.from(selectedFiles),
          });
        }
        return;
      }

      // Ctrl + V: paste
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'v') {
        if (clipboard) {
          e.preventDefault();
          handlePaste();
        }
        return;
      }

      // Delete: delete selected
      if (e.key === 'Delete') {
        if (selectedFiles.size > 0) {
          e.preventDefault();
          handleDeleteSelected();
        }
        return;
      }

      // F2: rename first selected file
      if (e.key === 'F2') {
        if (selectedFiles.size === 1) {
          e.preventDefault();
          const name = Array.from(selectedFiles)[0];
          setRenameTarget({ type: 'file', name, path: currentPath });
        }
        return;
      }

      // Escape: clear selection
      if (e.key === 'Escape') {
        e.preventDefault();
        clearSelection();
        return;
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [selectedFiles, activeResourceType, currentPath, clipboard, selectAllFiles, setClipboard, clearSelection, handlePaste, handleDeleteSelected, setRenameTarget]);

  // Breadcrumb
  const breadcrumbParts = currentPath.split('/').filter(Boolean);
  const filteredFiles = files.filter(f => f.name.toLowerCase().includes(searchQuery.toLowerCase()));
  const [isEditingPath, setIsEditingPath] = useState(false);
  const [editPathInput, setEditPathInput] = useState('');

  const handleStartEditingPath = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      setEditPathInput(currentPath);
      setIsEditingPath(true);
    }
  };

  const handlePathInputSubmit = () => {
    let newPath = editPathInput.trim();
    if (newPath === '') {
      newPath = '/';
    }
    if (!newPath.startsWith('/')) {
      newPath = '/' + newPath;
    }
    if (!newPath.endsWith('/')) {
      newPath = newPath + '/';
    }
    newPath = newPath.replace(/\/+/g, '/');

    setCurrentPath(newPath);
    setIsEditingPath(false);
  };

  const handlePathInputKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      handlePathInputSubmit();
    } else if (e.key === 'Escape') {
      setIsEditingPath(false);
    }
  };

  const handlePathInputBlur = () => {
    handlePathInputSubmit();
  };



  const handleContextMenu = (e: React.MouseEvent, file: FileInfo) => {
    e.preventDefault();
    e.stopPropagation();
    setContextMenu({ x: e.clientX, y: e.clientY, file });
  };

  const handleLogout = () => {
    localStorage.clear();
    setAuthenticated(false);
  };

  return (
    <div className="app-layout">
      {/* Header */}
      <header className="app-header">
        <div className="logo">
          <Image size={20} />
          <span>Media Manager</span>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <span style={{ fontSize: 13, color: 'var(--text-muted)' }}>
            {activeResourceType} · {currentPath}
          </span>
          <button className="toolbar-btn" onClick={handleLogout} title="Đăng xuất">
            <LogOut size={14} />
          </button>
        </div>
      </header>

      <div className="app-body">
        {/* Sidebar */}
        <aside className="sidebar">
          <div className="sidebar-section">
            <div className="sidebar-section-title">Loại tài nguyên</div>
            {resourceTypes.map((rt) => (
              <button
                key={rt.name}
                className={`resource-type-btn ${activeResourceType === rt.name ? 'active' : ''}`}
                onClick={() => { setActiveResourceType(rt.name); setCurrentPath('/'); }}
              >
                {getResourceIcon(rt.name)}
                {rt.name}
              </button>
            ))}
          </div>
          <div className="sidebar-divider" />
          <div className="sidebar-section">
            <div className="sidebar-section-title">Thư mục</div>
          </div>
          <FolderTree />
          <div style={{ marginTop: 'auto' }}>
            <StatsWidget />
          </div>
        </aside>

        {/* Main panel */}
        <main className="main-panel" onClick={() => { clearSelection(); setContextMenu(null); }}>
          {/* Toolbar */}
          <div className="toolbar">
            <button className="toolbar-btn primary" onClick={() => setShowUpload(true)}>
              <Upload size={14} /> Upload
            </button>
            <button className="toolbar-btn" onClick={() => setShowNewFolder(true)}>
              <FolderPlus size={14} /> Thư mục mới
            </button>
            {selectedFiles.size > 0 && (
              <>
                <div className="toolbar-sep" />
                <button className="toolbar-btn danger" onClick={handleDeleteSelected}>
                  <Trash2 size={14} /> Xóa ({selectedFiles.size})
                </button>
                <button className="toolbar-btn" onClick={() => setCompressTarget(Array.from(selectedFiles))} title="Nén các file đã chọn thành file ZIP">
                  <Archive size={14} /> Nén ZIP ({selectedFiles.size})
                </button>
              </>
            )}
            {selectedFiles.size >= 1 && isIntegrationMode && (
              <>
                <div className="toolbar-sep" />
                <button
                  className="toolbar-btn primary"
                  onClick={(e) => {
                    e.stopPropagation();
                    const selectedNames = Array.from(selectedFiles);
                    const selectedFilesObj = files.filter(f => selectedNames.includes(f.name));
                    if (selectedFilesObj.length > 0) {
                      handleChooseFiles(selectedFilesObj.map(f => f.url));
                    }
                  }}
                >
                  Chọn file ({selectedFiles.size})
                </button>
              </>
            )}
            {clipboard && (
              <>
                <div className="toolbar-sep" />
                <button
                  className="toolbar-btn primary"
                  onClick={(e) => { e.stopPropagation(); handlePaste(); }}
                  disabled={pasting}
                  title={`Dán các file từ thư mục ${clipboard.path}`}
                >
                  Dán ({clipboard.files.length} file)
                </button>
              </>
            )}
            <div className="toolbar-spacer" />
            <input
              type="text"
              className="form-input"
              placeholder="Tìm kiếm file..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              style={{ width: 180, height: 32, padding: '4px 10px', fontSize: 13, marginRight: 8 }}
            />
            <button className="toolbar-btn" onClick={loadFiles} title="Làm mới" style={{ marginRight: 8 }}>
              <RefreshCw size={14} className={loading ? 'spinner' : ''} />
            </button>
            <div className="view-toggle">
              <button
                className={`view-toggle-btn ${viewMode === 'grid' ? 'active' : ''}`}
                onClick={() => setViewMode('grid')}
                title="Grid view"
              ><LayoutGrid size={14} /></button>
              <button
                className={`view-toggle-btn ${viewMode === 'list' ? 'active' : ''}`}
                onClick={() => setViewMode('list')}
                title="List view"
              ><List size={14} /></button>
            </div>
          </div>

          {/* Breadcrumb */}
          <div
            className={`breadcrumb ${isEditingPath ? 'editing' : ''}`}
            onClick={handleStartEditingPath}
            style={{ cursor: isEditingPath ? 'default' : 'text' }}
          >
            {isEditingPath ? (
              <input
                type="text"
                className="breadcrumb-input"
                value={editPathInput}
                onChange={(e) => setEditPathInput(e.target.value)}
                onKeyDown={handlePathInputKeyDown}
                onBlur={handlePathInputBlur}
                autoFocus
              />
            ) : (
              <>
                <span
                  className={`breadcrumb-item ${currentPath === '/' ? 'active' : ''} ${dragOverPath === '/' ? 'drag-over' : ''}`}
                  onClick={(e) => { e.stopPropagation(); setCurrentPath('/'); }}
                  onDragOver={(e) => e.preventDefault()}
                  onDragEnter={(e) => { e.preventDefault(); setDragOverPath('/'); }}
                  onDragLeave={() => setDragOverPath(null)}
                  onDrop={(e) => handleBreadcrumbDrop(e, '/')}
                >{activeResourceType}</span>
                {breadcrumbParts.map((part, i) => {
                  const path = '/' + breadcrumbParts.slice(0, i + 1).join('/') + '/';
                  const isLast = i === breadcrumbParts.length - 1;
                  return (
                    <span key={path} style={{ display: 'flex', alignItems: 'center', gap: 4 }} onClick={(e) => e.stopPropagation()}>
                      <span className="breadcrumb-sep"><ChevronRight size={12} /></span>
                      <span
                        className={`breadcrumb-item ${isLast ? 'active' : ''} ${dragOverPath === path ? 'drag-over' : ''}`}
                        onClick={(e) => { e.stopPropagation(); !isLast && setCurrentPath(path); }}
                        onDragOver={(e) => e.preventDefault()}
                        onDragEnter={(e) => { e.preventDefault(); setDragOverPath(path); }}
                        onDragLeave={() => setDragOverPath(null)}
                        onDrop={(e) => handleBreadcrumbDrop(e, path)}
                      >{part}</span>
                    </span>
                  );
                })}
              </>
            )}
          </div>

          {/* File area */}
          <div className="file-area" onClick={() => setContextMenu(null)}>
            {loading ? (
              <div className="empty-state">
                <RefreshCw size={32} className="spinner" />
                <p>Đang tải...</p>
              </div>
            ) : filteredFiles.length === 0 ? (
              <div className="empty-state">
                <Folder size={48} />
                <p>{searchQuery ? "Không tìm thấy file nào khớp với tìm kiếm" : "Thư mục trống"}</p>
                {!searchQuery && (
                  <button className="toolbar-btn primary" onClick={() => setShowUpload(true)}>
                    <Upload size={14} /> Upload file đầu tiên
                  </button>
                )}
              </div>
            ) : viewMode === 'grid' ? (
              <div className="file-grid">
                {filteredFiles.map((file) => {
                  const isImg = /\.(jpg|jpeg|png|gif|webp|bmp)$/i.test(file.name);
                  const sel = selectedFiles.has(file.name);
                  return (
                    <div
                      key={file.name}
                      className={`file-card ${sel ? 'selected' : ''}`}
                      draggable={true}
                      onDragStart={(e) => {
                        const payload = {
                          action: 'move',
                          sourceType: activeResourceType,
                          sourcePath: currentPath,
                          files: sel ? Array.from(selectedFiles) : [file.name],
                        };
                        e.dataTransfer.setData('application/json', JSON.stringify(payload));
                      }}
                      onClick={(e) => { e.stopPropagation(); toggleSelectFile(file.name); }}
                      onContextMenu={(e) => handleContextMenu(e, file)}
                      onDoubleClick={() => isIntegrationMode ? handleChooseFiles([file.url]) : setPreviewFile(file)}
                    >
                      <input
                        type="checkbox"
                        className="file-card-checkbox"
                        checked={sel}
                        onChange={() => toggleSelectFile(file.name)}
                        onClick={(e) => e.stopPropagation()}
                      />
                      <div className="file-card-thumb">
                        {isImg ? (
                          <img
                            src={thumbnailUrl(activeResourceType, currentPath, file.name)}
                            alt={file.name}
                            loading="lazy"
                            onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }}
                          />
                        ) : (
                          <div className="file-icon">{getFileIcon(file.name, 36)}</div>
                        )}
                      </div>
                      <div className="file-card-info">
                        <div className="file-card-name" title={file.name}>{file.name}</div>
                        <div className="file-card-size">{formatBytes(file.size)}</div>
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : (
              <table className="file-list">
                <thead>
                  <tr>
                    <th style={{ width: 32 }}>
                      <input type="checkbox" onChange={(e) => e.target.checked ? selectAllFiles() : clearSelection()} />
                    </th>
                    <th>Tên</th>
                    <th>Kích thước</th>
                    <th>Ngày sửa</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredFiles.map((file) => {
                    const isImg = /\.(jpg|jpeg|png|gif|webp|bmp)$/i.test(file.name);
                    const sel = selectedFiles.has(file.name);
                    return (
                      <tr
                        key={file.name}
                        className={sel ? 'selected' : ''}
                        draggable={true}
                        onDragStart={(e) => {
                          const payload = {
                            action: 'move',
                            sourceType: activeResourceType,
                            sourcePath: currentPath,
                            files: sel ? Array.from(selectedFiles) : [file.name],
                          };
                          e.dataTransfer.setData('application/json', JSON.stringify(payload));
                        }}
                        onClick={(e) => { e.stopPropagation(); toggleSelectFile(file.name); }}
                        onContextMenu={(e) => handleContextMenu(e, file)}
                        onDoubleClick={() => isIntegrationMode ? handleChooseFiles([file.url]) : setPreviewFile(file)}
                      >
                        <td>
                          <input
                            type="checkbox"
                            checked={sel}
                            onChange={() => toggleSelectFile(file.name)}
                            onClick={(e) => e.stopPropagation()}
                          />
                        </td>
                        <td>
                          <div className="file-row-icon">
                            {isImg ? (
                              <img
                                className="file-thumb-sm"
                                src={thumbnailUrl(activeResourceType, currentPath, file.name, 28, 28)}
                                alt=""
                                loading="lazy"
                              />
                            ) : getFileIcon(file.name, 16)}
                            {file.name}
                          </div>
                        </td>
                        <td>{formatBytes(file.size)}</td>
                        <td>{file.date.replace(/(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})/, '$3/$2/$1 $4:$5')}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>

          {/* Status bar */}
          <div className="status-bar">
            <span>
              {searchQuery ? `Tìm thấy ${filteredFiles.length} / ${files.length}` : `${files.length}`} file{files.length !== 1 ? 's' : ''}
            </span>
            {selectedFiles.size > 0 && <span>{selectedFiles.size} đã chọn</span>}
          </div>
        </main>
      </div>

      {/* Modals */}
      {showUpload && <UploadModal onClose={() => setShowUpload(false)} onUploaded={loadFiles} />}
      {showNewFolder && (
        <NewFolderModal
          onClose={() => setShowNewFolder(false)}
          onCreated={() => {
            loadFiles();
            refreshFolderTree();
          }}
        />
      )}
      {renameTarget && (
        <RenameModal
          onClose={() => setRenameTarget(null)}
          onRenamed={() => {
            loadFiles();
            refreshFolderTree();
          }}
        />
      )}
      {previewFile && <PreviewModal file={previewFile} onClose={() => setPreviewFile(null)} />}
      {deleteConfirmTarget && (
        <ConfirmModal
          title="Xác nhận xóa"
          message={
            deleteConfirmTarget.length === 1
              ? `Bạn có chắc chắn muốn xóa file "${deleteConfirmTarget[0]}"?`
              : `Bạn có chắc chắn muốn xóa ${deleteConfirmTarget.length} file đã chọn?`
          }
          isDanger
          confirmText="Xóa"
          onClose={() => setDeleteConfirmTarget(null)}
          onConfirm={async () => {
            await deleteFiles(activeResourceType, currentPath, deleteConfirmTarget);
            loadFiles();
          }}
        />
      )}
      {compressTarget && (
        <CompressModal
          files={compressTarget}
          onClose={() => setCompressTarget(null)}
          onCompressed={() => {
            loadFiles();
            setCompressTarget(null);
          }}
        />
      )}
      {extractTarget && (
        <ConfirmModal
          title="Xác nhận giải nén"
          message={`Bạn có chắc chắn muốn giải nén file ZIP "${extractTarget}" vào thư mục này?`}
          confirmText="Giải nén"
          onClose={() => setExtractTarget(null)}
          onConfirm={async () => {
            await extractZip(activeResourceType, currentPath, extractTarget);
            loadFiles();
            setExtractTarget(null);
          }}
        />
      )}

      {/* Context Menu */}
      {contextMenu && (
        <ContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          file={contextMenu.file}
          onClose={() => setContextMenu(null)}
          onDelete={(f) => {
            setDeleteConfirmTarget([f.name]);
          }}
          onRefresh={loadFiles}
          onCompress={(files) => setCompressTarget(files)}
          onExtract={(fileName) => setExtractTarget(fileName)}
        />
      )}
    </div>
  );
}
