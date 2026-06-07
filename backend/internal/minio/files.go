package minioclient

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// FileObject represents a file stored in MinIO
type FileObject struct {
	Key          string
	Name         string
	Size         int64
	LastModified time.Time
	ContentType  string
}

// ListFiles returns all files (non-folder objects) under a prefix (one level deep).
func (c *Client) ListFiles(ctx context.Context, prefix string) ([]FileObject, error) {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	out, err := c.S3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(c.Bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})
	if err != nil {
		return nil, err
	}

	files := make([]FileObject, 0, len(out.Contents))
	for _, obj := range out.Contents {
		if obj.Key == nil {
			continue
		}
		name := strings.TrimPrefix(*obj.Key, prefix)
		// Skip folder placeholders and sub-path objects
		if name == "" || strings.Contains(name, "/") || name == ".keep" {
			continue
		}
		files = append(files, FileObject{
			Key:          *obj.Key,
			Name:         name,
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
		})
	}

	return files, nil
}

// PutFile uploads a file to MinIO
func (c *Client) PutFile(ctx context.Context, key, contentType string, body io.Reader, size int64) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(c.Bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}
	if size > 0 {
		input.ContentLength = aws.Int64(size)
	}

	_, err := c.S3.PutObject(ctx, input)
	return err
}

// DeleteFile deletes a single object
func (c *Client) DeleteFile(ctx context.Context, key string) error {
	_, err := c.S3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	})
	return err
}

// DeleteFiles deletes multiple objects in one batch call
func (c *Client) DeleteFiles(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	objs := make([]types.ObjectIdentifier, len(keys))
	for i, k := range keys {
		objs[i] = types.ObjectIdentifier{Key: aws.String(k)}
	}
	_, err := c.S3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(c.Bucket),
		Delete: &types.Delete{Objects: objs, Quiet: aws.Bool(true)},
	})
	return err
}

// CopyFile copies an object from srcKey to dstKey within the same bucket
func (c *Client) CopyFile(ctx context.Context, srcKey, dstKey string) error {
	_, err := c.S3.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(c.Bucket),
		CopySource: aws.String(c.Bucket + "/" + srcKey),
		Key:        aws.String(dstKey),
	})
	return err
}

// RenameFile copies then deletes the source object
func (c *Client) RenameFile(ctx context.Context, srcKey, dstKey string) error {
	if err := c.CopyFile(ctx, srcKey, dstKey); err != nil {
		return fmt.Errorf("copy failed: %w", err)
	}
	return c.DeleteFile(ctx, srcKey)
}

// GetObject returns the object body reader (caller must close)
func (c *Client) GetObject(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	out, err := c.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, 0, err
	}
	return out.Body, aws.ToInt64(out.ContentLength), nil
}

// HeadObject returns object metadata without downloading the body
func (c *Client) HeadObject(ctx context.Context, key string) (*s3.HeadObjectOutput, error) {
	return c.S3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	})
}

// PresignGetObject generates a presigned GET URL valid for the given duration
func (c *Client) PresignGetObject(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := c.Presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = ttl
	})
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// PublicURL builds the public URL for an object (for public buckets/prefixes)
func (c *Client) PublicURL(key string) string {
	base := strings.TrimRight(c.cfg.MinioPublicBaseURL, "/")
	return fmt.Sprintf("%s/%s/%s", base, c.Bucket, key)
}
