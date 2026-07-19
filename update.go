package music2bb

import (
	"context"
	"errors"

	"github.com/bagags/music2bb-go/internal/selfupdate"
)

// UpdateInfo describes the latest verified release available for this build.
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	Available      bool
}

type updateChecker interface {
	Check(context.Context) (string, string, bool, error)
}

// ReleaseChecker checks GitHub Releases for a newer verified core release.
// It deliberately does not replace a desktop executable: desktop artifacts are
// not part of the CLI release set.
type ReleaseChecker struct{ client updateChecker }

// NewReleaseChecker returns a release checker for the supplied application
// version. Development builds can still discover the latest stable release.
func NewReleaseChecker(currentVersion string) *ReleaseChecker {
	return &ReleaseChecker{client: selfupdate.New(currentVersion)}
}

// Check reports whether a newer verified release is available.
func (c *ReleaseChecker) Check(ctx context.Context) (UpdateInfo, error) {
	if c == nil || c.client == nil {
		return UpdateInfo{}, &Error{Category: ErrorInvalidInput, Operation: "check update", Message: "updater is not configured"}
	}
	current, latest, available, err := c.client.Check(ctx)
	info := UpdateInfo{CurrentVersion: current, LatestVersion: latest, Available: available}
	if err != nil {
		return info, wrapUpdateError("check update", err)
	}
	return info, nil
}

func wrapUpdateError(operation string, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &Error{Category: ErrorCancelled, Operation: operation, Err: err}
	}
	return &Error{Category: ErrorNetwork, Operation: operation, Err: err}
}
