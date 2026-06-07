import './index.css';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Toaster } from 'react-hot-toast';
import { useFileManagerStore } from './store/fileManagerStore';
import { LoginPage } from './components/LoginPage';
import { FileManager } from './components/FileManager';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, staleTime: 30_000 },
  },
});

export default function App() {
  const isAuthenticated = useFileManagerStore((s) => s.isAuthenticated);

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
