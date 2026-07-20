import { useEffect, useState } from 'react';
import './index.css';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Toaster } from 'react-hot-toast';
import { useFileManagerStore } from './store/fileManagerStore';
import { LoginPage } from './components/LoginPage';
import { FileManager } from './components/FileManager';
import { fetchConfig, embedLogin } from './api/filemanager';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, staleTime: 30_000 },
  },
});

export default function App() {
  const { isAuthenticated, setAuthenticated, setResourceTypes, setActiveResourceType } = useFileManagerStore();
  const [checkingAuth, setCheckingAuth] = useState(true);

  useEffect(() => {
    const initAuth = async () => {
      try {
        const urlParams = new URLSearchParams(window.location.search);
        const ticket = urlParams.get('ticket');
        if (ticket) {
          await embedLogin(ticket);
          // Remove the ticket from URL to prevent re-processing
          window.history.replaceState({}, document.title, window.location.pathname);
        }

        const cfg = await fetchConfig();
        setResourceTypes(cfg.resourceTypes);
        if (cfg.resourceTypes.length > 0) {
          setActiveResourceType(cfg.resourceTypes[0].name);
        }
        setAuthenticated(true);
      } catch (err) {
        // Fallback: If fetchConfig or embedLogin fails, clear credentials and require login
        setAuthenticated(false);
      } finally {
        setCheckingAuth(false);
      }
    };

    initAuth();
  }, [setAuthenticated, setResourceTypes, setActiveResourceType]);

  if (checkingAuth) {
    return (
      <div className="app-loading-screen">
        <div className="app-loading-spinner"></div>
        <p className="app-loading-text">Đang tải cấu hình...</p>
      </div>
    );
  }

  return (
    <QueryClientProvider client={queryClient}>
      <Toaster
        position="top-right"
        toastOptions={{
          style: {
            background: 'var(--bg-card)',
            color: 'var(--text-primary)',
            border: '1px solid var(--border-light)',
          },
        }}
      />
      {isAuthenticated ? <FileManager /> : <LoginPage />}
    </QueryClientProvider>
  );
}
