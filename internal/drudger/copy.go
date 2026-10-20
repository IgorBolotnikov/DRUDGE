package drudger

import (
	"fmt"
	"path/filepath"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

// LocalCopy is one path a launch copies from a repository into its worktree.
type LocalCopy struct {
	Repository string
	Path       string
}

// LocalFileCopied reports a path copied from a repository into its worktree.
type LocalFileCopied struct {
	LocalCopy
}

// LocalFileMissing reports a path to copy that the repository does not have.
// The launch goes on without it.
type LocalFileMissing struct {
	LocalCopy
	Dir string
}

// localCopies lists the paths a launch copies into the worktrees of the given
// repositories, in config order.
func localCopies(repositories []repositoryWorktree) []LocalCopy {
	var copies []LocalCopy
	for _, repository := range repositories {
		for _, path := range repository.Copy {
			copies = append(copies, LocalCopy{Repository: repository.Name, Path: path})
		}
	}
	return copies
}

// copyLocalFiles copies the listed paths of every given repository from the
// repository into its worktree. A path the repository does not have is
// reported and skipped. Any other failure stops the copy.
func (service *DrudgerService) copyLocalFiles(repositories []repositoryWorktree) error {
	for _, repository := range repositories {
		for _, path := range repository.Copy {
			copied := LocalCopy{Repository: repository.Name, Path: path}
			source := filepath.Join(repository.Dir, path)

			isPresent, err := common.Exists(source)
			if err != nil {
				return fmt.Errorf("could not copy %s of repository %s into its worktree: %w", path, repository.Name, err)
			}
			if !isPresent {
				service.progress.Report(LocalFileMissing{LocalCopy: copied, Dir: repository.Dir})
				continue
			}

			if err := common.CopyPath(source, filepath.Join(repository.Worktree, path)); err != nil {
				return fmt.Errorf("could not copy %s of repository %s into its worktree: %w", path, repository.Name, err)
			}
			service.progress.Report(LocalFileCopied{LocalCopy: copied})
		}
	}
	return nil
}
