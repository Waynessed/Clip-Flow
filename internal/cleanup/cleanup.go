package cleanup

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"strings"
	"time"
)

type References interface {
	Referenced(context.Context, string) (bool, error)
}
type Objects interface {
	List(context.Context) ([]types.Object, error)
	Delete(context.Context, string) error
}

// Sweep is explicit. The CLI supplies now - 24h. Check every candidate before
// deleting any so an initially failed reference pass cannot delete objects.
func Sweep(ctx context.Context, q References, s Objects, cutoff time.Time) ([]string, error) {
	objects, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	var candidates []string
	for _, o := range objects {
		if o.Key == nil || o.LastModified == nil || !o.LastModified.Before(cutoff) {
			continue
		}
		key := *o.Key
		if !strings.HasPrefix(key, "inputs/") && !strings.HasPrefix(key, "attempts/") {
			continue
		}
		yes, err := q.Referenced(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("reference check failed; cleanup refused: %w", err)
		}
		if !yes {
			candidates = append(candidates, key)
		}
	}
	var deleted []string
	for _, key := range candidates {
		yes, err := q.Referenced(ctx, key)
		if err != nil {
			return deleted, err
		}
		if yes {
			continue
		}
		if err = s.Delete(ctx, key); err != nil {
			return deleted, err
		}
		deleted = append(deleted, key)
	}
	return deleted, nil
}
