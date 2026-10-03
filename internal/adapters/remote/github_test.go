package remote

import (
	"context"
	"errors"
	"fmt"
	osexec "os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
)

// fakeRunner answers every command with stdout and err, and remembers what it
// ran and what it wrote to stdin.
type fakeRunner struct {
	stdout   string
	err      error
	calls    [][]string
	inputs   []string
	timeouts []time.Duration
}

func (runner *fakeRunner) Run(argv []string, timeout time.Duration) (string, string, error) {
	return runner.RunWithInput(argv, "", timeout)
}

func (runner *fakeRunner) RunWithInput(argv []string, input string, timeout time.Duration) (string, string, error) {
	runner.calls = append(runner.calls, argv)
	runner.inputs = append(runner.inputs, input)
	runner.timeouts = append(runner.timeouts, timeout)
	if runner.err != nil {
		return "", "", runner.err
	}
	return runner.stdout, "", nil
}

func TestGitHub_ParseRepository(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    remote.Repository
		wantErr bool
	}{
		{
			name: "scp-like ssh",
			url:  "git@github.com:owner/name.git",
			want: remote.Repository{Host: "github.com", Owner: "owner", Name: "name"},
		},
		{
			name: "ssh URL",
			url:  "ssh://git@github.com/owner/name.git",
			want: remote.Repository{Host: "github.com", Owner: "owner", Name: "name"},
		},
		{
			name: "ssh URL with a port",
			url:  "ssh://git@github.com:22/owner/name.git",
			want: remote.Repository{Host: "github.com", Owner: "owner", Name: "name"},
		},
		{
			name: "https URL with .git",
			url:  "https://github.com/owner/name.git",
			want: remote.Repository{Host: "github.com", Owner: "owner", Name: "name"},
		},
		{
			name: "https URL without .git",
			url:  "https://github.com/owner/name",
			want: remote.Repository{Host: "github.com", Owner: "owner", Name: "name"},
		},
		{
			name: "enterprise host",
			url:  "git@github.example.com:owner/name.git",
			want: remote.Repository{Host: "github.example.com", Owner: "owner", Name: "name"},
		},
		{name: "local path", url: "/srv/git/name.git", wantErr: true},
		{name: "file URL", url: "file:///srv/git/name.git", wantErr: true},
		{name: "no owner", url: "https://github.com/name.git", wantErr: true},
		{name: "nested path", url: "https://gitlab.com/group/subgroup/name.git", wantErr: true},
		{name: "no host", url: "git@:owner/name.git", wantErr: true},
		{name: "empty", url: "", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NewGitHub(&fakeRunner{}, time.Second).ParseRepository(test.url)
			if test.wantErr {
				if err == nil {
					t.Fatalf("ParseRepository(%q) = %+v, want an error", test.url, got)
				}
				if !strings.Contains(err.Error(), test.url) {
					t.Errorf("error = %q, want it to name the URL", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRepository(%q): %v", test.url, err)
			}
			if got != test.want {
				t.Errorf("ParseRepository(%q) = %+v, want %+v", test.url, got, test.want)
			}
		})
	}
}

func TestGitHub_CheckReady(t *testing.T) {
	const timeout = 7 * time.Second
	tests := []struct {
		name string
		err  error
		// wantErrText lists fragments the error must carry. A test with none
		// expects no error.
		wantErrText []string
	}{
		{name: "ready"},
		{
			name:        "gh missing",
			err:         fmt.Errorf("command gh failed: %w", &osexec.Error{Name: "gh", Err: osexec.ErrNotFound}),
			wantErrText: []string{"https://cli.github.com", "gh auth login", remote.PullRequestsEnabledKey},
		},
		{
			name:        "logged out",
			err:         errors.New("command gh failed: exit status 1: You are not logged into any GitHub hosts"),
			wantErrText: []string{"not logged in", "gh auth login"},
		},
		{
			name:        "gh hanging",
			err:         fmt.Errorf("command gh auth status did not finish within 7s and was killed: %w", context.DeadlineExceeded),
			wantErrText: []string{remote.TimeoutKey},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeRunner{err: test.err}

			err := NewGitHub(runner, timeout).CheckReady()

			if want := [][]string{{"gh", "auth", "status"}}; !slices.EqualFunc(runner.calls, want, slices.Equal) {
				t.Errorf("ran %v, want %v", runner.calls, want)
			}
			if want := []time.Duration{timeout}; !slices.Equal(runner.timeouts, want) {
				t.Errorf("ran with timeouts %v, want %v", runner.timeouts, want)
			}
			if len(test.wantErrText) == 0 {
				if err != nil {
					t.Fatalf("CheckReady: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error")
			}
			for _, fragment := range test.wantErrText {
				if !strings.Contains(err.Error(), fragment) {
					t.Errorf("error = %q, want it to name %q", err, fragment)
				}
			}
		})
	}
}

func TestGitHub_OpenPullRequest(t *testing.T) {
	const (
		timeout        = 7 * time.Second
		pullRequestURL = "https://github.com/owner/name/pull/42"
	)
	dto := remote.PullRequestDto{
		Repository: remote.Repository{Host: "github.com", Owner: "owner", Name: "name"},
		Base:       "main",
		Head:       "drudge/3f9a-add-retry",
		Title:      "Add retry to uploader",
		Body:       "## What\n\nRetries uploads.",
	}
	createArgv := []string{
		"gh", "pr", "create",
		"--repo", "github.com/owner/name",
		"--base", "main",
		"--head", "drudge/3f9a-add-retry",
		"--title", "Add retry to uploader",
		"--body-file", "-",
	}

	tests := []struct {
		name     string
		isDraft  bool
		stdout   string
		err      error
		wantArgv []string
		want     string
		// wantErrText lists fragments the error must carry. A test with none
		// expects no error.
		wantErrText []string
	}{
		{
			name:     "opened",
			stdout:   pullRequestURL + "\n",
			wantArgv: createArgv,
			want:     pullRequestURL,
		},
		{
			name:     "opened as a draft",
			isDraft:  true,
			stdout:   pullRequestURL + "\n",
			wantArgv: append(slices.Clone(createArgv), "--draft"),
			want:     pullRequestURL,
		},
		{
			name:     "the URL is the last line gh prints",
			stdout:   "Warning: 1 uncommitted change\n" + pullRequestURL + "\n",
			wantArgv: createArgv,
			want:     pullRequestURL,
		},
		{
			name:        "gh refused",
			err:         errors.New("command gh failed: exit status 1: a pull request already exists"),
			wantArgv:    createArgv,
			wantErrText: []string{"already exists", "drudge/3f9a-add-retry", "github.com/owner/name"},
		},
		{
			name:        "gh hanging",
			err:         fmt.Errorf("command gh pr create did not finish within 7s and was killed: %w", context.DeadlineExceeded),
			wantArgv:    createArgv,
			wantErrText: []string{remote.TimeoutKey},
		},
		{
			name:        "gh printed no URL",
			wantArgv:    createArgv,
			wantErrText: []string{"no URL"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeRunner{stdout: test.stdout, err: test.err}
			request := dto
			request.IsDraft = test.isDraft

			got, err := NewGitHub(runner, timeout).OpenPullRequest(request)

			if want := [][]string{test.wantArgv}; !slices.EqualFunc(runner.calls, want, slices.Equal) {
				t.Errorf("ran %v, want %v", runner.calls, want)
			}
			if want := []string{dto.Body}; !slices.Equal(runner.inputs, want) {
				t.Errorf("wrote %q to stdin, want %q", runner.inputs, want)
			}
			if want := []time.Duration{timeout}; !slices.Equal(runner.timeouts, want) {
				t.Errorf("ran with timeouts %v, want %v", runner.timeouts, want)
			}
			if len(test.wantErrText) == 0 {
				if err != nil {
					t.Fatalf("OpenPullRequest: %v", err)
				}
				if got != test.want {
					t.Errorf("OpenPullRequest() = %q, want %q", got, test.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error, got %q", got)
			}
			for _, fragment := range test.wantErrText {
				if !strings.Contains(err.Error(), fragment) {
					t.Errorf("error = %q, want it to name %q", err, fragment)
				}
			}
		})
	}
}

func TestGitHub_TemplatePaths(t *testing.T) {
	want := []string{".github/pull_request_template.md", "pull_request_template.md", "docs/pull_request_template.md"}
	if got := NewGitHub(&fakeRunner{}, time.Second).TemplatePaths(); !slices.Equal(got, want) {
		t.Errorf("TemplatePaths() = %v, want %v", got, want)
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		provider remote.Provider
		wantErr  bool
	}{
		{name: "github", provider: remote.ProviderGitHub},
		{name: "unknown provider", provider: "gitlab", wantErr: true},
		{name: "no provider", provider: "", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := New(test.provider, &fakeRunner{}, time.Second)
			if test.wantErr {
				if err == nil {
					t.Fatalf("New(%q) = %v, want an error", test.provider, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("New(%q): %v", test.provider, err)
			}
			if got.Provider() != test.provider {
				t.Errorf("New(%q) talks to %q", test.provider, got.Provider())
			}
		})
	}
}
