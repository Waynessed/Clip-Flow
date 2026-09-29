package cleanup

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"testing"
	"time"
)

type refs struct{ fail bool }

func (r refs) Referenced(ctx context.Context, key string) (bool, error) {
	if r.fail {
		return false, errors.New("db down")
	}
	return key == "inputs/referenced", nil
}

type objects struct {
	deleted []string
	now     time.Time
}

func (o *objects) List(context.Context) ([]types.Object, error) {
	old := o.now.Add(-25 * time.Hour)
	recent := o.now.Add(-time.Hour)
	a, b, c, d := "inputs/referenced", "attempts/orphan/preview.mp4", "inputs/recent", "other/unknown"
	return []types.Object{{Key: &a, LastModified: &old}, {Key: &b, LastModified: &old}, {Key: &c, LastModified: &recent}, {Key: &d, LastModified: &old}}, nil
}
func (o *objects) Delete(_ context.Context, key string) error {
	o.deleted = append(o.deleted, key)
	return nil
}
func TestSweepPreservesReferencesAgeAndUnknownNamespaces(t *testing.T) {
	s := &objects{now: time.Now()}
	deleted, err := Sweep(context.Background(), refs{}, s, s.now.Add(-24*time.Hour))
	if err != nil || len(deleted) != 1 || deleted[0] != "attempts/orphan/preview.mp4" {
		t.Fatalf("%v %v", deleted, err)
	}
}
func TestSweepRefusesDeletionWhenDatabaseUnavailable(t *testing.T) {
	s := &objects{now: time.Now()}
	_, err := Sweep(context.Background(), refs{fail: true}, s, s.now.Add(-24*time.Hour))
	if err == nil || len(s.deleted) != 0 {
		t.Fatal("deleted while reference checks failed")
	}
}
