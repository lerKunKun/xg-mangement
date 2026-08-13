package objectstore

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/xg-management/platform/backend/internal/config"
)

type Client struct {
	client *s3.Client
	bucket string
}

func New(ctx context.Context, storage config.ObjectStorageConfig) (*Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(storage.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(storage.AccessKeyID, storage.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load object storage configuration: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(storage.Endpoint)
		options.UsePathStyle = storage.UsePathStyle
	})
	return &Client{client: client, bucket: storage.Bucket}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(c.bucket)})
	return err
}

func (c *Client) S3() *s3.Client {
	return c.client
}
