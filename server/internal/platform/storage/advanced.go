package storage

import (
	"context"
	"errors"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const maxPresignTTL = 15 * time.Minute

type MultipartPart struct {
	Number         int32
	ETag           string
	ChecksumSHA256 string
}

type MultipartStore interface {
	Store
	CreateMultipart(context.Context, string, string) (string, error)
	UploadPart(context.Context, string, string, int32, io.Reader, string) (MultipartPart, error)
	CompleteMultipart(context.Context, string, string, []MultipartPart) error
	AbortMultipart(context.Context, string, string) error
}

type PresignStore interface {
	Store
	PresignGet(context.Context, string, time.Duration) (string, error)
	PresignPut(context.Context, string, time.Duration, string) (string, error)
}

func validateMultipartKey(key string) error {
	if strings.TrimSpace(key) == "" || len(key) > 1024 {
		return errors.New("invalid multipart object key")
	}
	return nil
}

func validateMultipartPart(partNumber int32) error {
	if partNumber < 1 || partNumber > 10000 {
		return errors.New("multipart part number must be between 1 and 10000")
	}
	return nil
}

func validatePresignTTL(ttl time.Duration) error {
	if ttl < time.Minute || ttl > maxPresignTTL {
		return errors.New("presign ttl must be between 1 minute and 15 minutes")
	}
	return nil
}

func (s *s3Store) CreateMultipart(ctx context.Context, key, contentType string) (string, error) {
	if err := validateMultipartKey(key); err != nil {
		return "", err
	}
	if err := RequireTargetTestedCapabilities(s, CapabilityMultipartRecovery); err != nil {
		return "", err
	}
	input := &s3.CreateMultipartUploadInput{Bucket: &s.bucket, Key: &key}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	out, err := s.client.CreateMultipartUpload(ctx, input)
	if err != nil {
		return "", err
	}
	if out.UploadId == nil || strings.TrimSpace(*out.UploadId) == "" {
		return "", errors.New("multipart create did not return an upload id")
	}
	return *out.UploadId, nil
}

func (s *s3Store) UploadPart(ctx context.Context, key, uploadID string, partNumber int32, body io.Reader, checksumSHA256 string) (MultipartPart, error) {
	if err := validateMultipartKey(key); err != nil {
		return MultipartPart{}, err
	}
	if err := validateMultipartPart(partNumber); err != nil {
		return MultipartPart{}, err
	}
	if strings.TrimSpace(uploadID) == "" || body == nil {
		return MultipartPart{}, errors.New("multipart upload id and body are required")
	}
	if err := RequireTargetTestedCapabilities(s, CapabilityMultipartRecovery); err != nil {
		return MultipartPart{}, err
	}
	input := &s3.UploadPartInput{Bucket: &s.bucket, Key: &key, UploadId: &uploadID, PartNumber: aws.Int32(partNumber), Body: body}
	if checksumSHA256 != "" {
		input.ChecksumSHA256 = aws.String(checksumSHA256)
	}
	out, err := s.client.UploadPart(ctx, input)
	if err != nil {
		return MultipartPart{}, err
	}
	if out.ETag == nil || strings.TrimSpace(*out.ETag) == "" {
		return MultipartPart{}, errors.New("multipart upload did not return an etag")
	}
	part := MultipartPart{Number: partNumber, ETag: *out.ETag}
	if out.ChecksumSHA256 != nil {
		part.ChecksumSHA256 = *out.ChecksumSHA256
	}
	return part, nil
}

func (s *s3Store) CompleteMultipart(ctx context.Context, key, uploadID string, parts []MultipartPart) error {
	if err := validateMultipartKey(key); err != nil {
		return err
	}
	if strings.TrimSpace(uploadID) == "" || len(parts) == 0 {
		return errors.New("multipart upload id and parts are required")
	}
	if err := RequireTargetTestedCapabilities(s, CapabilityMultipartRecovery); err != nil {
		return err
	}
	ordered := append([]MultipartPart(nil), parts...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Number < ordered[j].Number })
	completed := make([]types.CompletedPart, 0, len(ordered))
	var last int32
	for _, part := range ordered {
		if err := validateMultipartPart(part.Number); err != nil || part.Number == last || strings.TrimSpace(part.ETag) == "" {
			return errors.New("multipart parts must have unique valid numbers and etags")
		}
		last = part.Number
		item := types.CompletedPart{PartNumber: aws.Int32(part.Number), ETag: aws.String(part.ETag)}
		if part.ChecksumSHA256 != "" {
			item.ChecksumSHA256 = aws.String(part.ChecksumSHA256)
		}
		completed = append(completed, item)
	}
	_, err := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: &s.bucket, Key: &key, UploadId: &uploadID, MultipartUpload: &types.CompletedMultipartUpload{Parts: completed},
	})
	return err
}

func (s *s3Store) AbortMultipart(ctx context.Context, key, uploadID string) error {
	if err := validateMultipartKey(key); err != nil {
		return err
	}
	if strings.TrimSpace(uploadID) == "" {
		return errors.New("multipart upload id is required")
	}
	if err := RequireTargetTestedCapabilities(s, CapabilityMultipartRecovery); err != nil {
		return err
	}
	_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{Bucket: &s.bucket, Key: &key, UploadId: &uploadID})
	return err
}

func (s *s3Store) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if err := validateMultipartKey(key); err != nil {
		return "", err
	}
	if err := validatePresignTTL(ttl); err != nil {
		return "", err
	}
	if err := RequireTargetTestedCapabilities(s, CapabilityConstrainedPresign); err != nil {
		return "", err
	}
	out, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key}, func(options *s3.PresignOptions) { options.Expires = ttl })
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func (s *s3Store) PresignPut(ctx context.Context, key string, ttl time.Duration, contentType string) (string, error) {
	if err := validateMultipartKey(key); err != nil {
		return "", err
	}
	if err := validatePresignTTL(ttl); err != nil {
		return "", err
	}
	if err := RequireTargetTestedCapabilities(s, CapabilityConstrainedPresign); err != nil {
		return "", err
	}
	input := &s3.PutObjectInput{Bucket: &s.bucket, Key: &key}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	out, err := s.presign.PresignPutObject(ctx, input, func(options *s3.PresignOptions) { options.Expires = ttl })
	if err != nil {
		return "", err
	}
	return out.URL, nil
}
