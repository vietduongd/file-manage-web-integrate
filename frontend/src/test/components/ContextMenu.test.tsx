import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ContextMenu, PreviewModal } from '../../components/ContextMenu';
import { useFileManagerStore } from '../../store/fileManagerStore';

const dummyFile = {
  name: 'test-image.png',
  date: '2026-01-01',
  size: 1024,
  url: 'http://example.com/test-image.png',
  thumb: 'http://example.com/thumb.png',
};

const dummyZip = {
  name: 'archive.zip',
  date: '2026-01-01',
  size: 2048,
  url: 'http://example.com/archive.zip',
  thumb: '',
};

describe('ContextMenu Component', () => {
  const onClose = vi.fn();
  const onDelete = vi.fn();
  const onRefresh = vi.fn();
  const onCompress = vi.fn();
  const onExtract = vi.fn();

  beforeEach(() => {
    vi.restoreAllMocks();
    useFileManagerStore.setState({
      activeResourceType: 'Images',
      currentPath: '/',
      selectedFiles: new Set(),
      clipboard: null,
    });
  });

  it('renders context menu options for image file', () => {
    render(
      <ContextMenu
        x={100}
        y={100}
        file={dummyFile}
        onClose={onClose}
        onDelete={onDelete}
        onRefresh={onRefresh}
        onCompress={onCompress}
        onExtract={onExtract}
      />
    );

    expect(screen.getByText('Xem trước')).toBeInTheDocument();
    expect(screen.getByText('Tải xuống')).toBeInTheDocument();
    expect(screen.getByText('Đổi tên')).toBeInTheDocument();
    expect(screen.getByText('Sao chép')).toBeInTheDocument();
    expect(screen.getByText('Di chuyển (Cắt)')).toBeInTheDocument();
    expect(screen.getByText('Nén thành ZIP')).toBeInTheDocument();
    expect(screen.getByText('Xóa')).toBeInTheDocument();
    expect(screen.queryByText('Giải nén ZIP')).toBeNull();
  });

  it('renders extract zip option for .zip files', () => {
    render(
      <ContextMenu
        x={100}
        y={100}
        file={dummyZip}
        onClose={onClose}
        onDelete={onDelete}
        onRefresh={onRefresh}
        onCompress={onCompress}
        onExtract={onExtract}
      />
    );

    expect(screen.getByText('Giải nén ZIP')).toBeInTheDocument();
  });

  it('triggers copy action and updates store clipboard', () => {
    render(
      <ContextMenu
        x={100}
        y={100}
        file={dummyFile}
        onClose={onClose}
        onDelete={onDelete}
        onRefresh={onRefresh}
        onCompress={onCompress}
        onExtract={onExtract}
      />
    );

    fireEvent.click(screen.getByText('Sao chép'));

    expect(onClose).toHaveBeenCalled();
    expect(useFileManagerStore.getState().clipboard).toEqual({
      action: 'copy',
      type: 'Images',
      path: '/',
      files: ['test-image.png'],
    });
  });

  it('triggers delete callback', () => {
    render(
      <ContextMenu
        x={100}
        y={100}
        file={dummyFile}
        onClose={onClose}
        onDelete={onDelete}
        onRefresh={onRefresh}
        onCompress={onCompress}
        onExtract={onExtract}
      />
    );

    fireEvent.click(screen.getByText('Xóa'));

    expect(onClose).toHaveBeenCalled();
    expect(onDelete).toHaveBeenCalledWith(dummyFile);
  });
});

describe('PreviewModal Component', () => {
  it('renders image when file URL is an image', () => {
    render(<PreviewModal file={dummyFile} onClose={vi.fn()} />);

    const img = screen.getByRole('img');
    expect(img).toHaveAttribute('src', dummyFile.url);
    expect(img).toHaveAttribute('alt', dummyFile.name);
  });

  it('renders fallback notice when file URL is not an image', () => {
    const docFile = { ...dummyFile, name: 'file.pdf', url: 'http://example.com/file.pdf' };
    render(<PreviewModal file={docFile} onClose={vi.fn()} />);

    expect(screen.getByText('Không thể xem trước file này.')).toBeInTheDocument();
    expect(screen.getByText('Mở trong tab mới')).toBeInTheDocument();
  });
});
