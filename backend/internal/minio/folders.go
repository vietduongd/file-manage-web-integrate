package minioclient

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// BuildKey constructs the full S3 object key from resource type prefix, folder path, and filename.
// Example: BuildKey("images", "/photos/", "cat.jpg") → "images/photos/cat.jpg"
func BuildKey(prefix, folderPath, filename string) string {
	prefix = strings.Trim(prefix, "/")
	folderPath, _ = NormalizeFolderPath(folderPath)
	filename, _ = NormalizeRelativeObjectPath(filename)
	if folderPath == "" {
		return prefix + "/" + filename
	}
	return prefix + "/" + folderPath + "/" + filename
}

// FolderPrefix returns the full S3 prefix for a folder.
// Example: FolderPrefix("images", "/photos/") → "images/photos/"
func FolderPrefix(prefix, folderPath string) string {
	prefix = strings.Trim(prefix, "/")
	folderPath, _ = NormalizeFolderPath(folderPath)
	if folderPath == "" {
		return prefix + "/"
	}
	return prefix + "/" + folderPath + "/"
}

func NormalizeFolderPath(folderPath string) (string, error) {
	folderPath = strings.Trim(strings.TrimSpace(strings.ReplaceAll(folderPath, "\\", "/")), "/")
	return normalizeRelativePath(folderPath, true)
}

func NormalizeObjectName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid object name")
	}
	if strings.ContainsAny(name, `/\`) || strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("object name must not contain path separators")
	}
	return name, nil
}

func NormalizeRelativeObjectPath(objectPath string) (string, error) {
	return normalizeRelativePath(objectPath, false)
}

func normalizeRelativePath(value string, allowEmpty bool) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || value == "/" {
		if allowEmpty {
			return "", nil
		}
		return "", fmt.Errorf("path is required")
	}
	if strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	if strings.ContainsRune(value, 0) {
		return "", fmt.Errorf("path contains invalid characters")
	}

	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("path contains invalid segment %q", part)
		}
	}

	cleaned := strings.Trim(path.Clean("/"+value), "/")
	if cleaned == "." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path escapes resource root")
	}
	return cleaned, nil
}

// ListFolders returns virtual folder names under a given prefix (one level deep).
func (c *Client) ListFolders(ctx context.Context, prefix string) ([]string, error) {
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

	folders := make([]string, 0, len(out.CommonPrefixes))
	for _, cp := range out.CommonPrefixes {
		if cp.Prefix == nil {
			continue
		}
		// Extract folder name from prefix
		// e.g. "images/photos/vacation/" → strip "images/photos/" → "vacation"
		rel := strings.TrimPrefix(*cp.Prefix, prefix)
		name := strings.TrimSuffix(rel, "/")
		if name != "" && name != ".keep" {
			folders = append(folders, name)
		}
	}

	return folders, nil
}

// HasSubfolders checks if a prefix has any child folders
func (c *Client) HasSubfolders(ctx context.Context, prefix string) (bool, error) {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	out, err := c.S3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(c.Bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})
	if err != nil {
		return false, err
	}

	return len(out.CommonPrefixes) > 0, nil
}

// CreateFolder creates a placeholder object to simulate a folder
func (c *Client) CreateFolder(ctx context.Context, prefix string) error {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	key := prefix + ".keep"

	_, err := c.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.Bucket),
		Key:           aws.String(key),
		ContentLength: aws.Int64(0),
	})
	return err
}

// DeleteFolder deletes all objects under a prefix (recursive)
func (c *Client) DeleteFolder(ctx context.Context, prefix string) error {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	keys, err := c.listObjectKeys(ctx, prefix)
	if err != nil {
		return err
	}
	thumbKeys, err := c.listObjectKeys(ctx, "_thumbs/"+prefix)
	if err != nil {
		return err
	}

	return c.deleteObjectKeysInBatches(ctx, append(keys, thumbKeys...))
}

func (c *Client) listObjectKeys(ctx context.Context, prefix string) ([]string, error) {
	paginator := s3.NewListObjectsV2Paginator(c.S3, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.Bucket),
		Prefix: aws.String(prefix),
	})

	var keys []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
	}

	return keys, nil
}

func (c *Client) deleteObjectKeysInBatches(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	objectsToDelete := make([]types.ObjectIdentifier, len(keys))
	for i, key := range keys {
		objectsToDelete[i] = types.ObjectIdentifier{Key: aws.String(key)}
	}

	// Batch delete (up to 1000 per request)
	for i := 0; i < len(objectsToDelete); i += 1000 {
		end := i + 1000
		if end > len(objectsToDelete) {
			end = len(objectsToDelete)
		}
		batch := objectsToDelete[i:end]
		_, err := c.S3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(c.Bucket),
			Delete: &types.Delete{
				Objects: batch,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// RenameFolder copies all objects from oldPrefix to newPrefix, then deletes old ones
func (c *Client) RenameFolder(ctx context.Context, oldPrefix, newPrefix string) error {
	if !strings.HasSuffix(oldPrefix, "/") {
		oldPrefix += "/"
	}
	if !strings.HasSuffix(newPrefix, "/") {
		newPrefix += "/"
	}

	paginator := s3.NewListObjectsV2Paginator(c.S3, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.Bucket),
		Prefix: aws.String(oldPrefix),
	})

	var oldKeys []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			srcKey := *obj.Key
			dstKey := newPrefix + strings.TrimPrefix(srcKey, oldPrefix)

			// Copy object
			_, err := c.S3.CopyObject(ctx, &s3.CopyObjectInput{
				Bucket:     aws.String(c.Bucket),
				CopySource: aws.String(c.Bucket + "/" + srcKey),
				Key:        aws.String(dstKey),
			})
			if err != nil {
				return err
			}
			oldKeys = append(oldKeys, srcKey)
		}
	}

	// Delete old objects
	toDelete := make([]types.ObjectIdentifier, len(oldKeys))
	for i, k := range oldKeys {
		toDelete[i] = types.ObjectIdentifier{Key: aws.String(k)}
	}
	if len(toDelete) > 0 {
		_, err := c.S3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(c.Bucket),
			Delete: &types.Delete{Objects: toDelete, Quiet: aws.Bool(true)},
		})
		if err != nil {
			return err
		}
	}

	return nil
}
