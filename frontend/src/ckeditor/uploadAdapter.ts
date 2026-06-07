/**
 * CKEditor 5 Custom Upload Adapter
 * Tích hợp với Go backend API để upload ảnh trực tiếp từ CKEditor
 *
 * Cách dùng:
 *   import { CKFinderUploadAdapterPlugin } from './ckeditor/uploadAdapter';
 *
 *   ClassicEditor.create(element, {
 *     extraPlugins: [CKFinderUploadAdapterPlugin],
 *     ckfinderBackend: {
 *       apiUrl: 'http://localhost:8080',
 *       token: localStorage.getItem('access_token'),
 *       resourceType: 'Images',
 *       uploadPath: '/',
 *     }
 *   });
 */

interface FileLoader {
  file: Promise<File>;
  uploadTotal: number;
  uploaded: number;
}

interface CKEditorConfig {
  ckfinderBackend?: {
    apiUrl: string;
    token: string;
    resourceType?: string;
    uploadPath?: string;
  };
}

class CKFinderUploadAdapter {
  private loader: FileLoader;
  private apiUrl: string;
  private token: string;
  private resourceType: string;
  private uploadPath: string;
  private xhr: XMLHttpRequest | null = null;

  constructor(loader: FileLoader, config: CKEditorConfig['ckfinderBackend']) {
    this.loader = loader;
    this.apiUrl = config?.apiUrl || '';
    this.token = config?.token || localStorage.getItem('access_token') || '';
    this.resourceType = config?.resourceType || 'Images';
    this.uploadPath = config?.uploadPath || '/';
  }

  upload(): Promise<{ default: string }> {
    return this.loader.file.then(
      (file: File) =>
        new Promise((resolve, reject) => {
          this._initRequest();
          this._initListeners(resolve, reject, file);
          this._sendRequest(file);
        })
    );
  }

  abort(): void {
    if (this.xhr) {
      this.xhr.abort();
    }
  }

  private _initRequest(): void {
    const xhr = (this.xhr = new XMLHttpRequest());
    xhr.open('POST', `${this.apiUrl}/api/upload/ck`, true);
    xhr.setRequestHeader('Authorization', `Bearer ${this.token}`);
    xhr.responseType = 'json';
  }

  private _initListeners(
    resolve: (value: { default: string }) => void,
    reject: (reason?: any) => void,
    file: File
  ): void {
    const xhr = this.xhr!;
    const loader = this.loader;
    const genericErrorText = `Không thể upload file: ${file.name}.`;

    xhr.addEventListener('error', () => reject(genericErrorText));
    xhr.addEventListener('abort', () => reject());

    xhr.addEventListener('load', () => {
      const response = xhr.response;
      if (!response || response.error) {
        return reject(response?.error?.message || genericErrorText);
      }
      if (response.uploaded) {
        resolve({ default: response.url });
      } else {
        reject(genericErrorText);
      }
    });

    if (xhr.upload) {
      xhr.upload.addEventListener('progress', (evt) => {
        if (evt.lengthComputable) {
          loader.uploadTotal = evt.total;
          loader.uploaded = evt.loaded;
        }
      });
    }
  }

  private _sendRequest(file: File): void {
    const data = new FormData();
    data.append('upload', file);
    data.append('type', this.resourceType);
    data.append('path', this.uploadPath);
    this.xhr!.send(data);
  }
}

/**
 * CKEditor 5 plugin function
 * Đăng ký custom upload adapter vào FileRepository
 */
export function CKFinderUploadAdapterPlugin(editor: any): void {
  editor.plugins.get('FileRepository').createUploadAdapter = (loader: FileLoader) => {
    const config = editor.config.get('ckfinderBackend') as CKEditorConfig['ckfinderBackend'];
    return new CKFinderUploadAdapter(loader, config);
  };
}

/**
 * Ví dụ tích hợp CKEditor 5:
 *
 * import ClassicEditor from '@ckeditor/ckeditor5-build-classic';
 * import { CKFinderUploadAdapterPlugin } from './ckeditor/uploadAdapter';
 *
 * ClassicEditor.create(document.querySelector('#editor'), {
 *   extraPlugins: [CKFinderUploadAdapterPlugin],
 *   ckfinderBackend: {
 *     apiUrl: 'http://localhost:8080',
 *     token: localStorage.getItem('access_token') || '',
 *     resourceType: 'Images',
 *     uploadPath: '/',
 *   },
 * }).then(editor => {
 *   console.log('CKEditor initialized with MinIO upload adapter');
 * });
 */
