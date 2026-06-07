package handlers

import (
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ckfindercompatible/backend/internal/config"
	minioclient "github.com/ckfindercompatible/backend/internal/minio"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// StatsHandler handles storage metrics calculation
type StatsHandler struct {
	mc     *minioclient.Client
	cfg    *config.Config
	logger *zap.Logger
}

// NewStatsHandler creates a new StatsHandler
func NewStatsHandler(mc *minioclient.Client, cfg *config.Config, logger *zap.Logger) *StatsHandler {
	return &StatsHandler{mc: mc, cfg: cfg, logger: logger}
}

// ResourceStats represents statistics for a resource type
type ResourceStats struct {
	Count int64 `json:"count"`
	Size  int64 `json:"size"` // bytes
}

// StatsResponse represents the JSON response for GET /api/stats
type StatsResponse struct {
	TotalCount int64                    `json:"totalCount"`
	TotalSize  int64                    `json:"totalSize"`
	Breakdown  map[string]ResourceStats `json:"breakdown"`
}

// GetStats returns storage statistics recursively
func (h *StatsHandler) GetStats(c *gin.Context) {
	ctx := c.Request.Context()
	breakdown := make(map[string]ResourceStats)
	var totalCount int64
	var totalSize int64

	rts := h.cfg.ResourceTypes()
	for _, rt := range rts {
		prefix := rt.Prefix + "/"
		var count int64
		var size int64

		paginator := s3.NewListObjectsV2Paginator(h.mc.S3, &s3.ListObjectsV2Input{
			Bucket: aws.String(h.mc.Bucket),
			Prefix: aws.String(prefix),
		})

		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				h.logger.Error("Stats paginator failed", zap.String("prefix", prefix), zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to calculate statistics"})
				return
			}

			for _, obj := range page.Contents {
				if obj.Key == nil {
					continue
				}
				// Skip folder placeholders (.keep)
				if strings.HasSuffix(*obj.Key, ".keep") {
					continue
				}
				count++
				size += aws.ToInt64(obj.Size)
			}
		}

		breakdown[rt.Name] = ResourceStats{
			Count: count,
			Size:  size,
		}
		totalCount += count
		totalSize += size
	}

	c.JSON(http.StatusOK, StatsResponse{
		TotalCount: totalCount,
		TotalSize:  totalSize,
		Breakdown:  breakdown,
	})
}
