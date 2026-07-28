import { useRef } from 'react';

interface SidebarResizerProps {
  /** Bề rộng hiện tại, dùng làm mốc khi bắt đầu kéo */
  width: number;
  /** Gọi liên tục trong lúc kéo; bên nhận tự clamp và lưu */
  onResize: (width: number) => void;
}

/**
 * Thanh kéo ở mép phải sidebar. Chỉ làm một việc: quy đổi thao tác kéo
 * thành bề rộng mới. Không biết gì về store hay cây thư mục.
 */
export function SidebarResizer({ width, onResize }: SidebarResizerProps) {
  const startX = useRef(0);
  const startWidth = useRef(0);

  const handlePointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    startX.current = e.clientX;
    startWidth.current = width;
    e.currentTarget.setPointerCapture(e.pointerId);
  };

  const handlePointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!e.currentTarget.hasPointerCapture(e.pointerId)) return;
    onResize(startWidth.current + (e.clientX - startX.current));
  };

  const handlePointerUp = (e: React.PointerEvent<HTMLDivElement>) => {
    if (e.currentTarget.hasPointerCapture(e.pointerId)) {
      e.currentTarget.releasePointerCapture(e.pointerId);
    }
  };

  return (
    <div
      className="sidebar-resizer"
      role="separator"
      aria-orientation="vertical"
      aria-label="Kéo để đổi bề rộng thanh bên"
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      onPointerCancel={handlePointerUp}
    />
  );
}
