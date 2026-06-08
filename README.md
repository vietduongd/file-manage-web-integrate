# Media Manager — CKFinder Replacement with MinIO

Hệ thống quản lý file media tự chủ, thay thế CKFinder bằng:
- **Backend**: Go (Gin) — REST API + JWT authentication
- **Storage**: MinIO (S3-compatible object storage)
- **Frontend**: React 18 + TypeScript — Custom file manager UI
- **Integration**: CKEditor 5 custom upload adapter

## Kiến trúc

```
React UI  ──JWT──►  Go Backend  ──S3 API──►  MinIO
                    :8080                    :9000
```

## Bắt đầu nhanh

### 1. Cài đặt yêu cầu

- [Docker & Docker Compose](https://docs.docker.com/get-docker/)
- [Go 1.22+](https://go.dev/dl/) (để chạy backend local)
- [Node.js 20+](https://nodejs.org/) (để chạy frontend local)

### 2. Cấu hình

```bash
cp .env.example .env
# Chỉnh sửa .env nếu cần
```

### 3. Chạy với Docker Compose (đề xuất)

```bash
docker-compose up -d
```

Dịch vụ:
| Dịch vụ | URL |
|---|---|
| Frontend | http://localhost:3000 |
| Backend API | http://localhost:8080 |
| MinIO Console | http://localhost:9001 |
| MinIO S3 API | http://localhost:9000 |

Đăng nhập mặc định: `admin` / `admin123`

### 4. Chạy local (development)

**MinIO:**
```bash
docker-compose up minio -d
```

**Backend:**
```bash
cd backend
go run ./cmd/server/
```

**Frontend:**
```bash
cd frontend
npm install
npm run dev
# Mở http://localhost:3000
```

## API Endpoints

### Auth
```
POST /auth/token     { username, password } → { access_token, refresh_token }
POST /auth/refresh   { refresh_token }      → { access_token, refresh_token }
```

### Folders (yêu cầu Bearer token)
```
GET    /api/folders          ?type=Images&path=/
POST   /api/folder           { type, path, name }
DELETE /api/folder           { type, path }
PATCH  /api/folder/rename    { type, path, newName }
```

### Files (yêu cầu Bearer token)
```
GET    /api/files            ?type=Images&path=/
POST   /api/upload           multipart: { file, type, path }
POST   /api/upload/ck        multipart: { upload, type, path }  ← CKEditor format
DELETE /api/files            { type, path, files: [] }
PATCH  /api/file/rename      { type, path, name, newName }
POST   /api/files/move       { files: [], destination: {} }
POST   /api/files/copy       { files: [], destination: {} }
GET    /api/file/download    ?type=Images&path=/&name=x.jpg
```

### Media
```
GET    /api/thumbnail        ?type=Images&path=/&name=x.jpg&w=150&h=150
GET    /api/preview          ?type=Images&path=/&name=x.jpg&w=800
GET    /health
```

## Cấu trúc MinIO

```
Bucket: media
├── images/           ← Resource type "Images" (public read)
│   ├── photo.jpg
│   └── album/
│       └── sunset.jpg
├── files/            ← Resource type "Files" (private)
│   └── report.pdf
├── videos/           ← Resource type "Videos" (public read)
│   └── demo.mp4
└── _thumbs/          ← Thumbnail cache (tự động)
    └── images/photo.jpg_150x150.jpg
```

## Tích hợp CKEditor 5

```typescript
import ClassicEditor from '@ckeditor/ckeditor5-build-classic';
import { CKFinderUploadAdapterPlugin } from './src/ckeditor/uploadAdapter';

ClassicEditor.create(document.querySelector('#editor'), {
  extraPlugins: [CKFinderUploadAdapterPlugin],
  ckfinderBackend: {
    apiUrl: 'http://localhost:8080',
    token: localStorage.getItem('access_token') || '',
    resourceType: 'Images',
    uploadPath: '/',
  },
});
```

## Cấu hình .env

| Biến | Mặc định | Mô tả |
|---|---|---|
| `MINIO_ENDPOINT` | `localhost:9000` | MinIO endpoint |
| `MINIO_ACCESS_KEY` | `minioadmin` | Access key |
| `MINIO_SECRET_KEY` | `minioadmin` | Secret key |
| `MINIO_BUCKET` | `media` | Tên bucket |
| `MINIO_PUBLIC_BASE_URL` | `http://localhost:9000` | URL public cho file |
| `JWT_SECRET` | — | Secret key cho JWT (**bắt buộc đổi**) |
| `JWT_VALIDATION_MODE` | `local` | Chế độ xác thực JWT (`local` hoặc `remote` qua API bên ngoài) |
| `API_URL` | — | URL API xác thực bên ngoài (bắt buộc khi `JWT_VALIDATION_MODE=remote`) |
| `ADMIN_USERNAME` | `admin` | Username đăng nhập |
| `ADMIN_PASSWORD` | `admin123` | Password đăng nhập |
| `MAX_UPLOAD_SIZE_MB` | `50` | Kích thước tối đa upload |
| `FRONTEND_URL` | `http://localhost:3000` | URL frontend (CORS) |

## Production

```bash
# Build images
docker-compose -f docker-compose.yml build

# Start tất cả dịch vụ
docker-compose up -d

# Xem logs
docker-compose logs -f backend
```

**Lưu ý bảo mật khi deploy production:**
1. Đổi `JWT_SECRET` thành chuỗi ngẫu nhiên dài ≥ 32 ký tự
2. Đổi `ADMIN_PASSWORD` hoặc kết nối với database thực
3. Bật `MINIO_USE_SSL=true` với TLS certificate
4. Cấu hình Nginx reverse proxy với HTTPS

## License

MIT
