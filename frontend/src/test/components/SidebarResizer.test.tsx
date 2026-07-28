import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { SidebarResizer } from '../../components/SidebarResizer';

describe('SidebarResizer Component', () => {
  it('renders resizer element with role separator', () => {
    render(<SidebarResizer width={240} onResize={vi.fn()} />);
    const resizer = screen.getByRole('separator');
    expect(resizer).toBeInTheDocument();
    expect(resizer).toHaveAttribute('aria-label', 'Kéo để đổi bề rộng thanh bên');
  });

  it('triggers onResize when pointer events occur', () => {
    const onResizeMock = vi.fn();
    render(<SidebarResizer width={240} onResize={onResizeMock} />);
    const resizer = screen.getByRole('separator');

    resizer.setPointerCapture = vi.fn();
    resizer.hasPointerCapture = vi.fn().mockReturnValue(true);
    resizer.releasePointerCapture = vi.fn();

    fireEvent.pointerDown(resizer, { clientX: 100, pointerId: 1 });
    expect(resizer.setPointerCapture).toHaveBeenCalledWith(1);

    fireEvent.pointerMove(resizer, { clientX: 150, pointerId: 1 });
    expect(onResizeMock).toHaveBeenCalledWith(290); // 240 + (150 - 100)

    fireEvent.pointerUp(resizer, { pointerId: 1 });
    expect(resizer.releasePointerCapture).toHaveBeenCalledWith(1);
  });
});
