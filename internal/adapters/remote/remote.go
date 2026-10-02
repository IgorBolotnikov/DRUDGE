package remote

import (
	"fmt"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
)

// CommandRunner runs the CLI of a provider. A command that runs longer than
// timeout is killed and the error wraps context.DeadlineExceeded.
type CommandRunner interface {
	Run(argv []string, timeout time.Duration) (stdout string, stderr string, err error)
}

// New returns the remote of a provider, which runs every command of the
// provider CLI through runner and kills it after timeout. A provider drudge
// does not know is refused.
func New(provider remote.Provider, runner CommandRunner, timeout time.Duration) (remote.Remote, error) {
	switch provider {
	case remote.ProviderGitHub:
		return NewGitHub(runner, timeout), nil
	default:
		return nil, fmt.Errorf("remote provider %q is not supported, the supported ones are %v", provider, remote.Providers)
	}
}
