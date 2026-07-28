# Sidebar folder tree: sửa indent và làm responsive

Ngày: 2026-07-28
Phạm vi: `file-manage-web-integrate/frontend`

## Vấn đề

Cây thư mục ở sidebar sâu 4 cấp thì tên thư mục bị cắt gần hết, chỉ còn 3–4 ký tự rồi `...`.

Nguyên nhân: indent được áp dụng **hai lần**, cộng dồn.

- `src/components/FolderTree.tsx:48` — `style={{ paddingLeft: 8 + depth * 14 }}` trên chính item
- `src/index.css:190` — `.folder-children { padding-left: 16px }`, lồng nhau nên cộng dồn theo cấp

Mỗi cấp thụt vào 30px thay vì 14px như code có vẻ định.

Ngân sách bề ngang ở cấp 4 (depth = 3):

| Thành phần | px còn lại |
|---|---|
| `.sidebar` width | 240 |
| trừ padding `.folder-tree` (8 + 8) | 224 |
| trừ `.folder-children` × 3 cấp (16 × 3) | 176 |
| trừ padding item (8 + 14×3) và padding phải 8 | 118 |
| trừ toggle 16 + icon 14 + nút xoá 22 + 3 gap × 6 | 48 |

Còn 48px cho tên thư mục.

Hai vấn đề phụ phát hiện cùng lúc:

- `.folder-delete-btn` dùng `opacity: 0` chứ không phải `display: none`, nên chiếm 22px + 6px gap ở **mọi** hàng dù không nhìn thấy.
- `src/index.css` không có media query nào. `.sidebar` là `width: 240px; flex-shrink: 0`, nên trong iframe hẹp nó chiếm tỉ lệ rất lớn.

## Ngữ cảnh cần hỗ trợ

Đã chốt với người dùng:

- Iframe hẹp nhúng trong admin, bề ngang 600–900px
- Desktop mở trực tiếp, màn hình lớn

Không cần hỗ trợ tablet hay điện thoại. Chỉ dùng **một** breakpoint: 720px.

## Thiết kế

### 1. Một nguồn indent duy nhất

Xoá indent ở `.folder-children`, giữ indent trên item, điều khiển qua CSS variable thay vì số cứng inline:

```tsx
// FolderTree.tsx
style={{ '--depth': depth } as React.CSSProperties}
```

```css
.folder-tree { --indent-base: 8px; --indent-step: 12px; }
.folder-tree-item {
  padding-left: calc(var(--indent-base) + var(--depth, 0) * var(--indent-step));
}
.folder-children { padding-left: 0; }
```

Kết quả: 30px/cấp → 12px/cấp. Vì indent là padding chứ không phải margin, nền hover/active vẫn trải hết bề ngang.

`--depth` mặc định 0 để node gốc (`/ (Gốc)`) không cần khai báo biến vẫn đúng.

### 2. Đường guide dọc, không thêm DOM

Vẽ đúng `depth` đường kẻ 1px bằng gradient lặp trên chính item:

```css
.folder-tree-item {
  background-image: repeating-linear-gradient(to right,
    var(--border) 0 1px, transparent 1px var(--indent-step));
  background-size: calc(var(--depth, 0) * var(--indent-step)) 100%;
  background-position: var(--indent-base) 0;
  background-repeat: no-repeat;
}
```

Chỉ đụng `background-image`, không đụng `background-color`, nên hover / active / drag-over giữ nguyên hành vi hiện tại.

### 3. Đòi lại bề ngang từ nút xoá

```css
.folder-tree-item { position: relative; }
.folder-delete-btn { position: absolute; right: 4px; }
.folder-tree-item:hover .folder-name { padding-right: 26px; }
```

Nút xoá overlay lên tên thay vì chiếm chỗ cố định. `padding-right` khi hover để nút không đè lên phần chữ đang đọc.

Thêm `title={name}` vào `.folder-name` để hover ra tooltip tên đầy đủ khi bị cắt. Dùng native tooltip, không thêm dependency.

Tổng hiệu quả của mục 1 + 3: tên thư mục ở cấp 4 đi từ 48px lên khoảng 130px, đủ chứa ~18 ký tự.

### 4. Sidebar kéo rộng được

- `.sidebar { width: var(--sidebar-w, 240px) }`
- Component mới `src/components/SidebarResizer.tsx`: thanh kéo 4px `cursor: col-resize` ở mép phải sidebar, dùng pointer events với `setPointerCapture`.
- Clamp bề rộng trong khoảng 180–420px.
- Lưu vào `localStorage` khoá `mm.sidebarWidth`, đọc lại lúc khởi tạo.

`SidebarResizer` chỉ làm một việc là báo bề rộng mới ra ngoài. Nó không biết gì về `FolderTree` hay store, nhận `onResize` qua prop.

Thanh kéo chỉ render khi sidebar đang mở **và** bề ngang từ 720px trở lên. Khi sidebar đóng thì không có gì để kéo; ở chế độ overlay dưới 720px thì bề rộng do breakpoint quyết định, kéo tay sẽ mâu thuẫn với `--sidebar-w`. Giá trị `mm.sidebarWidth` đã lưu vẫn được giữ nguyên, không bị ghi đè khi đi qua hai trạng thái này.

### 5. Thu gọn và overlay khi hẹp

- Thêm `sidebarOpen: boolean` và `toggleSidebar()` vào `src/store/fileManagerStore.ts`. Dùng store sẵn có thay vì tạo state rời rạc.
- Giá trị khởi tạo: `window.innerWidth >= 720`.
- Nút hamburger đặt cạnh logo trong `FileManager.tsx` (khoảng dòng 411), **luôn hiện** ở mọi bề ngang để người dùng chủ động bật tắt.
- Dưới 720px: `.sidebar` chuyển `position: absolute` phủ lên lưới file, có `box-shadow` và một backdrop; bấm backdrop thì đóng; chọn thư mục xong tự đóng.
- Từ 720px trở lên: sidebar nằm trong luồng flex như hiện tại; khi đóng thì `width: 0`.

## Tệp bị ảnh hưởng

| Tệp | Thay đổi |
|---|---|
| `src/index.css` | indent variable, guide lines, nút xoá overlay, `.sidebar` width variable, media query 720px, style backdrop và nút hamburger |
| `src/components/FolderTree.tsx` | thay `paddingLeft` inline bằng `--depth`, thêm `title={name}`, tự đóng sidebar khi chọn thư mục ở chế độ hẹp |
| `src/components/FileManager.tsx` | nút hamburger, backdrop, gắn `SidebarResizer` vào `<aside>` |
| `src/components/SidebarResizer.tsx` | tệp mới |
| `src/store/fileManagerStore.ts` | `sidebarOpen`, `toggleSidebar()` |

## Kiểm chứng

Frontend chưa có test framework — `package.json` chỉ có `dev`, `build`, `preview`; không vitest, không testing-library. Spec này **không** thêm test tự động; dựng test framework là việc riêng, nằm ngoài phạm vi.

Kiểm chứng bằng:

1. `npm run build` — `tsc` bắt lỗi type của `--depth` trong `React.CSSProperties` và của các trường store mới.
2. Checklist thủ công:
   - Cây 4 cấp ở 1440px, 900px, 640px — tên thư mục đọc được, không bị cắt còn vài ký tự
   - Kéo resize: chạm biên 180px và 420px thì dừng đúng; reload giữ nguyên bề rộng
   - Toggle sidebar ở cả trên và dưới 720px
   - Dưới 720px: backdrop đóng được, chọn thư mục thì sidebar tự đóng
   - Drag-drop file vào node ở cấp 4 vẫn hoạt động
   - Nút xoá thư mục vẫn bấm được khi đã chuyển sang overlay

## Ngoài phạm vi

- Không dựng test framework cho frontend
- Không hỗ trợ tablet hay điện thoại
- Không đổi mô hình điều hướng (giữ cây, không chuyển sang drill-down)
- Không refactor các phần khác của `FileManager.tsx`
