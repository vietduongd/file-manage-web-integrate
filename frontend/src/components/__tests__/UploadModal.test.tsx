import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { UploadModal } from '../UploadModal';
import * as api from '../../api/filemanager';

vi.mock('../../api/filemanager', () => ({
  uploadFile: vi.fn(),
}));

describe('UploadModal Component', () => {
  const onClose = vi.fn();
  const onUploaded = vi.fn();

  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders modal with dropzone', () => {
    render(<UploadModal onClose={onClose} onUploaded={onUploaded} />);

    expect(screen.getByText('Upload File')).toBeInTheDocument();
    expect(screen.getByText('Kéo thả file hoặc click để chọn')).toBeInTheDocument();
  });

  it('allows file selection and uploading', async () => {
    vi.mocked(api.uploadFile).mockResolvedValue({
      fileName: 'test.jpg',
      uploaded: 1,
      url: '/test.jpg',
    });

    render(<UploadModal onClose={onClose} onUploaded={onUploaded} />);
    const dropzone = screen.getByText('Kéo thả file hoặc click để chọn').closest('.dropzone')!;

    const testFile = new File(['content'], 'test.jpg', { type: 'image/jpeg' });
    fireEvent.drop(dropzone, {
      dataTransfer: {
        files: [testFile],
        items: [{ kind: 'file', type: 'image/jpeg', getAsFile: () => testFile }],
        types: ['Files'],
      },
    });

    await waitFor(() => {
      expect(screen.getByText('test.jpg')).toBeInTheDocument();
    });

    const uploadBtn = screen.getByRole('button', { name: /Upload 1 file/i });
    fireEvent.click(uploadBtn);

    await waitFor(() => {
      expect(api.uploadFile).toHaveBeenCalled();
      expect(onUploaded).toHaveBeenCalled();
    });
  });

  it('calls onClose when close button clicked', () => {
    render(<UploadModal onClose={onClose} onUploaded={onUploaded} />);
    const closeButtons = screen.getAllByRole('button', { name: /Đóng/i });
    fireEvent.click(closeButtons[0]);
    expect(onClose).toHaveBeenCalled();
  });
});
