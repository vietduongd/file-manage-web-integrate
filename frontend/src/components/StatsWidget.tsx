import { useEffect, useRef, useState } from "react";
import { Database, RefreshCw } from "lucide-react";
import { fetchStats, type StorageStats } from "../api/filemanager";
import { useFileManagerStore } from "../store/fileManagerStore";

function formatBytes(bytes: number) {
  if (bytes === 0) return "0 B";
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
  return (bytes / (1024 * 1024)).toFixed(2) + " MB";
}

const LIMIT_BYTES = 1000 * 1024 * 1024; // 1000 MB mock limit

type StatsWidgetProps = {
  /** 'panel': khối đầy đủ (sidebar). 'header': nút gọn trên header + popover chi tiết. */
  variant?: "panel" | "header";
};

export function StatsWidget({ variant = "panel" }: StatsWidgetProps) {
  const [stats, setStats] = useState<StorageStats | null>(null);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const { folderRefreshKey } = useFileManagerStore();

  const loadStats = () => {
    setLoading(true);
    fetchStats()
      .then((data) => setStats(data))
      .catch((err) => console.error("Failed to load stats", err))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    loadStats();
  }, [folderRefreshKey]);

  // Đóng popover khi click ra ngoài / nhấn Escape
  useEffect(() => {
    if (!open) return;
    const handleClick = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    };
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", handleClick);
    document.addEventListener("keydown", handleKey);
    return () => {
      document.removeEventListener("mousedown", handleClick);
      document.removeEventListener("keydown", handleKey);
    };
  }, [open]);

  if (!stats) return null;

  const { totalSize, breakdown } = stats;

  const imagesSize = breakdown["Images"]?.size || 0;
  const videosSize = breakdown["Videos"]?.size || 0;
  const filesSize = breakdown["Files"]?.size || 0;

  const imagesPct = Math.min((imagesSize / LIMIT_BYTES) * 100, 100);
  const videosPct = Math.min((videosSize / LIMIT_BYTES) * 100, 100);
  const filesPct = Math.min((filesSize / LIMIT_BYTES) * 100, 100);

  const progressBar = (
    <div className="stats-progress-bar">
      <div
        className="progress-segment images"
        style={{ width: `${imagesPct}%` }}
        title={`Images: ${formatBytes(imagesSize)}`}
      />
      <div
        className="progress-segment videos"
        style={{ width: `${videosPct}%` }}
        title={`Videos: ${formatBytes(videosSize)}`}
      />
      <div
        className="progress-segment files"
        style={{ width: `${filesPct}%` }}
        title={`Files: ${formatBytes(filesSize)}`}
      />
    </div>
  );

  const header = (
    <div className="stats-widget-header">
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 6,
          fontWeight: 600,
          fontSize: 12,
        }}
      >
        <Database size={13} color="var(--accent)" />
        <span>Dung lượng lưu trữ</span>
      </div>
      <button
        className="stats-refresh-btn"
        onClick={loadStats}
        disabled={loading}
        title="Làm mới thống kê"
      >
        <RefreshCw size={11} className={loading ? "spinner" : ""} />
      </button>
    </div>
  );

  const body = (
    <div className="stats-widget-body">
      <div className="stats-total">
        <span className="stats-total-used">{formatBytes(totalSize)}</span>
        <span className="stats-total-limit"> / 1000 MB</span>
      </div>

      <div className="stats-progress-container">{progressBar}</div>

      <div className="stats-breakdown">
        <div className="stats-breakdown-item">
          <span className="bullet images" />
          <span className="label">Ảnh ({breakdown["Images"]?.count || 0})</span>
          <span className="val">{formatBytes(imagesSize)}</span>
        </div>
        <div className="stats-breakdown-item">
          <span className="bullet videos" />
          <span className="label">Video ({breakdown["Videos"]?.count || 0})</span>
          <span className="val">{formatBytes(videosSize)}</span>
        </div>
        <div className="stats-breakdown-item">
          <span className="bullet files" />
          <span className="label">
            Tập tin ({breakdown["Files"]?.count || 0})
          </span>
          <span className="val">{formatBytes(filesSize)}</span>
        </div>
      </div>
    </div>
  );

  if (variant === "header") {
    return (
      <div className="stats-header" ref={rootRef}>
        <button
          className={`stats-header-trigger ${open ? "active" : ""}`}
          onClick={() => setOpen((v) => !v)}
          title={`Dung lượng lưu trữ: ${formatBytes(totalSize)} / 1000 MB`}
          aria-expanded={open}
        >
          <Database size={13} color="var(--accent)" />
          <span className="stats-header-text">
            <span className="stats-header-used">{formatBytes(totalSize)}</span>
            <span className="stats-header-limit"> / 1000 MB</span>
          </span>
          <span className="stats-header-bar">{progressBar}</span>
        </button>
        {open && (
          <div className="stats-popover">
            {header}
            {body}
          </div>
        )}
      </div>
    );
  }

  return (
    <div className="stats-widget">
      {header}
      {body}
    </div>
  );
}
