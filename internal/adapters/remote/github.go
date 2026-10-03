package remote

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	osexec "os/exec"
	"slices"
	"strings"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
)

// Pieces of a gh invocation.
const (
	ghBinary         = "gh"
	authSubcommand   = "auth"
	statusSubcommand = "status"
	loginCommand     = "gh auth login"
	prSubcommand     = "pr"
	createSubcommand = "create"
	repoFlag         = "--repo"
	baseFlag         = "--base"
	headFlag         = "--head"
	titleFlag        = "--title"
	bodyFileFlag     = "--body-file"
	draftFlag        = "--draft"
	// stdinFile makes gh read the body from stdin.
	stdinFile    = "-"
	ghInstallURL = "https://cli.github.com"
)

// Pieces of a git remote URL.
const (
	schemeSeparator  = "://"
	userSeparator    = "@"
	scpPathSeparator = ":"
	pathSeparator    = "/"
	gitSuffix        = ".git"
)

// remoteURLSchemes are the schemes of a remote URL GitHub serves.
var remoteURLSchemes = []string{"https", "ssh"}

// templatePaths are where GitHub looks for the pull request template of a
// repository.
var templatePaths = []string{
	".github/pull_request_template.md",
	"pull_request_template.md",
	"docs/pull_request_template.md",
}

// GitHub runs gh on the host. The sandbox of a Drudger never runs it.
type GitHub struct {
	runner  CommandRunner
	timeout time.Duration
}

// NewGitHub returns a GitHub remote that kills every gh command after timeout.
func NewGitHub(runner CommandRunner, timeout time.Duration) *GitHub {
	return &GitHub{runner: runner, timeout: timeout}
}

func (github *GitHub) Provider() remote.Provider {
	return remote.ProviderGitHub
}

// CheckReady runs gh auth status. It refuses a host with no gh on its path and
// a gh that is not logged in to GitHub.
func (github *GitHub) CheckReady() error {
	_, _, err := github.runner.Run([]string{ghBinary, authSubcommand, statusSubcommand}, github.timeout)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, osexec.ErrNotFound):
		return fmt.Errorf(
			"pull requests are on and %s is not installed, install it from %s and run %s, or turn off %s",
			ghBinary, ghInstallURL, loginCommand, remote.PullRequestsEnabledKey,
		)
	case errors.Is(err, context.DeadlineExceeded):
		return github.timeoutError(err)
	default:
		return fmt.Errorf("%s is not logged in to GitHub, run %s: %w", ghBinary, loginCommand, err)
	}
}

func (github *GitHub) timeoutError(err error) error {
	return fmt.Errorf("%s did not answer within %s, raise %q if it needs longer: %w", ghBinary, github.timeout, remote.TimeoutKey, err)
}

func (github *GitHub) TemplatePaths() []string {
	return slices.Clone(templatePaths)
}

// ParseRepository reads a remote URL of the shape git@host:owner/name.git,
// ssh://git@host/owner/name.git or https://host/owner/name, where the .git
// suffix is optional.
func (github *GitHub) ParseRepository(remoteURL string) (remote.Repository, error) {
	host, repositoryPath, ok := splitRemoteURL(remoteURL)
	if !ok {
		return remote.Repository{}, unparsableURLError(remoteURL)
	}

	owner, name, ok := strings.Cut(strings.TrimSuffix(strings.Trim(repositoryPath, pathSeparator), gitSuffix), pathSeparator)
	if !ok || owner == "" || name == "" || strings.Contains(name, pathSeparator) {
		return remote.Repository{}, unparsableURLError(remoteURL)
	}
	return remote.Repository{Host: host, Owner: owner, Name: name}, nil
}

// splitRemoteURL cuts a remote URL into its host and the path of the
// repository on it. A URL with no scheme is read as the scp-like syntax git
// accepts for ssh.
func splitRemoteURL(remoteURL string) (host string, repositoryPath string, ok bool) {
	if strings.Contains(remoteURL, schemeSeparator) {
		parsed, err := url.Parse(remoteURL)
		if err != nil || !slices.Contains(remoteURLSchemes, parsed.Scheme) || parsed.Hostname() == "" {
			return "", "", false
		}
		return parsed.Hostname(), parsed.Path, true
	}

	userHost, repositoryPath, ok := strings.Cut(remoteURL, scpPathSeparator)
	if !ok {
		return "", "", false
	}
	host = userHost[strings.LastIndex(userHost, userSeparator)+1:]
	if host == "" {
		return "", "", false
	}
	return host, repositoryPath, true
}

func unparsableURLError(remoteURL string) error {
	return fmt.Errorf(
		"%q is not a GitHub repository URL, expected git@github.com:owner/name.git, ssh://git@github.com/owner/name.git or https://github.com/owner/name.git",
		remoteURL,
	)
}

// OpenPullRequest runs gh pr create with the body on stdin and returns the URL
// gh prints on its last line of output. The head branch has to be on GitHub
// already.
func (github *GitHub) OpenPullRequest(dto remote.PullRequestDto) (string, error) {
	repository := strings.Join([]string{dto.Repository.Host, dto.Repository.Owner, dto.Repository.Name}, pathSeparator)
	argv := []string{
		ghBinary, prSubcommand, createSubcommand,
		repoFlag, repository,
		baseFlag, dto.Base,
		headFlag, dto.Head,
		titleFlag, dto.Title,
		bodyFileFlag, stdinFile,
	}
	if dto.IsDraft {
		argv = append(argv, draftFlag)
	}

	stdout, _, err := github.runner.RunWithInput(argv, dto.Body, github.timeout)
	if errors.Is(err, context.DeadlineExceeded) {
		return "", github.timeoutError(err)
	}
	if err != nil {
		return "", fmt.Errorf("could not open a pull request from %s into %s on %s: %w", dto.Head, dto.Base, repository, err)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	pullRequestURL := strings.TrimSpace(lines[len(lines)-1])
	if pullRequestURL == "" {
		return "", fmt.Errorf("%s opened a pull request from %s into %s on %s and printed no URL", ghBinary, dto.Head, dto.Base, repository)
	}
	return pullRequestURL, nil
}
