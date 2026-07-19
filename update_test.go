package music2bb

import (
	"context"
	"errors"
	"testing"
)

type fakeUpdateClient struct {
	current, latest string
	available       bool
	checkErr        error
}

func (f fakeUpdateClient) Check(context.Context) (string, string, bool, error) {
	return f.current, f.latest, f.available, f.checkErr
}

func TestReleaseCheckerReturnsInfo(t *testing.T) {
	checker := &ReleaseChecker{client: fakeUpdateClient{current: "v1.0.0", latest: "v1.1.0", available: true}}
	info, err := checker.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.CurrentVersion != "v1.0.0" || info.LatestVersion != "v1.1.0" || !info.Available {
		t.Fatalf("info = %#v", info)
	}
}

func TestReleaseCheckerCategorizesCancellation(t *testing.T) {
	checker := &ReleaseChecker{client: fakeUpdateClient{checkErr: context.Canceled}}
	_, err := checker.Check(context.Background())
	if !errors.Is(err, context.Canceled) || CategoryOf(err) != ErrorCancelled {
		t.Fatalf("err = %v, category = %s", err, CategoryOf(err))
	}
}
