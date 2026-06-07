package handlers

import (
	"net/http"

	"github.com/ckfindercompatible/backend/internal/config"
	"github.com/ckfindercompatible/backend/internal/models"
	"github.com/gin-gonic/gin"
)

// ConfigHandler handles GET /api/config
type ConfigHandler struct {
	cfg *config.Config
}

// NewConfigHandler creates a new ConfigHandler
func NewConfigHandler(cfg *config.Config) *ConfigHandler {
	return &ConfigHandler{cfg: cfg}
}

// GetConfig returns the file manager configuration
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	rts := h.cfg.ResourceTypes()
	result := make([]models.ResourceTypeInfo, len(rts))

	for i, rt := range rts {
		baseURL := h.cfg.MinioPublicBaseURL + "/" + h.cfg.MinioBucket + "/" + rt.Prefix + "/"
		result[i] = models.ResourceTypeInfo{
			Name:              rt.Name,
			AllowedExtensions: rt.AllowedExtensions,
			MaxSizeMB:         rt.MaxSizeMB,
			PublicRead:        rt.PublicRead,
			URL:               baseURL,
		}
	}

	c.JSON(http.StatusOK, models.ConfigResponse{
		ResourceTypes: result,
		MaxUploadMB:   h.cfg.MaxUploadSizeMB,
	})
}
