package handlers

import (
	"archive/zip"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ckfindercompatible/backend/internal/config"
	minioclient "github.com/ckfindercompatible/backend/internal/minio"
	"github.com/ckfindercompatible/backend/internal/models"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	maxExtractedZipEntries = 500
	maxExtractedZipBytes   = int64(200 * 1024 * 1024)
)

// FilesHandler handles file-related endpoints
type FilesHandler struct {
	mc     *minioclient.Client
	cfg    *config.Config
	logger *zap.Logger
}

// NewFilesHandler creates a new FilesHandler
func NewFilesHandler(mc *minioclient.Client, cfg *config.Config, logger *zap.Logger) *FilesHandler {
	return &FilesHandler{mc: mc, cfg: cfg, logger: logger}
}

// ListFiles handles GET /api/files?type=Images&path=/
func (h *FilesHandler) ListFiles(c *gin.Context) {
	resourceTypeName := c.Query("type")
	folderPath, err := minioclient.NormalizeFolderPath(c.Query("path"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Invalid folder path"))
		return
	}

	rt, err := h.cfg.GetResourceType(resourceTypeName)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Unknown resource type: "+resourceTypeName))
		return
	}

	prefix := minioclient.FolderPrefix(rt.Prefix, folderPath)
	objects, err := h.mc.ListFiles(c.Request.Context(), prefix)
	if err != nil {
		h.logger.Error("ListFiles failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Failed to list files"))
		return
	}

	files := make([]models.FileInfo, 0, len(objects))
	for _, obj := range objects {
		fileURL := h.mc.PublicURL(obj.Key)
		thumbURL := ""
		if minioclient.IsImage(obj.Name) {
			thumbURL = fmt.Sprintf("/api/thumbnail?type=%s&path=%s&name=%s",
				resourceTypeName, folderPath, obj.Name)
		}
		files = append(files, models.FileInfo{
			Name:  obj.Name,
			Date:  obj.LastModified.UTC().Format("200601021504"),
			Size:  obj.Size,
			URL:   fileURL,
			Thumb: thumbURL,
		})
	}

	currentURL := h.cfg.MinioPublicBaseURL + "/" + h.cfg.MinioBucket + "/" + prefix
	c.JSON(http.StatusOK, models.FilesResponse{
		ResourceType: rt.Name,
		CurrentFolder: models.CurrentFolder{
			Path: "/" + strings.TrimPrefix(folderPath, "/"),
			URL:  currentURL,
			ACL:  255,
		},
		Files: files,
	})
}

// DeleteFiles handles DELETE /api/files
func (h *FilesHandler) DeleteFiles(c *gin.Context) {
	var req models.DeleteFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, err.Error()))
		return
	}

	folderPath, err := minioclient.NormalizeFolderPath(req.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Invalid folder path"))
		return
	}

	rt, err := h.cfg.GetResourceType(req.Type)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Unknown resource type"))
		return
	}

	keys := make([]string, 0, len(req.Files))
	for _, name := range req.Files {
		fileName, err := minioclient.NormalizeObjectName(name)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResp(400, "Invalid file name"))
			return
		}
		key := minioclient.BuildKey(rt.Prefix, folderPath, fileName)
		keys = append(keys, key)
	}

	if err := h.mc.DeleteFiles(c.Request.Context(), keys); err != nil {
		h.logger.Error("DeleteFiles failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Failed to delete files"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": len(keys)})
}

// RenameFile handles PATCH /api/file/rename
func (h *FilesHandler) RenameFile(c *gin.Context) {
	var req models.RenameFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, err.Error()))
		return
	}

	folderPath, err := minioclient.NormalizeFolderPath(req.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Invalid folder path"))
		return
	}
	fileName, err := minioclient.NormalizeObjectName(req.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Invalid file name"))
		return
	}
	newName, err := minioclient.NormalizeObjectName(req.NewName)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Invalid new file name"))
		return
	}

	rt, err := h.cfg.GetResourceType(req.Type)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Unknown resource type"))
		return
	}

	// Preserve extension
	oldExt := strings.ToLower(filepath.Ext(fileName))
	newExt := strings.ToLower(filepath.Ext(newName))
	if newExt == "" {
		newName = newName + oldExt
	}

	if !rt.IsExtensionAllowed(newExt) {
		c.JSON(http.StatusBadRequest, errorResp(400, "Extension not allowed"))
		return
	}

	srcKey := minioclient.BuildKey(rt.Prefix, folderPath, fileName)
	dstKey := minioclient.BuildKey(rt.Prefix, folderPath, newName)

	if err := h.mc.RenameFile(c.Request.Context(), srcKey, dstKey); err != nil {
		h.logger.Error("RenameFile failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Failed to rename file"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"renamed": true, "newName": newName, "url": h.mc.PublicURL(dstKey)})
}

// MoveFiles handles POST /api/files/move
func (h *FilesHandler) MoveFiles(c *gin.Context) {
	var req models.MoveFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, err.Error()))
		return
	}

	moved := 0
	for _, f := range req.Files {
		srcRT, err := h.cfg.GetResourceType(f.Type)
		if err != nil {
			continue
		}
		dstRT, err := h.cfg.GetResourceType(req.Destination.Type)
		if err != nil {
			continue
		}
		srcPath, err := minioclient.NormalizeFolderPath(f.Path)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResp(400, "Invalid source folder path"))
			return
		}
		dstPath, err := minioclient.NormalizeFolderPath(req.Destination.Path)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResp(400, "Invalid destination folder path"))
			return
		}
		fileName, err := minioclient.NormalizeObjectName(f.Name)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResp(400, "Invalid file name"))
			return
		}

		srcKey := minioclient.BuildKey(srcRT.Prefix, srcPath, fileName)
		dstKey := minioclient.BuildKey(dstRT.Prefix, dstPath, fileName)

		if err := h.mc.RenameFile(c.Request.Context(), srcKey, dstKey); err != nil {
			h.logger.Warn("MoveFile failed", zap.String("src", srcKey), zap.Error(err))
			continue
		}
		moved++
	}

	c.JSON(http.StatusOK, gin.H{"moved": moved})
}

// CopyFiles handles POST /api/files/copy
func (h *FilesHandler) CopyFiles(c *gin.Context) {
	var req models.CopyFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, err.Error()))
		return
	}

	copied := 0
	for _, f := range req.Files {
		srcRT, err := h.cfg.GetResourceType(f.Type)
		if err != nil {
			continue
		}
		dstRT, err := h.cfg.GetResourceType(req.Destination.Type)
		if err != nil {
			continue
		}
		srcPath, err := minioclient.NormalizeFolderPath(f.Path)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResp(400, "Invalid source folder path"))
			return
		}
		dstPath, err := minioclient.NormalizeFolderPath(req.Destination.Path)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResp(400, "Invalid destination folder path"))
			return
		}
		fileName, err := minioclient.NormalizeObjectName(f.Name)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResp(400, "Invalid file name"))
			return
		}

		srcKey := minioclient.BuildKey(srcRT.Prefix, srcPath, fileName)
		dstKey := minioclient.BuildKey(dstRT.Prefix, dstPath, fileName)

		if err := h.mc.CopyFile(c.Request.Context(), srcKey, dstKey); err != nil {
			h.logger.Warn("CopyFile failed", zap.String("src", srcKey), zap.Error(err))
			continue
		}
		copied++
	}

	c.JSON(http.StatusOK, gin.H{"copied": copied})
}

// DownloadFile handles GET /api/file/download
func (h *FilesHandler) DownloadFile(c *gin.Context) {
	resourceTypeName := c.Query("type")
	folderPath, err := minioclient.NormalizeFolderPath(c.Query("path"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Invalid folder path"))
		return
	}
	fileName, err := minioclient.NormalizeObjectName(c.Query("name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Invalid file name"))
		return
	}

	if fileName == "" {
		c.JSON(http.StatusBadRequest, errorResp(400, "Missing file name"))
		return
	}

	rt, err := h.cfg.GetResourceType(resourceTypeName)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Unknown resource type"))
		return
	}

	key := minioclient.BuildKey(rt.Prefix, folderPath, fileName)
	url, err := h.mc.PresignGetObject(c.Request.Context(), key, 15*time.Minute)
	if err != nil {
		h.logger.Error("PresignGetObject failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Failed to generate download URL"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}

// UploadFile handles POST /api/upload (multipart form)
func (h *FilesHandler) UploadFile(c *gin.Context) {
	resourceTypeName := c.PostForm("type")
	folderPath, err := minioclient.NormalizeFolderPath(c.PostForm("path"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Invalid folder path"))
		return
	}

	rt, err := h.cfg.GetResourceType(resourceTypeName)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Unknown resource type"))
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "No file provided"))
		return
	}
	defer file.Close()
	fileName, err := minioclient.NormalizeObjectName(header.Filename)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Invalid file name"))
		return
	}

	// Validate size
	maxBytes := rt.MaxSizeMB * 1024 * 1024
	if header.Size > maxBytes {
		c.JSON(http.StatusBadRequest, errorResp(400, fmt.Sprintf("File too large (max %d MB)", rt.MaxSizeMB)))
		return
	}

	// Validate extension
	ext := strings.ToLower(filepath.Ext(fileName))
	if !rt.IsExtensionAllowed(ext) {
		c.JSON(http.StatusBadRequest, errorResp(400, fmt.Sprintf("Extension %q not allowed for %s", ext, rt.Name)))
		return
	}

	// Detect MIME type
	mime, err := mimetype.DetectReader(file)
	if err == nil {
		_, _ = file.Seek(0, 0)
	}
	contentType := header.Header.Get("Content-Type")
	if mime != nil {
		contentType = mime.String()
	}

	key := minioclient.BuildKey(rt.Prefix, folderPath, fileName)
	if err := h.mc.PutFile(c.Request.Context(), key, contentType, file, header.Size); err != nil {
		h.logger.Error("PutFile failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Failed to upload file"))
		return
	}

	fileURL := h.mc.PublicURL(key)
	c.JSON(http.StatusOK, models.UploadResponse{
		FileName: fileName,
		Uploaded: 1,
		URL:      fileURL,
	})
}

// CKEditorUpload handles POST /api/upload/ck (CKEditor 5 format)
func (h *FilesHandler) CKEditorUpload(c *gin.Context) {
	resourceTypeName := c.DefaultPostForm("type", "Images")
	folderPath, err := minioclient.NormalizeFolderPath(c.DefaultPostForm("path", "/"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.CKEditorUploadResponse{
			Uploaded: 0,
			Error: &struct {
				Message string `json:"message"`
			}{"Invalid folder path"},
		})
		return
	}

	rt, err := h.cfg.GetResourceType(resourceTypeName)
	if err != nil {
		rt, _ = h.cfg.GetResourceType("Images") // fallback to Images
	}

	file, header, err := c.Request.FormFile("upload") // CKEditor uses "upload" field
	if err != nil {
		c.JSON(http.StatusBadRequest, models.CKEditorUploadResponse{
			Uploaded: 0,
			Error: &struct {
				Message string `json:"message"`
			}{"No file provided"},
		})
		return
	}
	defer file.Close()
	fileName, err := minioclient.NormalizeObjectName(header.Filename)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.CKEditorUploadResponse{
			Uploaded: 0,
			Error: &struct {
				Message string `json:"message"`
			}{"Invalid file name"},
		})
		return
	}

	maxBytes := rt.MaxSizeMB * 1024 * 1024
	if header.Size > maxBytes {
		c.JSON(http.StatusBadRequest, models.CKEditorUploadResponse{
			Uploaded: 0,
			Error: &struct {
				Message string `json:"message"`
			}{fmt.Sprintf("File too large (max %d MB)", rt.MaxSizeMB)},
		})
		return
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	if !rt.IsExtensionAllowed(ext) {
		c.JSON(http.StatusBadRequest, models.CKEditorUploadResponse{
			Uploaded: 0,
			Error: &struct {
				Message string `json:"message"`
			}{fmt.Sprintf("Extension %q not allowed", ext)},
		})
		return
	}

	contentType := header.Header.Get("Content-Type")
	key := minioclient.BuildKey(rt.Prefix, folderPath, fileName)
	if err := h.mc.PutFile(c.Request.Context(), key, contentType, file, header.Size); err != nil {
		h.logger.Error("CKEditorUpload PutFile failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.CKEditorUploadResponse{
			Uploaded: 0,
			Error: &struct {
				Message string `json:"message"`
			}{"Upload failed"},
		})
		return
	}

	fileURL := h.mc.PublicURL(key)
	c.JSON(http.StatusOK, models.CKEditorUploadResponse{
		Uploaded: 1,
		FileName: fileName,
		URL:      fileURL,
	})
}

// CompressFiles handles POST /api/files/compress
func (h *FilesHandler) CompressFiles(c *gin.Context) {
	var req models.CompressFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, err.Error()))
		return
	}

	folderPath, err := minioclient.NormalizeFolderPath(req.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Invalid folder path"))
		return
	}
	zipName, err := minioclient.NormalizeObjectName(req.ZipName)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Invalid zip name"))
		return
	}

	rt, err := h.cfg.GetResourceType(req.Type)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Unknown resource type"))
		return
	}

	if !strings.HasSuffix(strings.ToLower(zipName), ".zip") {
		zipName = zipName + ".zip"
	}

	dstKey := minioclient.BuildKey(rt.Prefix, folderPath, zipName)

	// Create a temporary zip file
	tmpFile, err := os.CreateTemp("", "compress-*.zip")
	if err != nil {
		h.logger.Error("Failed to create temp zip file", zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Failed to create archive"))
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	zipWriter := zip.NewWriter(tmpFile)

	for _, filename := range req.Files {
		fileName, err := minioclient.NormalizeObjectName(filename)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResp(400, "Invalid file name"))
			return
		}
		srcKey := minioclient.BuildKey(rt.Prefix, folderPath, fileName)
		reader, _, err := h.mc.GetObject(c.Request.Context(), srcKey)
		if err != nil {
			h.logger.Warn("Failed to open source file for zipping", zap.String("key", srcKey), zap.Error(err))
			continue
		}

		writer, err := zipWriter.Create(fileName)
		if err != nil {
			reader.Close()
			h.logger.Warn("Failed to create zip header", zap.String("file", fileName), zap.Error(err))
			continue
		}

		_, err = io.Copy(writer, reader)
		reader.Close()
		if err != nil {
			h.logger.Warn("Failed to copy content to zip", zap.String("file", fileName), zap.Error(err))
			continue
		}
	}

	if err := zipWriter.Close(); err != nil {
		h.logger.Error("Failed to close zip writer", zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Failed to build archive"))
		return
	}

	// Seek back to start of temp file
	if _, err := tmpFile.Seek(0, 0); err != nil {
		h.logger.Error("Failed to seek temp file", zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Failed to seek archive"))
		return
	}

	stat, err := tmpFile.Stat()
	if err != nil {
		h.logger.Error("Failed to stat temp file", zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Failed to get archive size"))
		return
	}

	// Upload to MinIO
	if err := h.mc.PutFile(c.Request.Context(), dstKey, "application/zip", tmpFile, stat.Size()); err != nil {
		h.logger.Error("Failed to upload ZIP archive to MinIO", zap.String("key", dstKey), zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Failed to upload archive"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"compressed": true,
		"fileName":   zipName,
		"url":        h.mc.PublicURL(dstKey),
	})
}

// ExtractZip handles POST /api/files/extract
func (h *FilesHandler) ExtractZip(c *gin.Context) {
	var req models.ExtractZipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, err.Error()))
		return
	}

	folderPath, err := minioclient.NormalizeFolderPath(req.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Invalid folder path"))
		return
	}
	fileName, err := minioclient.NormalizeObjectName(req.FileName)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Invalid zip file name"))
		return
	}

	rt, err := h.cfg.GetResourceType(req.Type)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Unknown resource type"))
		return
	}

	srcKey := minioclient.BuildKey(rt.Prefix, folderPath, fileName)

	// Fetch ZIP from MinIO
	reader, _, err := h.mc.GetObject(c.Request.Context(), srcKey)
	if err != nil {
		h.logger.Error("Failed to download ZIP file", zap.String("key", srcKey), zap.Error(err))
		c.JSON(http.StatusNotFound, errorResp(404, "ZIP file not found"))
		return
	}
	defer reader.Close()

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "extract-*.zip")
	if err != nil {
		h.logger.Error("Failed to create temp file for extraction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Extraction setup failed"))
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, reader); err != nil {
		h.logger.Error("Failed to copy zip data to temp file", zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Failed to download archive"))
		return
	}

	// Open ZIP
	zipReader, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		h.logger.Error("Failed to open zip reader", zap.Error(err))
		c.JSON(http.StatusBadRequest, errorResp(400, "Invalid zip file format"))
		return
	}
	defer zipReader.Close()

	extractedCount := 0
	var extractedBytes int64
	for _, file := range zipReader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		normName, nextBytes, err := validateZipExtractionEntry(file.Name, file.UncompressedSize64, extractedCount, extractedBytes, rt)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResp(400, err.Error()))
			return
		}
		if normName == "" {
			continue
		}
		extractedBytes = nextBytes

		fReader, err := file.Open()
		if err != nil {
			h.logger.Warn("Failed to open file inside zip", zap.String("file", file.Name), zap.Error(err))
			continue
		}

		dstKey := minioclient.BuildKey(rt.Prefix, folderPath, normName)

		contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(normName)))
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		if err := h.mc.PutFile(c.Request.Context(), dstKey, contentType, fReader, int64(file.UncompressedSize64)); err != nil {
			h.logger.Warn("Failed to upload extracted file to MinIO", zap.String("key", dstKey), zap.Error(err))
			fReader.Close()
			continue
		}
		fReader.Close()
		extractedCount++
	}

	c.JSON(http.StatusOK, gin.H{
		"extracted": true,
		"count":     extractedCount,
	})
}

func validateZipExtractionEntry(name string, uncompressedSize uint64, extractedCount int, extractedBytes int64, rt *config.ResourceTypeConfig) (string, int64, error) {
	if extractedCount >= maxExtractedZipEntries {
		return "", extractedBytes, fmt.Errorf("ZIP archive contains too many files")
	}
	if uncompressedSize > uint64(maxExtractedZipBytes) {
		return "", extractedBytes, fmt.Errorf("ZIP archive is too large to extract")
	}
	nextBytes := extractedBytes + int64(uncompressedSize)
	if nextBytes > maxExtractedZipBytes {
		return "", extractedBytes, fmt.Errorf("ZIP archive is too large to extract")
	}

	normName, err := minioclient.NormalizeRelativeObjectPath(name)
	if err != nil {
		return "", extractedBytes, fmt.Errorf("ZIP archive contains invalid file paths")
	}
	ext := strings.ToLower(filepath.Ext(normName))
	if !rt.IsExtensionAllowed(ext) {
		return "", extractedBytes, nil
	}
	return normName, nextBytes, nil
}
