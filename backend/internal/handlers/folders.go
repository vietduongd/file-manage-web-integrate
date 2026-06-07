package handlers

import (
	"net/http"
	"strings"

	"github.com/ckfindercompatible/backend/internal/config"
	minioclient "github.com/ckfindercompatible/backend/internal/minio"
	"github.com/ckfindercompatible/backend/internal/models"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// FoldersHandler handles folder-related endpoints
type FoldersHandler struct {
	mc     *minioclient.Client
	cfg    *config.Config
	logger *zap.Logger
}

// NewFoldersHandler creates a new FoldersHandler
func NewFoldersHandler(mc *minioclient.Client, cfg *config.Config, logger *zap.Logger) *FoldersHandler {
	return &FoldersHandler{mc: mc, cfg: cfg, logger: logger}
}

// ListFolders handles GET /api/folders?type=Images&path=/
func (h *FoldersHandler) ListFolders(c *gin.Context) {
	resourceTypeName := c.Query("type")
	folderPath := c.Query("path")

	rt, err := h.cfg.GetResourceType(resourceTypeName)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Unknown resource type: "+resourceTypeName))
		return
	}

	prefix := minioclient.FolderPrefix(rt.Prefix, folderPath)
	folderNames, err := h.mc.ListFolders(c.Request.Context(), prefix)
	if err != nil {
		h.logger.Error("ListFolders failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Failed to list folders"))
		return
	}

	folders := make([]models.FolderInfo, 0, len(folderNames))
	for _, name := range folderNames {
		childPrefix := prefix + name
		hasChildren, _ := h.mc.HasSubfolders(c.Request.Context(), childPrefix)
		childPath := cleanPath(folderPath) + name + "/"
		folders = append(folders, models.FolderInfo{
			Name:        name,
			Path:        childPath,
			HasChildren: hasChildren,
			ACL:         255,
		})
	}

	currentURL := h.cfg.MinioPublicBaseURL + "/" + h.cfg.MinioBucket + "/" + prefix
	c.JSON(http.StatusOK, models.FoldersResponse{
		ResourceType: rt.Name,
		CurrentFolder: models.CurrentFolder{
			Path: "/" + strings.TrimPrefix(folderPath, "/"),
			URL:  currentURL,
			ACL:  255,
		},
		Folders: folders,
	})
}

// CreateFolder handles POST /api/folder
func (h *FoldersHandler) CreateFolder(c *gin.Context) {
	var req models.CreateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, err.Error()))
		return
	}

	rt, err := h.cfg.GetResourceType(req.Type)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Unknown resource type"))
		return
	}

	// Sanitize folder name
	if !isValidName(req.Name) {
		c.JSON(http.StatusBadRequest, errorResp(400, "Invalid folder name"))
		return
	}

	prefix := minioclient.FolderPrefix(rt.Prefix, req.Path) + req.Name
	if err := h.mc.CreateFolder(c.Request.Context(), prefix); err != nil {
		h.logger.Error("CreateFolder failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Failed to create folder"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"created": true, "name": req.Name})
}

// DeleteFolder handles DELETE /api/folder
func (h *FoldersHandler) DeleteFolder(c *gin.Context) {
	var req models.DeleteFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, err.Error()))
		return
	}

	rt, err := h.cfg.GetResourceType(req.Type)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Unknown resource type"))
		return
	}

	prefix := minioclient.FolderPrefix(rt.Prefix, req.Path)
	if err := h.mc.DeleteFolder(c.Request.Context(), prefix); err != nil {
		h.logger.Error("DeleteFolder failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Failed to delete folder"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// RenameFolder handles PATCH /api/folder/rename
func (h *FoldersHandler) RenameFolder(c *gin.Context) {
	var req models.RenameFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, err.Error()))
		return
	}

	rt, err := h.cfg.GetResourceType(req.Type)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResp(400, "Unknown resource type"))
		return
	}

	if !isValidName(req.NewName) {
		c.JSON(http.StatusBadRequest, errorResp(400, "Invalid folder name"))
		return
	}

	// Build old and new prefixes
	// If path is "/photos/vacation/", old = "images/photos/vacation/", new = "images/photos/newname/"
	pathParts := strings.Split(strings.Trim(req.Path, "/"), "/")
	parentPath := ""
	if len(pathParts) > 1 {
		parentPath = strings.Join(pathParts[:len(pathParts)-1], "/")
	}

	oldPrefix := minioclient.FolderPrefix(rt.Prefix, req.Path)
	newPrefix := minioclient.FolderPrefix(rt.Prefix, parentPath) + req.NewName

	if err := h.mc.RenameFolder(c.Request.Context(), oldPrefix, newPrefix); err != nil {
		h.logger.Error("RenameFolder failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Failed to rename folder"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"renamed": true, "newName": req.NewName})
}

// ---- helpers ----

func errorResp(code int, message string) models.ErrorResponse {
	return models.ErrorResponse{
		Error: models.ErrorDetail{Code: code, Message: message},
	}
}

func cleanPath(p string) string {
	p = strings.Trim(p, "/")
	if p == "" {
		return "/"
	}
	return "/" + p + "/"
}

func isValidName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, ch := range invalid {
		if strings.Contains(name, ch) {
			return false
		}
	}
	return true
}
