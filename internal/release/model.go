// Package release updates the drg binary to the newest published release.
package release

// UpdateResult is the outcome of one update.
type UpdateResult struct {
	PreviousVersion string
	Version         string
	BinaryPath      string
	IsUpToDate      bool
}
