// Package remote holds what drudge does on the service the repositories of a
// project are hosted on, as a port the domain layer calls and an adapter
// carries out.
package remote

// Provider is the service the repositories of a project are hosted on.
type Provider string

const (
	ProviderGitHub Provider = "github"
)

// Providers are the providers a config may name.
var Providers = []Provider{ProviderGitHub}

// Config keys, named in error messages so they match what a user writes in a
// config file.
const (
	PullRequestsEnabledKey = "remote.pullRequests.isEnabled"
	TimeoutKey             = "remote.timeoutSeconds"
)

// Remote is what drudge runs against the provider of a project.
type Remote interface {
	// Provider names the service this remote talks to.
	Provider() Provider
	// CheckReady refuses a provider drudge cannot open pull requests on, and
	// says what the user has to set up.
	CheckReady() error
	// TemplatePaths lists where a repository keeps its pull request template,
	// relative to the repository root, in the order they are looked up.
	TemplatePaths() []string
	// ParseRepository reads the repository a git remote URL points at. A URL
	// of another provider or of an unknown shape is refused.
	ParseRepository(url string) (Repository, error)
	// OpenPullRequest opens a pull request and returns its URL.
	OpenPullRequest(dto PullRequestDto) (string, error)
}

// Repository is one repository on a provider.
type Repository struct {
	Host  string
	Owner string
	Name  string
}

// PullRequestDto is what a pull request is opened from. Head is the branch
// the work is on and Base is the branch it goes into.
type PullRequestDto struct {
	Repository Repository
	Base       string
	Head       string
	Title      string
	Body       string
	IsDraft    bool
}
