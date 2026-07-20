import { useState } from 'react';
import { FolderOpen, Lock } from 'lucide-react';
import { login } from '../api/filemanager';
import { useFileManagerStore } from '../store/fileManagerStore';

export function LoginPage() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const setAuthenticated = useFileManagerStore((s) => s.setAuthenticated);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      await login(username, password);
      setAuthenticated(true);
    } catch {
      setError('Tên đăng nhập hoặc mật khẩu không đúng');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-page">
      <div className="login-card">
        <div className="login-logo">
          <FolderOpen size={28} />
          <span>Media Manager</span>
        </div>
        <form className="login-form" onSubmit={handleSubmit}>
          <div>
            <label className="form-label">Tên đăng nhập</label>
            <input
              className="form-input"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="admin"
              required
              autoFocus
            />
          </div>
          <div>
            <label className="form-label">Mật khẩu</label>
            <input
              className="form-input"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              required
            />
          </div>
          {error && <p className="login-error">{error}</p>}
          <button className="login-btn" type="submit" disabled={loading}>
            {loading ? 'Đang đăng nhập...' : <><Lock size={14} style={{ display: 'inline', marginRight: 6 }} />Đăng nhập</>}
          </button>
        </form>
      </div>
    </div>
  );
}
