import { useCallback, useState } from 'react';
import { useDropzone } from 'react-dropzone';
import { Upload, X, CheckCircle, AlertCircle } from 'lucide-react';
import { uploadFile } from '../api/filemanager';
import { useFileManagerStore } from '../store/fileManagerStore';

interface UploadItem {
  file: File;
  status: 'pending' | 'uploading' | 'done' | 'error';
  progress: number;
  error?: string;
}

interface UploadModalProps {
  onClose: () => void;
  onUploaded: () => void;
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}

export function UploadModal({ onClose, onUploaded }: UploadModalProps) {
  const [items, setItems] = useState<UploadItem[]>([]);
  const [uploading, setUploading] = useState(false);
  const { activeResourceType, currentPath } = useFileManagerStore();

  const updateItem = (index: number, updates: Partial<UploadItem>) => {
    setItems((prev) => prev.map((item, i) => (i === index ? { ...item, ...updates } : item)));
  };

  const onDrop = useCallback((accepted: File[]) => {
    const newItems = accepted.map((f) => ({
      file: f,
      status: 'pending' as const,
      progress: 0,
    }));
    setItems((prev) => [...prev, ...newItems]);
  }, []);

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    multiple: true,
  });

  const handleUpload = async () => {
    setUploading(true);
    const pending = items.filter((i) => i.status === 'pending');
    let uploadedCount = 0;

    for (let i = 0; i < items.length; i++) {
      if (items[i].status !== 'pending') continue;
      updateItem(i, { status: 'uploading', progress: 0 });
      try {
        await uploadFile(activeResourceType, currentPath, items[i].file, (p) => {
          updateItem(i, { progress: p });
        });
        updateItem(i, { status: 'done', progress: 100 });
        uploadedCount++;
      } catch (err: any) {
        updateItem(i, { status: 'error', error: err?.response?.data?.error?.message || 'Upload thất bại' });
      }
    }

    setUploading(false);
    if (pending.length > 0 && uploadedCount > 0) onUploaded();
  };

  const removeItem = (index: number) => {
    setItems((prev) => prev.filter((_, i) => i !== index));
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" style={{ maxWidth: 520 }} onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <span className="modal-title">Upload File</span>
          <button className="modal-close" onClick={onClose}><X size={16} /></button>
        </div>
        <div className="modal-body">
          <div {...getRootProps()} className={`dropzone ${isDragActive ? 'active' : ''}`}>
            <input {...getInputProps()} />
            <div className="dropzone-icon"><Upload size={32} /></div>
            <p className="dropzone-text">
              {isDragActive ? 'Thả file vào đây...' : 'Kéo thả file hoặc click để chọn'}
            </p>
            <p className="dropzone-hint">Đang upload vào: {activeResourceType} / {currentPath}</p>
          </div>

          {items.length > 0 && (
            <div className="upload-list">
              {items.map((item, i) => (
                <div key={i} className="upload-item">
                  <span className="upload-item-name" title={item.file.name}>{item.file.name}</span>
                  <span className="upload-item-size">{formatBytes(item.file.size)}</span>
                  {item.status === 'uploading' && (
                    <div className="progress-bar">
                      <div className="progress-fill" style={{ width: `${item.progress}%` }} />
                    </div>
                  )}
                  {item.status === 'done' && <CheckCircle size={16} color="var(--success)" />}
                  {item.status === 'error' && (
                    <span title={item.error}><AlertCircle size={16} color="var(--danger)" /></span>
                  )}
                  {item.status === 'pending' && (
                    <button
                      onClick={() => removeItem(i)}
                      style={{ background: 'transparent', color: 'var(--text-muted)', padding: 2 }}
                    >
                      <X size={14} />
                    </button>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
        <div className="modal-footer">
          <button className="toolbar-btn" onClick={onClose}>Đóng</button>
          <button
            className="toolbar-btn primary"
            onClick={handleUpload}
            disabled={uploading || items.filter((i) => i.status === 'pending').length === 0}
          >
            <Upload size={14} />
            {uploading ? 'Đang upload...' : `Upload ${items.filter((i) => i.status === 'pending').length} file`}
          </button>
        </div>
      </div>
    </div>
  );
}
