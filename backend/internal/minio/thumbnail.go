package minioclient

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/disintegration/imaging"
)

// ThumbOptions configures thumbnail generation
type ThumbOptions struct {
	Width  int
	Height int
	Fit    bool // true = crop to fit, false = fit within bounds
}

// ThumbnailKey returns the MinIO key for a cached thumbnail
func ThumbnailKey(originalKey string, opts ThumbOptions) string {
	return fmt.Sprintf("_thumbs/%s_%dx%d.jpg", originalKey, opts.Width, opts.Height)
}

// GetOrCreateThumbnail returns a thumbnail image as bytes.
// It checks the cache first, creates and caches if not found.
func (c *Client) GetOrCreateThumbnail(ctx context.Context, originalKey string, opts ThumbOptions) ([]byte, error) {
	thumbKey := ThumbnailKey(originalKey, opts)

	// Try cache
	body, _, err := c.GetObject(ctx, thumbKey)
	if err == nil {
		defer body.Close()
		return readAll(body)
	}

	// Generate thumbnail from original
	origBody, _, err := c.GetObject(ctx, originalKey)
	if err != nil {
		return nil, fmt.Errorf("original not found: %w", err)
	}
	defer origBody.Close()

	origBytes, err := readAll(origBody)
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(origBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	var resized *image.NRGBA
	if opts.Fit && opts.Width > 0 && opts.Height > 0 {
		resized = imaging.Fill(img, opts.Width, opts.Height, imaging.Center, imaging.Lanczos)
	} else if opts.Width > 0 && opts.Height > 0 {
		resized = imaging.Fit(img, opts.Width, opts.Height, imaging.Lanczos)
	} else if opts.Width > 0 {
		resized = imaging.Resize(img, opts.Width, 0, imaging.Lanczos)
	} else {
		resized = imaging.Resize(img, 0, opts.Height, imaging.Lanczos)
	}

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, resized, imaging.JPEG, imaging.JPEGQuality(85)); err != nil {
		return nil, fmt.Errorf("failed to encode thumbnail: %w", err)
	}
	thumbBytes := buf.Bytes()

	// Cache to MinIO (best-effort, don't fail if cache fails)
	_ = c.S3.PutObject // preload
	_, _ = c.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.Bucket),
		Key:           aws.String(thumbKey),
		Body:          bytes.NewReader(thumbBytes),
		ContentType:   aws.String("image/jpeg"),
		ContentLength: aws.Int64(int64(len(thumbBytes))),
	})

	return thumbBytes, nil
}

// IsImage returns true if the file extension suggests an image
func IsImage(filename string) bool {
	ext := strings.ToLower(strings.TrimPrefix(getExt(filename), "."))
	imageExts := map[string]bool{
		"jpg": true, "jpeg": true, "png": true,
		"gif": true, "webp": true, "bmp": true,
	}
	return imageExts[ext]
}

func getExt(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[i:]
		}
	}
	return ""
}

func readAll(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var buf bytes.Buffer
	tmp := make([]byte, 32*1024)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			break
		}
	}
	return buf.Bytes(), nil
}
