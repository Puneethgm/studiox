package s3

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Uploader struct {
	client        *s3.Client
	bucket        string
	publicURLBase string
}

func NewUploader(client *s3.Client, bucket, publicURLBase string) *Uploader {
	return &Uploader{
		client:        client,
		bucket:        bucket,
		publicURLBase: publicURLBase,
	}
}

// UploadImage uploads an image to S3 and returns a pre-signed URL.
// key should be the S3 object key (e.g., "logos/studio-id.png" or "social-posts/studio-id/file.jpg")
// data is the raw file bytes
// contentType is the MIME type (e.g., "image/png")
func (u *Uploader) UploadImage(ctx context.Context, key string, data []byte, contentType string) (publicURL string, err error) {
	uploader := manager.NewUploader(u.client)

	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})

	if err != nil {
		return "", fmt.Errorf("upload to S3: %w", err)
	}

	// Return public S3 URL (bucket is now public)
	publicURL = fmt.Sprintf("%s/%s", u.publicURLBase, key)
	return publicURL, nil
}

// DeleteImage deletes an image from S3 by key
func (u *Uploader) DeleteImage(ctx context.Context, key string) error {
	_, err := u.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete from S3: %w", err)
	}
	return nil
}
