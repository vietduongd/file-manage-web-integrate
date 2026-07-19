import { useEffect, useState } from 'react';
import {
  ChevronRight,
  ChevronDown,
  Folder,
  FolderOpen,
  Trash2,
} from 'lucide-react';
import { fetchFolders, moveFiles, type FolderInfo } from '../api/filemanager';
import { useFileManagerStore } from '../store/fileManagerStore';

interface FolderTreeNodeProps {
  name: string;
  path: string;
  resourceType: string;
  depth: number;
  hasChildren: boolean;
}

function FolderTreeNode({ name, path, resourceType, depth, hasChildren }: FolderTreeNodeProps) {
  const [children, setChildren] = useState<FolderInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [isDragOver, setIsDragOver] = useState(false);
  const { currentPath, setCurrentPath, folderRefreshKey, expandedPaths, togglePathExpanded, refreshFolderTree, setDeleteFolderTarget } = useFileManagerStore();
  const isActive = currentPath === path;
  const isExpanded = expandedPaths.has(path);

  const handleToggle = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!hasChildren) return;
    togglePathExpanded(path);
  };

  useEffect(() => {
    if (isExpanded && hasChildren) {
      setLoading(true);
      fetchFolders(resourceType, path)
        .then((res) => setChildren(res.folders))
        .catch(() => {})
        .finally(() => setLoading(false));
    }
  }, [isExpanded, hasChildren, resourceType, path, folderRefreshKey]);

  return (
    <div>
      <div
        className={`folder-tree-item ${isActive ? 'active' : ''} ${isDragOver ? 'drag-over' : ''}`}
        style={{ paddingLeft: `${8 + depth * 14}px` }}
        onClick={() => setCurrentPath(path)}
        onDragOver={(e) => { e.preventDefault(); e.stopPropagation(); }}
        onDragEnter={(e) => { e.preventDefault(); e.stopPropagation(); setIsDragOver(true); }}
        onDragLeave={(e) => { e.preventDefault(); e.stopPropagation(); setIsDragOver(false); }}
        onDrop={async (e) => {
          e.preventDefault();
          e.stopPropagation();
          setIsDragOver(false);
          try {
            const rawData = e.dataTransfer.getData('application/json');
            if (!rawData) return;
            const payload = JSON.parse(rawData);
            if (payload.action === 'move') {
              if (payload.sourcePath === path && payload.sourceResourceType === resourceType) {
                return;
              }
              const filesToMove = payload.files.map((name: string) => ({
                type: payload.sourceResourceType,
                path: payload.sourcePath,
                name: name
              }));
              const destination = {
                type: resourceType,
                path: path,
                name: ''
              };
              await moveFiles(filesToMove, destination);
              refreshFolderTree();
            }
          } catch (err) {
            console.error("Tree drop failed", err);
          }
        }}
      >
        <span className="toggle-icon" onClick={handleToggle}>
          {hasChildren ? (
            loading ? <span className="spinner" style={{ display: 'inline-block', width: 12, height: 12 }}>↻</span>
              : isExpanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />
          ) : null}
        </span>
        <span className="folder-icon">
          {isActive || isExpanded ? <FolderOpen size={14} /> : <Folder size={14} />}
        </span>
        <span className="folder-name">{name}</span>
        <button
          className="folder-delete-btn"
          title="Xóa thư mục"
          onClick={(e) => {
            e.stopPropagation();
            setDeleteFolderTarget({ name, path });
          }}
        >
          <Trash2 size={12} />
        </button>
      </div>
      {isExpanded && children.length > 0 && (
        <div className="folder-children">
          {children.map((f) => (
            <FolderTreeNode
              key={f.path}
              name={f.name}
              path={f.path}
              resourceType={resourceType}
              depth={depth + 1}
              hasChildren={f.hasChildren}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export function FolderTree() {
  const { activeResourceType, currentPath, setCurrentPath, folders, setFolders, folderRefreshKey, setExpandedPaths, refreshFolderTree } = useFileManagerStore();
  const [loading, setLoading] = useState(false);
  const [expandingAll, setExpandingAll] = useState(false);
  const [isRootDragOver, setIsRootDragOver] = useState(false);

  useEffect(() => {
    setLoading(true);
    fetchFolders(activeResourceType, '/')
      .then((res) => setFolders(res.folders))
      .catch(() => setFolders([]))
      .finally(() => setLoading(false));
  }, [activeResourceType, setFolders, folderRefreshKey]);

  const handleExpandAll = async () => {
    setExpandingAll(true);
    const newExpanded = new Set<string>();

    async function traverse(p: string) {
      try {
        const res = await fetchFolders(activeResourceType, p);
        for (const f of res.folders) {
          if (f.hasChildren) {
            newExpanded.add(f.path);
            await traverse(f.path);
          }
        }
      } catch (err) { /* ignore */ }
    }

    await traverse('/');
    setExpandedPaths(newExpanded);
    setExpandingAll(false);
  };

  const handleCollapseAll = () => {
    setExpandedPaths(new Set());
  };

  return (
    <div className="folder-tree">
      <div className="folder-tree-toolbar" style={{ display: 'flex', gap: 12, padding: '0 8px 8px', margin: '0 0 8px 0', fontSize: 11, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)' }}>
        <button 
          className="tree-action-btn" 
          onClick={handleExpandAll} 
          disabled={expandingAll || loading}
          style={{ background: 'transparent', color: 'inherit', padding: 0, display: 'flex', alignItems: 'center', gap: 4 }}
        >
          <ChevronDown size={12} /> {expandingAll ? 'Đang mở...' : 'Mở rộng hết'}
        </button>
        <button 
          className="tree-action-btn" 
          onClick={handleCollapseAll} 
          disabled={expandingAll || loading}
          style={{ background: 'transparent', color: 'inherit', padding: 0, display: 'flex', alignItems: 'center', gap: 4 }}
        >
          <ChevronRight size={12} /> Thu gọn hết
        </button>
      </div>

      {/* Root */}
      <div
        className={`folder-tree-item ${currentPath === '/' ? 'active' : ''} ${isRootDragOver ? 'drag-over' : ''}`}
        onClick={() => setCurrentPath('/')}
        onDragOver={(e) => { e.preventDefault(); e.stopPropagation(); }}
        onDragEnter={(e) => { e.preventDefault(); e.stopPropagation(); setIsRootDragOver(true); }}
        onDragLeave={(e) => { e.preventDefault(); e.stopPropagation(); setIsRootDragOver(false); }}
        onDrop={async (e) => {
          e.preventDefault();
          e.stopPropagation();
          setIsRootDragOver(false);
          try {
            const rawData = e.dataTransfer.getData('application/json');
            if (!rawData) return;
            const payload = JSON.parse(rawData);
            if (payload.action === 'move') {
              if (payload.sourcePath === '/' && payload.sourceResourceType === activeResourceType) {
                return;
              }
              const filesToMove = payload.files.map((name: string) => ({
                type: payload.sourceResourceType,
                path: payload.sourcePath,
                name: name
              }));
              const destination = {
                type: activeResourceType,
                path: '/',
                name: ''
              };
              await moveFiles(filesToMove, destination);
              refreshFolderTree();
            }
          } catch (err) {
            console.error("Root tree drop failed", err);
          }
        }}
      >
        <span className="toggle-icon" />
        <span className="folder-icon"><FolderOpen size={14} /></span>
        <span className="folder-name">/ (Gốc)</span>
      </div>

      {loading ? (
        <div style={{ padding: '8px 16px', fontSize: 12, color: 'var(--text-muted)' }}>Đang tải...</div>
      ) : (
        folders.map((f) => (
          <FolderTreeNode
            key={f.path}
            name={f.name}
            path={f.path}
            resourceType={activeResourceType}
            depth={0}
            hasChildren={f.hasChildren}
          />
        ))
      )}
    </div>
  );
}
