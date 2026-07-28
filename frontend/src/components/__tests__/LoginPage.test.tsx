import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { LoginPage } from '../LoginPage';
import * as api from '../../api/filemanager';
import { useFileManagerStore } from '../../store/fileManagerStore';

vi.mock('../../api/filemanager', () => ({
  login: vi.fn(),
}));

describe('LoginPage Component', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    useFileManagerStore.setState({ isAuthenticated: false });
  });

  it('renders login form correctly', () => {
    render(<LoginPage />);

    expect(screen.getByText('Media Manager')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('admin')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('••••••••')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Đăng nhập/i })).toBeInTheDocument();
  });

  it('handles successful login submission', async () => {
    vi.mocked(api.login).mockResolvedValueOnce({
      access_token: 'token123',
      refresh_token: 'refresh123',
      expires_in: 3600,
      token_type: 'Bearer',
    });

    render(<LoginPage />);

    const usernameInput = screen.getByPlaceholderText('admin');
    const passwordInput = screen.getByPlaceholderText('••••••••');
    const submitBtn = screen.getByRole('button', { name: /Đăng nhập/i });

    fireEvent.change(usernameInput, { target: { value: 'admin' } });
    fireEvent.change(passwordInput, { target: { value: 'secret' } });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(api.login).toHaveBeenCalledWith('admin', 'secret');
      expect(useFileManagerStore.getState().isAuthenticated).toBe(true);
    });
  });

  it('displays error message on failed login', async () => {
    vi.mocked(api.login).mockRejectedValueOnce(new Error('Unauthorized'));

    render(<LoginPage />);

    fireEvent.change(screen.getByPlaceholderText('admin'), { target: { value: 'admin' } });
    fireEvent.change(screen.getByPlaceholderText('••••••••'), { target: { value: 'wrong' } });
    fireEvent.click(screen.getByRole('button', { name: /Đăng nhập/i }));

    await waitFor(() => {
      expect(screen.getByText('Tên đăng nhập hoặc mật khẩu không đúng')).toBeInTheDocument();
      expect(useFileManagerStore.getState().isAuthenticated).toBe(false);
    });
  });
});
