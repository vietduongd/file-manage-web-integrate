import { useEffect, useRef } from 'react';
import {
  Download, Pencil, Trash2, Copy, Scissors, Eye, Archive,
} from 'lucide-react';
import { downloadFile } from '../api/filemanager';
import { useFileManagerStore } from '../store/fileManagerStore';
import type { FileInfo } from '../api/filemanager';

interface ContextMenuProps {
  x: number;
  y: number;
  file: FileInfo;
  onClose: () => void;
  onDelete: (file: FileInfo) => void;
  onRefresh: () => void;
  onCompress: (files: string[]) => void;
  onExtract: (fileName: string) => void;
}

export function ContextMenu({ x, y, file, onClose, onDelete, onRefresh, onCompress, onExtract }: ContextMenuProps) {
  const menuRef = useRef<HTMLDivElement>(null);
  const { activeResourceType, currentPath, setRenameTarget, setPreviewFile, selectedFiles, setClipboard } = useFileManagerStore();

  // Close on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [onClose]);

  // Adjust position to stay in viewport
  const style: React.CSSProperties = {
    top: Math.min(y, window.innerHeight - 260),
    left: Math.min(x, window.innerWidth - 200),
  };

  const handleDownload = async () => {
    onClose();
    try {
      const { url } = await downloadFile(activeResourceType, currentPath, file.name);
      window.open(url, '_blank');
    } catch { /* ignore */ }
  };

  const handleRename = () => {
    onClose();
    setRenameTarget({ type: 'file', name: file.name, path: currentPath });
  };

  const handlePreview = () => {
    onClose();
    setPreviewFile(file);
  };

  const handleDelete = () => {
    onClose();
    onDelete(file);
  };

  const getSelectedFiles = () => {
    if (selectedFiles.has(file.name)) {
      return Array.from(selectedFiles);
    }
    return [file.name];
  };

  const handleCopy = () => {
    onClose();
    setClipboard({
      action: 'copy',
      type: activeResourceType,
      path: currentPath,
      files: getSelectedFiles()
    });
  };

  const handleCut = () => {
    onClose();
    setClipboard({
      action: 'cut',
      type: activeResourceType,
      path: currentPath,
      files: getSelectedFiles()
    });
  };

  const isZip = file.name.toLowerCase().endsWith('.zip');

  return (
    <div className="context-menu" ref={menuRef} style={style}>
      <button className="context-menu-item" onClick={handlePreview}>
        <Eye size={14} /> Xem trước
      </button>
      <button className="context-menu-item" onClick={handleDownload}>
        <Download size={14} /> Tải xuống
      </button>
      <div className="context-menu-sep" />
      <button className="context-menu-item" onClick={handleRename}>
        <Pencil size={14} /> Đổi tên
      </button>
      <button className="context-menu-item" onClick={handleCopy}>
        <Copy size={14} /> Sao chép
      </button>
      <button className="context-menu-item" onClick={handleCut}>
        <Scissors size={14} /> Di chuyển (Cắt)
      </button>
      <div className="context-menu-sep" />
      {isZip && (
        <button className="context-menu-item" onClick={() => { onClose(); onExtract(file.name); }}>
          <Archive size={14} /> Giải nén ZIP
        </button>
      )}
      <button className="context-menu-item" onClick={() => { onClose(); onCompress(getSelectedFiles()); }}>
        <Archive size={14} /> Nén thành ZIP
      </button>
      <div className="context-menu-sep" />
      <button className="context-menu-item danger" onClick={handleDelete}>
        <Trash2 size={14} /> Xóa
      </button>
    </div>
  );
}

// ── Preview Modal ─────────────────────────────────────────────────────────

interface PreviewModalProps {
  file: FileInfo;
  onClose: () => void;
}

function isImageUrl(url: string) {
  return /\.(jpg|jpeg|png|gif|webp|bmp|svg)(\?|$)/i.test(url);
}

export function PreviewModal({ file, onClose }: PreviewModalProps) {
  return (
    <div className="modal-overlay preview-modal" onClick={onClose}>
      <div className="modal" style={{ maxWidth: '90vw', width: 'auto' }} onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <span className="modal-title" style={{ maxWidth: 400, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {file.name}
          </span>
          <button className="modal-close" onClick={onClose}>✕</button>
        </div>
        <div className="modal-body preview-content">
          {isImageUrl(file.url) ? (
            <img src={file.url} alt={file.name} />
          ) : (
            <div style={{ padding: 40, color: 'var(--text-muted)', textAlign: 'center' }}>
              <p style={{ fontSize: 14 }}>Không thể xem trước file này.</p>
              <a href={file.url} target="_blank" rel="noreferrer" style={{ color: 'var(--accent)', marginTop: 12, display: 'block' }}>
                Mở trong tab mới
              </a>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
