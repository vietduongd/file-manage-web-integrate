package models

// ---- Auth ----

// TokenRequest is the request body for POST /auth/token
type TokenRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// TokenResponse is the response body for POST /auth/token
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds
	TokenType    string `json:"token_type"`
}

// RefreshRequest is the request body for POST /auth/refresh
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// ---- Config ----

// ResourceTypeInfo is the public config for a resource type
type ResourceTypeInfo struct {
	Name              string   `json:"name"`
	AllowedExtensions []string `json:"allowedExtensions"`
	MaxSizeMB         int64    `json:"maxSizeMb"`
	PublicRead        bool     `json:"publicRead"`
	URL               string   `json:"url"`
}

// ConfigResponse is the response for GET /api/config
type ConfigResponse struct {
	ResourceTypes []ResourceTypeInfo `json:"resourceTypes"`
	MaxUploadMB   int64              `json:"maxUploadMb"`
}

// ---- Folders ----

// FolderInfo represents a virtual folder
type FolderInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	HasChildren bool   `json:"hasChildren"`
	ACL         int    `json:"acl"`
}

// FoldersResponse is the response for GET /api/folders
type FoldersResponse struct {
	ResourceType  string        `json:"resourceType"`
	CurrentFolder CurrentFolder `json:"currentFolder"`
	Folders       []FolderInfo  `json:"folders"`
}

// CreateFolderRequest is the request body for POST /api/folder
type CreateFolderRequest struct {
	Type string `json:"type" binding:"required"`
	Path string `json:"path" binding:"required"`
	Name string `json:"name" binding:"required"`
}

// DeleteFolderRequest is the request body for DELETE /api/folder
type DeleteFolderRequest struct {
	Type string `json:"type" binding:"required"`
	Path string `json:"path" binding:"required"`
}

// RenameFolderRequest is the request body for PATCH /api/folder/rename
type RenameFolderRequest struct {
	Type    string `json:"type" binding:"required"`
	Path    string `json:"path" binding:"required"`
	NewName string `json:"newName" binding:"required"`
}

// ---- Files ----

// FileInfo represents a file stored in MinIO
type FileInfo struct {
	Name  string `json:"name"`
	Date  string `json:"date"`  // format: YYYYMMDDHHmm
	Size  int64  `json:"size"`  // bytes
	URL   string `json:"url"`   // public URL
	Thumb string `json:"thumb"` // thumbnail URL (empty for non-images)
}

// CurrentFolder holds info about the current browsed folder
type CurrentFolder struct {
	Path string `json:"path"`
	URL  string `json:"url"`
	ACL  int    `json:"acl"`
}

// FilesResponse is the response for GET /api/files
type FilesResponse struct {
	ResourceType  string        `json:"resourceType"`
	CurrentFolder CurrentFolder `json:"currentFolder"`
	Files         []FileInfo    `json:"files"`
}

// UploadResponse is the response for POST /api/upload (standard)
type UploadResponse struct {
	FileName string `json:"fileName"`
	Uploaded int    `json:"uploaded"` // 1 = success
	URL      string `json:"url"`
}

// CKEditorUploadResponse is the response format expected by CKEditor 5
type CKEditorUploadResponse struct {
	Uploaded int    `json:"uploaded"`
	FileName string `json:"fileName"`
	URL      string `json:"url"`
	Error    *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// DeleteFilesRequest is the request body for DELETE /api/files
type DeleteFilesRequest struct {
	Type  string   `json:"type" binding:"required"`
	Path  string   `json:"path" binding:"required"`
	Files []string `json:"files" binding:"required"`
}

// RenameFileRequest is the request body for PATCH /api/file/rename
type RenameFileRequest struct {
	Type    string `json:"type" binding:"required"`
	Path    string `json:"path" binding:"required"`
	Name    string `json:"name" binding:"required"`
	NewName string `json:"newName" binding:"required"`
}

// FileRef is a reference to a specific file
type FileRef struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Name string `json:"name"`
}

// MoveFilesRequest is the request body for POST /api/files/move
type MoveFilesRequest struct {
	Files       []FileRef `json:"files" binding:"required"`
	Destination FileRef   `json:"destination" binding:"required"`
}

// CopyFilesRequest is the request body for POST /api/files/copy
type CopyFilesRequest struct {
	Files       []FileRef `json:"files" binding:"required"`
	Destination FileRef   `json:"destination" binding:"required"`
}

// ---- Errors ----

// ErrorResponse is the standard error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail holds the error details
type ErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ---- ZIP Operations ----

// CompressFilesRequest is the request body for POST /api/files/compress
type CompressFilesRequest struct {
	Type    string   `json:"type" binding:"required"`
	Path    string   `json:"path" binding:"required"`
	Files   []string `json:"files" binding:"required"`
	ZipName string   `json:"zipName" binding:"required"`
}

// ExtractZipRequest is the request body for POST /api/files/extract
type ExtractZipRequest struct {
	Type     string `json:"type" binding:"required"`
	Path     string `json:"path" binding:"required"`
	FileName string `json:"fileName" binding:"required"`
}
