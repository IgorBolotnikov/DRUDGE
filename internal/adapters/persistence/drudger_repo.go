package persistence

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"drudge/internal/common"
	"drudge/internal/drudger"
	"drudge/internal/task"
)

const (
	// DrudgersFileName is the file listing project's Drudgers.
	DrudgersFileName = "drudgers.json"

	// drudgersLockFileName guards the Drudgers file against two processes
	// handing out the same slot. It has no content.
	//
	// The lock lives in its own file because it has to be takeable before a
	// project has any Drudgers. Creating the Drudgers file to lock it
	// would leave an empty one that reads back as malformed JSON.
	drudgersLockFileName = DrudgersFileName + ".lock"
)

// drudgersFile is the stored shape of project's Drudgers.
type drudgersFile struct {
	Drudgers []storedDrudger `json:"drudgers"`
}

// storedDrudger is one Drudger as it is written to file.
type storedDrudger struct {
	Slot          int       `json:"slot"`
	Sandbox       string    `json:"sandbox"`
	Task          string    `json:"task,omitempty"`
	SandboxHealth string    `json:"sandboxHealth,omitempty"`
	AgentHealth   string    `json:"agentHealth,omitempty"`
	LastChecked   time.Time `json:"lastChecked"`
}

// FileDrudgerRepository keeps each project's Drudgers in a JSON file inside
// the global projects directory.
type FileDrudgerRepository struct {
	// This attribute is mostly for testing purposes.
	projectsDirPath string
}

func NewFileDrudgerRepository(projectsDirPath string) *FileDrudgerRepository {
	return &FileDrudgerRepository{projectsDirPath: projectsDirPath}
}

// ListDrudgers reads project's Drudgers.
func (repo *FileDrudgerRepository) ListDrudgers(projectSlug string) ([]*drudger.Drudger, error) {
	path, err := repo.drudgersFilePath(projectSlug)
	if err != nil {
		return nil, err
	}
	return readDrudgersFile(path)
}

// UpdateDrudgers runs change against the project's Drudgers under an exclusive
// lock and writes back what it returns.
func (repo *FileDrudgerRepository) UpdateDrudgers(projectSlug string, change func([]*drudger.Drudger) ([]*drudger.Drudger, error)) error {
	projectDir, err := repo.resolveProjectDir(projectSlug)
	if err != nil {
		return err
	}
	if err := common.EnsureDir(projectDir); err != nil {
		return err
	}

	unlock, err := lockDrudgers(filepath.Join(projectDir, drudgersLockFileName))
	if err != nil {
		return err
	}
	defer unlock()

	path := filepath.Join(projectDir, DrudgersFileName)

	drudgers, err := readDrudgersFile(path)
	if err != nil {
		return err
	}

	updated, err := change(drudgers)
	if err != nil {
		return err
	}

	return writeDrudgersFile(path, updated)
}

// readDrudgersFile parses the Drudgers from a file ans returns their pool.
// If file is missing, it returns nil which also means a pool is empty.
func readDrudgersFile(path string) ([]*drudger.Drudger, error) {
	exists, err := common.Exists(path)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	var stored drudgersFile
	if err := common.ReadJSON(path, &stored); err != nil {
		return nil, err
	}

	drudgers := make([]*drudger.Drudger, 0, len(stored.Drudgers))
	for _, entry := range stored.Drudgers {
		drudgers = append(drudgers, &drudger.Drudger{
			Slot:          entry.Slot,
			Sandbox:       entry.Sandbox,
			TaskID:        task.TaskID(entry.Task),
			SandboxHealth: drudger.SandboxHealth(entry.SandboxHealth),
			AgentHealth:   drudger.AgentHealth(entry.AgentHealth),
			LastChecked:   entry.LastChecked,
		})
	}
	return drudgers, nil
}

// writeDrudgersFile stores the Drudgers, always sorted by slot from lowest
// to highest.
func writeDrudgersFile(path string, drudgers []*drudger.Drudger) error {
	stored := drudgersFile{Drudgers: make([]storedDrudger, 0, len(drudgers))}
	for _, entry := range drudgers {
		stored.Drudgers = append(stored.Drudgers, storedDrudger{
			Slot:          entry.Slot,
			Sandbox:       entry.Sandbox,
			Task:          string(entry.TaskID),
			SandboxHealth: string(entry.SandboxHealth),
			AgentHealth:   string(entry.AgentHealth),
			LastChecked:   entry.LastChecked,
		})
	}
	slices.SortFunc(stored.Drudgers, func(first, second storedDrudger) int {
		return cmp.Compare(first.Slot, second.Slot)
	})

	return common.WriteJSON(path, stored)
}

// lockDrudgers takes an exclusive lock on the lock file and returns the
// release callback. It also waits for a lock another process holds. If the
// process dies, then lock is released by the kernel.
func lockDrudgers(path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, common.DefaultFilePerm)
	if err != nil {
		return nil, fmt.Errorf("could not open the Drudgers lock file %s: %w", path, err)
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, fmt.Errorf("could not lock %s: %w", path, err)
	}

	return func() {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}, nil
}

func (repo *FileDrudgerRepository) resolveProjectsDir() (string, error) {
	// This is basically an override for testing purposes.
	// Real projects dir is resolved from home dir lower in the code.
	if repo.projectsDirPath != "" {
		return repo.projectsDirPath, nil
	}
	home, err := common.HomeDir()
	if err != nil {
		return "", err
	}
	return common.ProjectsDir(home), nil
}

func (repo *FileDrudgerRepository) resolveProjectDir(projectSlug string) (string, error) {
	projectsDir, err := repo.resolveProjectsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(projectsDir, projectSlug), nil
}

func (repo *FileDrudgerRepository) drudgersFilePath(projectSlug string) (string, error) {
	projectDir, err := repo.resolveProjectDir(projectSlug)
	if err != nil {
		return "", err
	}
	return filepath.Join(projectDir, DrudgersFileName), nil
}
