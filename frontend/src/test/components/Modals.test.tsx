import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { RenameModal, NewFolderModal, DeleteFolderModal, ConfirmModal, CompressModal } from '../../components/Modals';
import * as api from '../../api/filemanager';
import { useFileManagerStore } from '../../store/fileManagerStore';

vi.mock('../../api/filemanager', () => ({
  renameFile: vi.fn(),
  renameFolder: vi.fn(),
  createFolder: vi.fn(),
  deleteFolder: vi.fn(),
  compressFiles: vi.fn(),
}));

describe('Modals Components', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    useFileManagerStore.setState({
      activeResourceType: 'Images',
      currentPath: '/',
      renameTarget: null,
      deleteFolderTarget: null,
    });
  });

  describe('RenameModal', () => {
    it('returns null if renameTarget is null', () => {
      const { container } = render(<RenameModal onClose={vi.fn()} onRenamed={vi.fn()} />);
      expect(container.firstChild).toBeNull();
    });

    it('renames file when target is a file', async () => {
      useFileManagerStore.setState({ renameTarget: { type: 'file', name: 'old.png', path: '/' } });
      vi.mocked(api.renameFile).mockResolvedValueOnce({ success: true });

      const onClose = vi.fn();
      const onRenamed = vi.fn();

      render(<RenameModal onClose={onClose} onRenamed={onRenamed} />);

      expect(screen.getByText('Đổi tên file')).toBeInTheDocument();
      const input = screen.getByRole('textbox');
      fireEvent.change(input, { target: { value: 'new.png' } });
      fireEvent.click(screen.getByRole('button', { name: 'Đổi tên' }));

      await waitFor(() => {
        expect(api.renameFile).toHaveBeenCalledWith('Images', '/', 'old.png', 'new.png');
        expect(onRenamed).toHaveBeenCalled();
        expect(onClose).toHaveBeenCalled();
      });
    });
  });

  describe('NewFolderModal', () => {
    it('creates a new folder on submit', async () => {
      vi.mocked(api.createFolder).mockResolvedValueOnce({ success: true });
      const onClose = vi.fn();
      const onCreated = vi.fn();

      render(<NewFolderModal onClose={onClose} onCreated={onCreated} />);

      const input = screen.getByPlaceholderText('Thư mục mới');
      fireEvent.change(input, { target: { value: 'documents' } });
      fireEvent.submit(input.closest('form')!);

      await waitFor(() => {
        expect(api.createFolder).toHaveBeenCalledWith('Images', '/', 'documents');
        expect(onCreated).toHaveBeenCalled();
        expect(onClose).toHaveBeenCalled();
      });
    });
  });

  describe('DeleteFolderModal', () => {
    it('requires matching folder name to confirm deletion', async () => {
      useFileManagerStore.setState({ deleteFolderTarget: { name: 'docs', path: '/docs' } });
      vi.mocked(api.deleteFolder).mockResolvedValueOnce({ success: true });
      const onClose = vi.fn();
      const onDeleted = vi.fn();

      render(<DeleteFolderModal onClose={onClose} onDeleted={onDeleted} />);

      const deleteBtn = screen.getByRole('button', { name: 'Xóa thư mục' });
      expect(deleteBtn).toBeDisabled();

      const input = screen.getByPlaceholderText('docs');
      fireEvent.change(input, { target: { value: 'docs' } });
      expect(deleteBtn).not.toBeDisabled();

      fireEvent.click(deleteBtn);

      await waitFor(() => {
        expect(api.deleteFolder).toHaveBeenCalledWith('Images', '/docs');
        expect(onDeleted).toHaveBeenCalled();
        expect(onClose).toHaveBeenCalled();
      });
    });
  });

  describe('ConfirmModal', () => {
    it('renders title/message and handles confirm action', async () => {
      const onConfirm = vi.fn();
      const onClose = vi.fn();

      render(
        <ConfirmModal
          title="Xác nhận xóa"
          message="Bạn có chắc chắn?"
          onClose={onClose}
          onConfirm={onConfirm}
          isDanger={true}
        />
      );

      expect(screen.getByText('Xác nhận xóa')).toBeInTheDocument();
      expect(screen.getByText('Bạn có chắc chắn?')).toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', { name: 'Xác nhận' }));

      await waitFor(() => {
        expect(onConfirm).toHaveBeenCalled();
        expect(onClose).toHaveBeenCalled();
      });
    });
  });

  describe('CompressModal', () => {
    it('compresses selected files into zip archive', async () => {
      vi.mocked(api.compressFiles).mockResolvedValueOnce({ success: true });
      const onClose = vi.fn();
      const onCompressed = vi.fn();

      render(<CompressModal files={['a.png', 'b.png']} onClose={onClose} onCompressed={onCompressed} />);

      expect(screen.getByText('Nén 2 file đã chọn thành tập tin ZIP.')).toBeInTheDocument();
      fireEvent.click(screen.getByRole('button', { name: 'Nén' }));

      await waitFor(() => {
        expect(api.compressFiles).toHaveBeenCalledWith('Images', '/', ['a.png', 'b.png'], 'archive');
        expect(onCompressed).toHaveBeenCalled();
        expect(onClose).toHaveBeenCalled();
      });
    });
  });
});
