package persistence

import (
	"path/filepath"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

// FileDraftRepository keeps each draft of a project as a markdown file in the
// drafts directory of the project.
type FileDraftRepository struct {
	// home is the home directory the drafts live under. An empty one stands
	// for the home directory of the user. This attribute is mostly for testing
	// purposes.
	home string
}

func NewFileDraftRepository(home string) *FileDraftRepository {
	return &FileDraftRepository{home: home}
}

func (repo *FileDraftRepository) ReadDraft(projectSlug string, name string) (string, bool, error) {
	path, err := repo.DraftPath(projectSlug, name)
	if err != nil {
		return "", false, err
	}
	isFound, err := common.Exists(path)
	if err != nil || !isFound {
		return "", false, err
	}
	content, err := common.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	return content, true, nil
}

// WriteDraft writes a draft and creates the drafts directory of the project
// when it is missing.
func (repo *FileDraftRepository) WriteDraft(projectSlug string, name string, content string) error {
	path, err := repo.DraftPath(projectSlug, name)
	if err != nil {
		return err
	}
	if err := common.EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return common.WriteFile(path, content)
}

func (repo *FileDraftRepository) RemoveDraft(projectSlug string, name string) error {
	path, err := repo.DraftPath(projectSlug, name)
	if err != nil {
		return err
	}
	return common.RemoveAll(path)
}

func (repo *FileDraftRepository) DraftPath(projectSlug string, name string) (string, error) {
	home := repo.home
	if home == "" {
		userHome, err := common.HomeDir()
		if err != nil {
			return "", err
		}
		home = userHome
	}
	return common.DraftPath(home, projectSlug, name), nil
}
