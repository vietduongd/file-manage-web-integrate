package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	// Server
	ServerPort string
	ServerEnv  string

	// MinIO / S3
	MinioEndpoint      string
	MinioAccessKey     string
	MinioSecretKey     string
	MinioBucket        string
	MinioUseSSL        bool
	MinioPublicBaseURL string

	// JWT
	JWTSecret              string
	JWTExpiryHours         int
	JWTRefreshExpiryHours  int

	// Auth
	AdminUsername string
	AdminPassword string

	// Upload
	MaxUploadSizeMB int64

	// Allowed extensions per resource type
	AllowedImageExts []string
	AllowedFileExts  []string
	AllowedVideoExts []string

	// CORS
	FrontendURL string
}

var cfg *Config

// Load reads configuration from environment variables (with .env fallback)
func Load() *Config {
	// Try to load .env file (ignore error if not found)
	_ = godotenv.Load()

	cfg = &Config{
		ServerPort: getEnv("SERVER_PORT", "8080"),
		ServerEnv:  getEnv("SERVER_ENV", "development"),

		MinioEndpoint:      getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey:     getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey:     getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinioBucket:        getEnv("MINIO_BUCKET", "media"),
		MinioUseSSL:        getEnvBool("MINIO_USE_SSL", false),
		MinioPublicBaseURL: getEnv("MINIO_PUBLIC_BASE_URL", "http://localhost:9000"),

		JWTSecret:             getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpiryHours:        getEnvInt("JWT_EXPIRY_HOURS", 24),
		JWTRefreshExpiryHours: getEnvInt("JWT_REFRESH_EXPIRY_HOURS", 168),

		AdminUsername: getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "admin123"),

		MaxUploadSizeMB: int64(getEnvInt("MAX_UPLOAD_SIZE_MB", 50)),

		AllowedImageExts: splitEnv("ALLOWED_IMAGE_EXTS", "jpg,jpeg,png,gif,webp,svg,bmp,ico"),
		AllowedFileExts:  splitEnv("ALLOWED_FILE_EXTS", "pdf,doc,docx,xls,xlsx,ppt,pptx,txt,csv,zip,rar,7z"),
		AllowedVideoExts: splitEnv("ALLOWED_VIDEO_EXTS", "mp4,webm,ogg,avi,mov,mkv"),

		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
	}

	return cfg
}

// Get returns the loaded configuration (panics if not loaded)
func Get() *Config {
	if cfg == nil {
		panic("config not loaded, call config.Load() first")
	}
	return cfg
}

// ResourceTypes returns the list of resource type names
func (c *Config) ResourceTypes() []ResourceTypeConfig {
	return []ResourceTypeConfig{
		{
			Name:              "Images",
			Prefix:            "images",
			AllowedExtensions: c.AllowedImageExts,
			MaxSizeMB:         c.MaxUploadSizeMB,
			PublicRead:        true,
		},
		{
			Name:              "Files",
			Prefix:            "files",
			AllowedExtensions: c.AllowedFileExts,
			MaxSizeMB:         c.MaxUploadSizeMB,
			PublicRead:        false,
		},
		{
			Name:              "Videos",
			Prefix:            "videos",
			AllowedExtensions: c.AllowedVideoExts,
			MaxSizeMB:         c.MaxUploadSizeMB,
			PublicRead:        true,
		},
	}
}

// ResourceTypeConfig holds config for a single resource type
type ResourceTypeConfig struct {
	Name              string
	Prefix            string
	AllowedExtensions []string
	MaxSizeMB         int64
	PublicRead        bool
}

// GetResourceType finds a resource type by name (case-insensitive)
func (c *Config) GetResourceType(name string) (*ResourceTypeConfig, error) {
	for _, rt := range c.ResourceTypes() {
		if strings.EqualFold(rt.Name, name) {
			return &rt, nil
		}
	}
	return nil, fmt.Errorf("resource type %q not found", name)
}

// IsExtensionAllowed checks if the file extension is allowed for a resource type
func (rt *ResourceTypeConfig) IsExtensionAllowed(ext string) bool {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	for _, allowed := range rt.AllowedExtensions {
		if strings.EqualFold(allowed, ext) {
			return true
		}
	}
	return false
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		i, err := strconv.Atoi(v)
		if err == nil {
			return i
		}
	}
	return fallback
}

func splitEnv(key, fallback string) []string {
	v := getEnv(key, fallback)
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}
