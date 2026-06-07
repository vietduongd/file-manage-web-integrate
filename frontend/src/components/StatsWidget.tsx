import { useEffect, useState } from "react";
import { Database, RefreshCw } from "lucide-react";
import { fetchStats, type StorageStats } from "../api/filemanager";
import { useFileManagerStore } from "../store/fileManagerStore";

function formatBytes(bytes: number) {
  if (bytes === 0) return "0 B";
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
  return (bytes / (1024 * 1024)).toFixed(2) + " MB";
}

export function StatsWidget() {
  const [stats, setStats] = useState<StorageStats | null>(null);
  const [loading, setLoading] = useState(false);
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

  if (!stats) return null;

  const { totalCount, totalSize, breakdown } = stats;

  const limitBytes = 1000 * 1024 * 1024; // 1000 MB mock limit
  const pct = Math.min((totalSize / limitBytes) * 100, 100);

  const imagesSize = breakdown["Images"]?.size || 0;
  const videosSize = breakdown["Videos"]?.size || 0;
  const filesSize = breakdown["Files"]?.size || 0;

  const imagesPct = Math.min((imagesSize / limitBytes) * 100, 100);
  const videosPct = Math.min((videosSize / limitBytes) * 100, 100);
  const filesPct = Math.min((filesSize / limitBytes) * 100, 100);

  return (
    <div className="stats-widget">
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

      <div className="stats-widget-body">
        <div className="stats-total">
          <span className="stats-total-used">{formatBytes(totalSize)}</span>
          <span className="stats-total-limit"> / 1000 MB</span>
        </div>

        <div className="stats-progress-container">
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
        </div>

        <div className="stats-breakdown">
          <div className="stats-breakdown-item">
            <span className="bullet images" />
            <span className="label">
              Ảnh ({breakdown["Images"]?.count || 0})
            </span>
            <span className="val">{formatBytes(imagesSize)}</span>
          </div>
          <div className="stats-breakdown-item">
            <span className="bullet videos" />
            <span className="label">
              Video ({breakdown["Videos"]?.count || 0})
            </span>
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
    </div>
  );
}
