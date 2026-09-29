package integration_test

import "github.com/aws/aws-sdk-go-v2/service/s3"

func nilSafeBucket(bucket string) *s3.DeleteBucketInput {
	return &s3.DeleteBucketInput{Bucket: &bucket}
}
