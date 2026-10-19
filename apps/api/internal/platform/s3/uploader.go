package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

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

// UploadPrivate uploads data to S3 the same way UploadImage does, but never
// constructs or returns a public URL — used for account-deletion archives
// (internal/studios/archive.go), which must never be reachable at a public
// URL the way logos/social-post media are. Callers must not synthesize a
// public URL for a key uploaded through this method.
func (u *Uploader) UploadPrivate(ctx context.Context, key string, data []byte, contentType string) error {
	uploader := manager.NewUploader(u.client)

	_, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("upload to S3 (private): %w", err)
	}
	return nil
}

// Download retrieves an object's raw bytes from S3 — used by the account
// recovery flow (internal/studios/deletion_service.go's RecoverArchive) to
// fetch an encrypted archive back out of S3 before decrypting it
// server-side.
func (u *Uploader) Download(ctx context.Context, key string) ([]byte, error) {
	out, err := u.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get object from S3: %w", err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("read S3 object body: %w", err)
	}
	return data, nil
}

// PresignGet returns a time-limited signed URL for retrieving a private
// object directly from S3 without going through the app. The account
// recovery flow doesn't call this today — it decrypts and streams the
// archive server-side instead (RecoverArchive already re-authenticates the
// requester via the token+email check, so there's no benefit to handing
// out a presigned URL to the *encrypted* blob on top of that). It's kept
// here, ready to use, for ops/support tooling that needs to hand someone a
// direct link to a private object (e.g. inspecting a stuck archive) without
// building a dedicated download endpoint for it.
func (u *Uploader) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(u.client)
	req, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign get: %w", err)
	}
	return req.URL, nil
}
