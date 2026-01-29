package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Storage implements Storage using AWS S3
type S3Storage struct {
	client *s3.Client
	bucket string
	prefix string
}

// NewS3Storage creates a new S3Storage instance
func NewS3Storage(client *s3.Client, bucket string, prefix string) *S3Storage {
	return &S3Storage{
		client: client,
		bucket: bucket,
		prefix: prefix,
	}
}

// objectKey returns the full S3 object key for a note
func (ss *S3Storage) objectKey(noteID string) string {
	return fmt.Sprintf("%s/%s", strings.TrimSuffix(ss.prefix, "/"), noteID)
}

// Read retrieves note content from S3
func (ss *S3Storage) Read(ctx context.Context, noteID string) (string, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(ss.bucket),
		Key:    aws.String(ss.objectKey(noteID)),
	}

	result, err := ss.client.GetObject(ctx, input)
	if err != nil {
		// Check if it's a NoSuchKey error
		if strings.Contains(err.Error(), "NoSuchKey") {
			return "", nil // Return empty string for missing note
		}
		return "", fmt.Errorf("failed to read note from S3: %w", err)
	}
	defer func() {
		_ = result.Body.Close()
	}()

	content, err := io.ReadAll(result.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read note content: %w", err)
	}

	return string(content), nil
}

// Write saves note content to S3
func (ss *S3Storage) Write(ctx context.Context, noteID string, content string) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(ss.bucket),
		Key:    aws.String(ss.objectKey(noteID)),
		Body:   strings.NewReader(content),
	}

	_, err := ss.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to write note to S3: %w", err)
	}

	return nil
}

// Delete removes a note from S3
func (ss *S3Storage) Delete(ctx context.Context, noteID string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(ss.bucket),
		Key:    aws.String(ss.objectKey(noteID)),
	}

	_, err := ss.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete note from S3: %w", err)
	}

	return nil
}
