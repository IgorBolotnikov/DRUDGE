package release

// ReleaseRepository reads the published releases of drg.
type ReleaseRepository interface {
	// LatestVersion returns the tag of the newest release, like v0.1.1.
	LatestVersion() (string, error)
	// DownloadFile returns the contents of one file attached to a release.
	DownloadFile(version string, fileName string) ([]byte, error)
}
