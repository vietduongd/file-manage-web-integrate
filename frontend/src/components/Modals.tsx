import { useState } from 'react';
import { X } from 'lucide-react';
import { renameFile, renameFolder } from '../api/filemanager';
import { useFileManagerStore } from '../store/fileManagerStore';

interface RenameModalProps {
  onClose: () => void;
  onRenamed: () => void;
}

export function RenameModal({ onClose, onRenamed }: RenameModalProps) {
  const { renameTarget, activeResourceType } = useFileManagerStore();
  const [newName, setNewName] = useState(renameTarget?.name ?? '');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  if (!renameTarget) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newName.trim() || newName === renameTarget.name) return;
    setLoading(true);
    setError('');
    try {
      if (renameTarget.type === 'file') {
        await renameFile(activeResourceType, renameTarget.path, renameTarget.name, newName.trim());
      } else {
        await renameFolder(activeResourceType, renameTarget.path, newName.trim());
      }
      onRenamed();
      onClose();
    } catch (err: any) {
      setError(err?.response?.data?.error?.message || 'Đổi tên thất bại');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <span className="modal-title">
            Đổi tên {renameTarget.type === 'file' ? 'file' : 'thư mục'}
          </span>
          <button className="modal-close" onClick={onClose}><X size={16} /></button>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="modal-body">
            <label className="form-label">Tên mới</label>
            <input
              className="form-input"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              autoFocus
              required
            />
            {error && <p style={{ color: 'var(--danger)', fontSize: 13, marginTop: 8 }}>{error}</p>}
          </div>
          <div className="modal-footer">
            <button type="button" className="toolbar-btn" onClick={onClose}>Hủy</button>
            <button type="submit" className="toolbar-btn primary" disabled={loading}>
              {loading ? 'Đang đổi tên...' : 'Đổi tên'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

interface NewFolderModalProps {
  onClose: () => void;
  onCreated: () => void;
}

export function NewFolderModal({ onClose, onCreated }: NewFolderModalProps) {
  const [name, setName] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const { activeResourceType, currentPath } = useFileManagerStore();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    setLoading(true);
    setError('');
    try {
      const { createFolder } = await import('../api/filemanager');
      await createFolder(activeResourceType, currentPath, name.trim());
      onCreated();
      onClose();
    } catch (err: any) {
      setError(err?.response?.data?.error?.message || 'Tạo thư mục thất bại');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <span className="modal-title">Tạo thư mục mới</span>
          <button className="modal-close" onClick={onClose}><X size={16} /></button>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="modal-body">
            <label className="form-label">Tên thư mục</label>
            <input
              className="form-input"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Thư mục mới"
              autoFocus
              required
            />
            {error && <p style={{ color: 'var(--danger)', fontSize: 13, marginTop: 8 }}>{error}</p>}
          </div>
        </form>
      </div>
    </div>
  );
}

interface ConfirmModalProps {
  title: string;
  message: string;
  onClose: () => void;
  onConfirm: () => void | Promise<void>;
  confirmText?: string;
  cancelText?: string;
  isDanger?: boolean;
}

export function ConfirmModal({
  title,
  message,
  onClose,
  onConfirm,
  confirmText = 'Xác nhận',
  cancelText = 'Hủy',
  isDanger = false
}: ConfirmModalProps) {
  const [loading, setLoading] = useState(false);

  const handleConfirm = async () => {
    setLoading(true);
    try {
      await onConfirm();
      onClose();
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <span className="modal-title">{title}</span>
          <button className="modal-close" onClick={onClose}><X size={16} /></button>
        </div>
        <div className="modal-body">
          <p style={{ fontSize: 14, color: 'var(--text-primary)' }}>{message}</p>
        </div>
        <div className="modal-footer">
          <button type="button" className="toolbar-btn" onClick={onClose} disabled={loading}>{cancelText}</button>
          <button
            type="button"
            className={`toolbar-btn ${isDanger ? 'danger' : 'primary'}`}
            style={isDanger ? { background: 'var(--danger)', color: 'white', borderColor: 'var(--danger)' } : {}}
            onClick={handleConfirm}
            disabled={loading}
          >
            {loading ? 'Đang thực hiện...' : confirmText}
          </button>
        </div>
      </div>
    </div>
  );
}

interface CompressModalProps {
  files: string[];
  onClose: () => void;
  onCompressed: () => void;
}

export function CompressModal({ files, onClose, onCompressed }: CompressModalProps) {
  const { activeResourceType, currentPath } = useFileManagerStore();
  const [zipName, setZipName] = useState('archive');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!zipName.trim()) return;
    setLoading(true);
    setError('');
    try {
      const { compressFiles } = await import('../api/filemanager');
      await compressFiles(activeResourceType, currentPath, files, zipName.trim());
      onCompressed();
      onClose();
    } catch (err: any) {
      setError(err?.response?.data?.error?.message || 'Nén file thất bại');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <span className="modal-title">Nén thành ZIP</span>
          <button className="modal-close" onClick={onClose}><X size={16} /></button>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="modal-body">
            <p style={{ fontSize: 13, color: 'var(--text-muted)', marginBottom: 12 }}>
              Nén {files.length} file đã chọn thành tập tin ZIP.
            </p>
            <label className="form-label">Tên tập tin ZIP</label>
            <input
              className="form-input"
              value={zipName}
              onChange={(e) => setZipName(e.target.value)}
              placeholder="archive"
              autoFocus
              required
            />
            {error && <p style={{ color: 'var(--danger)', fontSize: 13, marginTop: 8 }}>{error}</p>}
          </div>
          <div className="modal-footer">
            <button type="button" className="toolbar-btn" onClick={onClose}>Hủy</button>
            <button type="submit" className="toolbar-btn primary" disabled={loading}>
              {loading ? 'Đang nén...' : 'Nén'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
