package persistence

import (
	"cmp"
	"errors"
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

// What an update does when someone else holds the Drudgers lock.
const (
	waitForLock  = true
	giveUpOnLock = false
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
// lock and writes back what it returns. It waits for a lock someone else
// holds.
func (repo *FileDrudgerRepository) UpdateDrudgers(projectSlug string, change func([]*drudger.Drudger) ([]*drudger.Drudger, error)) error {
	_, err := repo.updateDrudgers(projectSlug, change, waitForLock)
	return err
}

// TryUpdateDrudgers runs change the way UpdateDrudgers does, but gives up
// when someone else holds the lock. It reports whether it took the lock.
func (repo *FileDrudgerRepository) TryUpdateDrudgers(projectSlug string, change func([]*drudger.Drudger) ([]*drudger.Drudger, error)) (bool, error) {
	return repo.updateDrudgers(projectSlug, change, giveUpOnLock)
}

// updateDrudgers reads the project's Drudgers under the lock, hands them to
// change and writes back what it returns. It reports whether it took the lock,
// which is always true when it was told to wait for one.
func (repo *FileDrudgerRepository) updateDrudgers(projectSlug string, change func([]*drudger.Drudger) ([]*drudger.Drudger, error), wait bool) (bool, error) {
	projectDir, err := repo.resolveProjectDir(projectSlug)
	if err != nil {
		return false, err
	}
	if err := common.EnsureDir(projectDir); err != nil {
		return false, err
	}

	unlock, locked, err := lockDrudgers(filepath.Join(projectDir, drudgersLockFileName), wait)
	if err != nil {
		return false, err
	}
	if !locked {
		return false, nil
	}
	defer unlock()

	path := filepath.Join(projectDir, DrudgersFileName)

	drudgers, err := readDrudgersFile(path)
	if err != nil {
		return false, err
	}

	updated, err := change(drudgers)
	if err != nil {
		return false, err
	}

	if err := writeDrudgersFile(path, updated); err != nil {
		return false, err
	}
	return true, nil
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
// release callback. If the process dies, then lock is released by the kernel.
//
// With waitForLock it waits for a lock someone else holds and always comes
// back with it. With giveUpOnLock it comes back at once, and locked says
// whether it got the lock. A caller that did not get the lock gets a nil
// release callback.
func lockDrudgers(path string, wait bool) (unlock func(), locked bool, err error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, common.DefaultFilePerm)
	if err != nil {
		return nil, false, fmt.Errorf("could not open the Drudgers lock file %s: %w", path, err)
	}

	lockMode := syscall.LOCK_EX
	if !wait {
		lockMode |= syscall.LOCK_NB
	}

	if err := syscall.Flock(int(file.Fd()), lockMode); err != nil {
		file.Close()
		if !wait && errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("could not lock %s: %w", path, err)
	}

	return func() {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}, true, nil
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
