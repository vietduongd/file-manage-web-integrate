import { describe, it, expect, vi, beforeEach } from 'vitest';
import { CKFinderUploadAdapterPlugin } from '../../ckeditor/uploadAdapter';

describe('CKFinderUploadAdapterPlugin', () => {
  let mockXHR: any;

  beforeEach(() => {
    mockXHR = {
      open: vi.fn(),
      send: vi.fn(),
      abort: vi.fn(),
      addEventListener: vi.fn(),
      upload: {
        addEventListener: vi.fn(),
      },
      response: null,
    };
    class MockXMLHttpRequest {
      open = mockXHR.open;
      send = mockXHR.send;
      abort = mockXHR.abort;
      addEventListener = mockXHR.addEventListener;
      upload = mockXHR.upload;
      get response() {
        return mockXHR.response;
      }
      withCredentials = false;
      responseType = '';
    }
    vi.stubGlobal('XMLHttpRequest', MockXMLHttpRequest);
  });

  it('registers upload adapter on FileRepository', async () => {
    let adapterInstance: any;
    const mockFile = new File(['test content'], 'image.png', { type: 'image/png' });
    const mockLoader = {
      file: Promise.resolve(mockFile),
      uploadTotal: 0,
      uploaded: 0,
    };

    const mockEditor = {
      plugins: {
        get: vi.fn().mockReturnValue({
          createUploadAdapter: null,
        }),
      },
      config: {
        get: vi.fn().mockReturnValue({
          apiUrl: 'http://localhost:8080',
          resourceType: 'Images',
          uploadPath: '/',
        }),
      },
    };

    const fileRepoMock = { createUploadAdapter: null as any };
    mockEditor.plugins.get.mockReturnValue(fileRepoMock);

    CKFinderUploadAdapterPlugin(mockEditor);

    expect(fileRepoMock.createUploadAdapter).toBeDefined();
    adapterInstance = fileRepoMock.createUploadAdapter(mockLoader);

    const uploadPromise = adapterInstance.upload();
    await Promise.resolve();

    expect(mockXHR.open).toHaveBeenCalledWith('POST', 'http://localhost:8080/api/upload/ck', true);
    expect(mockXHR.send).toHaveBeenCalled();

    const loadHandler = mockXHR.addEventListener.mock.calls.find(([event]: [string]) => event === 'load')[1];
    mockXHR.response = { uploaded: true, url: 'http://example.com/uploaded.png' };
    loadHandler();

    const result = await uploadPromise;
    expect(result).toEqual({ default: 'http://example.com/uploaded.png' });
  });

  it('handles abort correctly', async () => {
    const mockEditor = {
      plugins: { get: vi.fn().mockReturnValue({}) },
      config: { get: vi.fn() },
    };
    const fileRepoMock = { createUploadAdapter: null as any };
    mockEditor.plugins.get.mockReturnValue(fileRepoMock);

    CKFinderUploadAdapterPlugin(mockEditor);

    const mockLoader = { file: Promise.resolve(new File([], 'test.png')), uploadTotal: 0, uploaded: 0 };
    const adapter = fileRepoMock.createUploadAdapter(mockLoader);

    adapter.upload();
    await Promise.resolve();
    adapter.abort();

    expect(mockXHR.abort).toHaveBeenCalled();
  });
});
