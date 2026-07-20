package handlers

import (
	"net/http"
	"strconv"

	"github.com/ckfindercompatible/backend/internal/config"
	minioclient "github.com/ckfindercompatible/backend/internal/minio"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MediaHandler handles thumbnail and preview endpoints
type MediaHandler struct {
	mc     *minioclient.Client
	cfg    *config.Config
	logger *zap.Logger
}

// NewMediaHandler creates a new MediaHandler
func NewMediaHandler(mc *minioclient.Client, cfg *config.Config, logger *zap.Logger) *MediaHandler {
	return &MediaHandler{mc: mc, cfg: cfg, logger: logger}
}

// Thumbnail handles GET /api/thumbnail?type=Images&path=/&name=x.jpg&w=150&h=150
func (h *MediaHandler) Thumbnail(c *gin.Context) {
	h.serveResized(c, 150, 150, true)
}

// Preview handles GET /api/preview?type=Images&path=/&name=x.jpg&w=800
func (h *MediaHandler) Preview(c *gin.Context) {
	h.serveResized(c, 800, 0, false)
}

func (h *MediaHandler) serveResized(c *gin.Context, defaultW, defaultH int, fit bool) {
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

	w := parseIntDefault(c.Query("w"), defaultW)
	h2 := parseIntDefault(c.Query("h"), defaultH)

	key := minioclient.BuildKey(rt.Prefix, folderPath, fileName)

	if !minioclient.IsImage(fileName) {
		// For non-images, redirect to the direct URL
		c.Redirect(http.StatusFound, h.mc.PublicURL(key))
		return
	}

	opts := minioclient.ThumbOptions{Width: w, Height: h2, Fit: fit}
	thumbBytes, err := h.mc.GetOrCreateThumbnail(c.Request.Context(), key, opts)
	if err != nil {
		if minioclient.IsOriginalNotFound(err) {
			h.logger.Warn("thumbnail original not found", zap.String("key", key), zap.Error(err))
			c.JSON(http.StatusNotFound, errorResp(404, "Original file not found"))
			return
		}
		h.logger.Error("GetOrCreateThumbnail failed", zap.String("key", key), zap.Error(err))
		c.JSON(http.StatusInternalServerError, errorResp(500, "Failed to generate thumbnail"))
		return
	}

	c.Header("Cache-Control", "public, max-age=86400")
	c.Data(http.StatusOK, "image/jpeg", thumbBytes)
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return def
	}
	return v
}
