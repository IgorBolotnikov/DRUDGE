package drudger

import (
	"fmt"
	"path/filepath"

	"github.com/IgorBolotnikov/DRUDGE/internal/git"
	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
)

// checkRemote refuses a run that could not open its pull requests. The
// provider has to be ready, and every repository of the project needs an
// origin remote whose URL the provider reads. A project with pull requests off
// passes.
func (service *DrudgerService) checkRemote(layout projectLayout) error {
	if service.remote == nil {
		return nil
	}
	if err := service.remote.CheckReady(); err != nil {
		return err
	}

	for _, repository := range service.settings.Repositories {
		dir := filepath.Join(layout.Dir, repository.Path)
		name := repositoryNameOf(dir)

		hasOrigin, err := service.gitOps.HasRemote(dir, git.OriginRemote)
		if err != nil {
			return err
		}
		if !hasOrigin {
			return fmt.Errorf(
				"repository %s has no %s remote to open pull requests on, add one with git remote add %s <url>, or turn off %s",
				name, git.OriginRemote, git.OriginRemote, remote.PullRequestsEnabledKey,
			)
		}

		remoteURL, err := service.gitOps.RemoteURL(dir, git.OriginRemote)
		if err != nil {
			return err
		}
		if _, err := service.remote.ParseRepository(remoteURL); err != nil {
			return fmt.Errorf(
				"the %s remote of repository %s is not on %s, point it there or turn off %s: %w",
				git.OriginRemote, name, service.remote.Provider(), remote.PullRequestsEnabledKey, err,
			)
		}
	}
	return nil
}
