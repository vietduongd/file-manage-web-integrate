import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { StatsWidget } from '../../components/StatsWidget';
import * as api from '../../api/filemanager';

vi.mock('../../api/filemanager', () => ({
  fetchStats: vi.fn(),
}));

describe('StatsWidget Component', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders stats fetched from API', async () => {
    vi.mocked(api.fetchStats).mockResolvedValueOnce({
      totalCount: 5,
      totalSize: 1048576, // 1.00 MB
      breakdown: {
        Images: { count: 3, size: 524288 },
        Videos: { count: 1, size: 262144 },
        Files: { count: 1, size: 262144 },
      },
    });

    render(<StatsWidget />);

    await waitFor(() => {
      expect(screen.getByText('Dung lượng lưu trữ')).toBeInTheDocument();
      expect(screen.getByText('1.00 MB')).toBeInTheDocument();
      expect(screen.getByText('Ảnh (3)')).toBeInTheDocument();
      expect(screen.getByText('Video (1)')).toBeInTheDocument();
      expect(screen.getByText('Tập tin (1)')).toBeInTheDocument();
    });
  });

  it('re-fetches stats on clicking refresh button', async () => {
    vi.mocked(api.fetchStats).mockResolvedValue({
      totalCount: 0,
      totalSize: 0,
      breakdown: {},
    });

    render(<StatsWidget />);

    await waitFor(() => {
      expect(api.fetchStats).toHaveBeenCalled();
    });

    const initialCalls = vi.mocked(api.fetchStats).mock.calls.length;

    const refreshBtn = screen.getByTitle('Làm mới thống kê');
    fireEvent.click(refreshBtn);

    await waitFor(() => {
      expect(api.fetchStats).toHaveBeenCalledTimes(initialCalls + 1);
    });
  });
});
