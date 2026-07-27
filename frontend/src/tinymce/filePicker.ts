/**
 * TinyMCE 8 file picker — mở media manager trong URL dialog
 *
 * Cách dùng:
 *   import { mediaManagerFilePicker } from './tinymce/filePicker';
 *
 *   tinymce.init({
 *     selector: '#editor',
 *     file_picker_types: 'image media file',
 *     file_picker_callback: mediaManagerFilePicker({
 *       baseUrl: 'https://file.example.com/embed',
 *       ticket: () => currentTicket,
 *     }),
 *   });
 */

interface MediaManagerPayload {
  url: string;
  urls: string[];
}

export interface FilePickerOptions {
  /** URL của trang embed, ví dụ 'https://file.example.com/embed' */
  baseUrl: string;
  /** Ticket đăng nhập embed. Dùng hàm nếu ticket được refresh theo phiên. */
  ticket?: string | (() => string);
  /** Cho chọn nhiều file. TinyMCE file_picker chỉ nhận 1 URL nên mặc định false. */
  multiple?: boolean;
  dialogTitle?: string;
  width?: number;
  height?: number;
}

type FilePickerCallback = (
  callback: (url: string, meta?: Record<string, unknown>) => void,
  value: string,
  meta: { filetype: string }
) => void;

function isMediaManagerMessage(details: unknown): details is { data: MediaManagerPayload } {
  if (!details || typeof details !== 'object') return false;
  const d = details as Record<string, unknown>;
  if (d.mceAction !== 'mediaManagerSelect') return false;
  const data = d.data as Record<string, unknown> | undefined;
  return !!data && typeof data.url === 'string';
}

export function buildEmbedUrl(opts: FilePickerOptions): string {
  const url = new URL(opts.baseUrl, window.location.href);
  url.searchParams.set('mode', 'iframe');
  url.searchParams.set('multiple', opts.multiple ? '1' : '0');
  url.searchParams.set('origin', window.location.origin);
  const ticket = typeof opts.ticket === 'function' ? opts.ticket() : opts.ticket;
  if (ticket) url.searchParams.set('ticket', ticket);
  return url.toString();
}

/**
 * Trả về hàm dùng cho `file_picker_callback` của TinyMCE.
 *
 * Khi multiple = true, file đầu tiên được đưa vào ô input của dialog và các
 * file còn lại được chèn thẳng vào nội dung editor — vì file_picker_callback
 * chỉ nhận được đúng một URL.
 */
export function mediaManagerFilePicker(opts: FilePickerOptions): FilePickerCallback {
  return function filePicker(this: any, callback) {
    // TinyMCE bind `this` là editor khi gọi file_picker_callback
    const editor = this;
    editor.windowManager.openUrl({
      title: opts.dialogTitle || 'Chọn file',
      url: buildEmbedUrl(opts),
      width: opts.width || 1000,
      height: opts.height || 650,
      onMessage: (api: any, details: unknown) => {
        if (!isMediaManagerMessage(details)) return;
        const { url, urls } = details.data;
        callback(url, { alt: '' });
        if (opts.multiple && urls.length > 1) {
          urls.slice(1).forEach((extra) => {
            editor.insertContent(`<img src="${extra}" alt="" />`);
          });
        }
        api.close();
      },
    });
  };
}
