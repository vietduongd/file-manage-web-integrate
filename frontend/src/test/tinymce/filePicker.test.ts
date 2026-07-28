import { describe, it, expect, vi } from 'vitest';
import { buildEmbedUrl, mediaManagerFilePicker } from '../../tinymce/filePicker';

describe('TinyMCE filePicker adapter', () => {
  describe('buildEmbedUrl', () => {
    it('constructs correct embed URL query parameters', () => {
      const url = buildEmbedUrl({
        baseUrl: 'http://localhost:3000/embed',
        ticket: 'ticket-xyz',
        multiple: true,
      });

      expect(url).toContain('mode=iframe');
      expect(url).toContain('multiple=1');
      expect(url).toContain('ticket=ticket-xyz');
      expect(url).toContain('origin=http%3A%2F%2Flocalhost');
    });

    it('supports function-based ticket parameter', () => {
      const url = buildEmbedUrl({
        baseUrl: 'http://localhost:3000/embed',
        ticket: () => 'dynamic-ticket',
      });

      expect(url).toContain('ticket=dynamic-ticket');
    });
  });

  describe('mediaManagerFilePicker', () => {
    it('opens TinyMCE windowManager URL dialog and handles selection message', () => {
      const mockEditor = {
        windowManager: {
          openUrl: vi.fn(),
        },
        insertContent: vi.fn(),
      };

      const callback = vi.fn();
      const pickerFn = mediaManagerFilePicker({
        baseUrl: 'http://localhost:3000/embed',
        ticket: 'ticket-123',
        multiple: true,
      });

      pickerFn.call(mockEditor, callback, '', { filetype: 'image' });

      expect(mockEditor.windowManager.openUrl).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'Chọn file',
          width: 1000,
          height: 650,
        })
      );

      const openUrlArg = mockEditor.windowManager.openUrl.mock.calls[0][0];
      const mockApi = { close: vi.fn() };

      const messageDetails = {
        mceAction: 'mediaManagerSelect',
        data: {
          url: 'http://example.com/file1.png',
          urls: ['http://example.com/file1.png', 'http://example.com/file2.png'],
        },
      };

      openUrlArg.onMessage(mockApi, messageDetails);

      expect(callback).toHaveBeenCalledWith('http://example.com/file1.png', { alt: '' });
      expect(mockEditor.insertContent).toHaveBeenCalledWith('<img src="http://example.com/file2.png" alt="" />');
      expect(mockApi.close).toHaveBeenCalled();
    });
  });
});
