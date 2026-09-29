package storage

import (
	"clipflow/internal/config"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"io"
	"os"
	"time"
)

type Store struct {
	Client *s3.Client
	Bucket string
}

func New() *Store {
	c := aws.Config{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider(config.Env("S3_ACCESS_KEY", "clipflow"), config.Env("S3_SECRET_KEY", "local-demo-storage-password"), ""), RetryMaxAttempts: 2}
	return &Store{Client: s3.NewFromConfig(c, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(config.Env("S3_ENDPOINT", "http://minio:9000"))
		o.UsePathStyle = true
	}), Bucket: config.Env("S3_BUCKET", "clips")}
}
func (s *Store) Ensure(ctx context.Context) error {
	if err := s.Ready(ctx); err == nil {
		return nil
	}
	_, err := s.Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &s.Bucket})
	if err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}
	return s.Ready(ctx)
}
func (s *Store) Ready(ctx context.Context) error {
	_, err := s.Client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.Bucket})
	return err
}
func (s *Store) Put(ctx context.Context, key, path, contentType string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = s.Client.PutObject(ctx, &s3.PutObjectInput{Bucket: &s.Bucket, Key: &key, Body: f, ContentType: &contentType})
	return err
}
func (s *Store) Get(ctx context.Context, key string) (*s3.GetObjectOutput, error) {
	return s.Client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.Bucket, Key: &key})
}
func (s *Store) Download(ctx context.Context, key, path string) error {
	obj, err := s.Get(ctx, key)
	if err != nil {
		return err
	}
	defer obj.Body.Close()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, obj.Body)
	return err
}
func (s *Store) List(ctx context.Context) ([]types.Object, error) {
	p := s3.NewListObjectsV2Paginator(s.Client, &s3.ListObjectsV2Input{Bucket: &s.Bucket})
	var out []types.Object
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Contents...)
	}
	return out, nil
}
func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.Bucket, Key: &key})
	return err
}
func Timeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 20*time.Second)
}
