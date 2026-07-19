package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ckfindercompatible/backend/internal/auth"
	"github.com/ckfindercompatible/backend/internal/config"
	"github.com/ckfindercompatible/backend/internal/handlers"
	minioclient "github.com/ckfindercompatible/backend/internal/minio"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// ── Config ────────────────────────────────────────────────────────────
	cfg := config.Load()

	// ── Logger ────────────────────────────────────────────────────────────
	logger := buildLogger(cfg.ServerEnv)
	defer logger.Sync() //nolint:errcheck

	logger.Info("starting server",
		zap.String("env", cfg.ServerEnv),
		zap.String("port", cfg.ServerPort),
		zap.String("minio_endpoint", cfg.MinioEndpoint),
		zap.String("bucket", cfg.MinioBucket),
	)

	// ── MinIO client ──────────────────────────────────────────────────────
	mc, err := minioclient.New(cfg, logger)
	if err != nil {
		logger.Fatal("failed to create MinIO client", zap.Error(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := mc.EnsureBucket(ctx); err != nil {
		logger.Fatal("failed to ensure MinIO bucket", zap.Error(err))
	}

	// ── Gin router ────────────────────────────────────────────────────────
	if cfg.ServerEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger(logger))

	// CORS
	r.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			if cfg.ServerEnv != "production" {
				// Trong development: cho phép mọi localhost/127.0.0.1 ở bất kỳ port nào
				return strings.HasPrefix(origin, "http://localhost") ||
					strings.HasPrefix(origin, "http://127.0.0.1") ||
					strings.HasPrefix(origin, "https://localhost")
			}
			// Trong production: chỉ cho phép FRONTEND_URL
			return origin == cfg.FrontendURL
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// ── Handlers ──────────────────────────────────────────────────────────
	authHandler := handlers.NewAuthHandler(cfg)
	configHandler := handlers.NewConfigHandler(cfg)
	foldersHandler := handlers.NewFoldersHandler(mc, cfg, logger)
	filesHandler := handlers.NewFilesHandler(mc, cfg, logger)
	mediaHandler := handlers.NewMediaHandler(mc, cfg, logger)
	statsHandler := handlers.NewStatsHandler(mc, cfg, logger)

	// ── Routes ────────────────────────────────────────────────────────────

	// Health check (no auth)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().UTC()})
	})

	// Auth routes (no middleware)
	authGroup := r.Group("/auth")
	{
		authGroup.POST("/embed/login", authHandler.EmbedLogin)
		authGroup.POST("/token", authHandler.Token)
		authGroup.POST("/external-token", authHandler.ExternalToken)
		authGroup.POST("/refresh", authHandler.Refresh)
	}

	// Protected API routes
	api := r.Group("/api")
	api.Use(auth.Middleware(cfg.JWTSecret))
	{
		// Config
		api.GET("/config", configHandler.GetConfig)

		// Folders
		api.GET("/folders", foldersHandler.ListFolders)
		api.POST("/folder", foldersHandler.CreateFolder)
		api.DELETE("/folder", foldersHandler.DeleteFolder)
		api.PATCH("/folder/rename", foldersHandler.RenameFolder)

		// Files
		api.GET("/files", filesHandler.ListFiles)
		api.DELETE("/files", filesHandler.DeleteFiles)
		api.PATCH("/file/rename", filesHandler.RenameFile)
		api.POST("/files/move", filesHandler.MoveFiles)
		api.POST("/files/copy", filesHandler.CopyFiles)
		api.GET("/file/download", filesHandler.DownloadFile)
		api.POST("/files/compress", filesHandler.CompressFiles)
		api.POST("/files/extract", filesHandler.ExtractZip)

		// Upload
		api.POST("/upload", filesHandler.UploadFile)
		api.POST("/upload/ck", filesHandler.CKEditorUpload)

		// Media (thumbnail / preview)
		api.GET("/thumbnail", mediaHandler.Thumbnail)
		api.GET("/preview", mediaHandler.Preview)

		// Stats
		api.GET("/stats", statsHandler.GetStats)
	}

	// ── HTTP Server with graceful shutdown ────────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.ServerPort),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}
	logger.Info("server exited")
}

// buildLogger creates a zap logger appropriate for the environment
func buildLogger(env string) *zap.Logger {
	var zapCfg zap.Config
	if env == "production" {
		zapCfg = zap.NewProductionConfig()
	} else {
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	logger, _ := zapCfg.Build()
	return logger
}

// requestLogger returns a Gin middleware that logs HTTP requests using zap
func requestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		logger.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.ClientIP()),
		)
	}
}
