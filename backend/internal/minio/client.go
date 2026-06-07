package minioclient

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ckfindercompatible/backend/internal/config"
	"go.uber.org/zap"
)

// Client wraps the S3 client with app-specific helpers
type Client struct {
	S3     *s3.Client
	Presign *s3.PresignClient
	Bucket string
	cfg    *config.Config
	logger *zap.Logger
}

var instance *Client

// New creates a new MinIO client using the app config
func New(cfg *config.Config, logger *zap.Logger) (*Client, error) {
	scheme := "http"
	if cfg.MinioUseSSL {
		scheme = "https"
	}
	endpoint := fmt.Sprintf("%s://%s", scheme, cfg.MinioEndpoint)

	resolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               endpoint,
				HostnameImmutable: true,
				SigningRegion:     "us-east-1",
			}, nil
		},
	)

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithEndpointResolverWithOptions(resolver),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.MinioAccessKey,
			cfg.MinioSecretKey,
			"",
		)),
		awsconfig.WithRegion("us-east-1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true // Required for MinIO
	})

	instance = &Client{
		S3:      s3Client,
		Presign: s3.NewPresignClient(s3Client),
		Bucket:  cfg.MinioBucket,
		cfg:     cfg,
		logger:  logger,
	}

	return instance, nil
}

// Get returns the singleton client instance
func Get() *Client {
	if instance == nil {
		panic("minio client not initialized, call minioclient.New() first")
	}
	return instance
}

// EnsureBucket creates the bucket if it does not exist
func (c *Client) EnsureBucket(ctx context.Context) error {
	_, err := c.S3.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(c.Bucket),
	})
	if err == nil {
		return nil // already exists
	}

	_, err = c.S3.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(c.Bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket %q: %w", c.Bucket, err)
	}

	c.logger.Info("created MinIO bucket", zap.String("bucket", c.Bucket))
	return nil
}
