// Package github reads drg releases from GitHub over HTTPS.
package github

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"
)

// RepositoryURL is the GitHub repository drg is released from.
const RepositoryURL = "https://github.com/IgorBolotnikov/DRUDGE"

// RequestTimeout caps one request, including reading the body of a download.
const RequestTimeout = 2 * time.Minute

const (
	latestReleasePath   = "/releases/latest"
	downloadPathPattern = "/releases/download/%s/%s"
	locationHeader      = "Location"
)

// Releases reads releases from the web pages of a GitHub repository. It needs
// no API token and has no API rate limit.
type Releases struct {
	client        *http.Client
	repositoryURL string
}

// New returns Releases for the repository at repositoryURL.
func New(repositoryURL string, timeout time.Duration) *Releases {
	client := &http.Client{Timeout: timeout}
	return &Releases{client: client, repositoryURL: repositoryURL}
}

// LatestVersion reads the tag from the redirect GitHub answers the latest
// release page with. The redirect points to the page of the tag.
func (r *Releases) LatestVersion() (string, error) {
	client := *r.client
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	response, err := client.Get(r.repositoryURL + latestReleasePath)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusMultipleChoices || response.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("%s answered %s, expected a redirect to the latest release", response.Request.URL, response.Status)
	}
	location, err := url.Parse(response.Header.Get(locationHeader))
	if err != nil || location.Path == "" {
		return "", errors.New("the latest release redirect has no location")
	}
	return path.Base(location.Path), nil
}

func (r *Releases) DownloadFile(version string, fileName string) ([]byte, error) {
	fileURL := r.repositoryURL + fmt.Sprintf(downloadPathPattern, url.PathEscape(version), url.PathEscape(fileName))
	response, err := r.client.Get(fileURL)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", fileURL, response.Status)
	}
	return io.ReadAll(response.Body)
}
