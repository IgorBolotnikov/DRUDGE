package persistence

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"drudge/internal/common"
	"drudge/internal/drudger"
	"drudge/internal/task"
)

const drudgerTestProject = "test-project"

// setupDrudgerRepo builds a repository over a temp projects directory and
// returns it with the path of the project's Drudgers file.
func setupDrudgerRepo(t *testing.T) (*FileDrudgerRepository, string) {
	t.Helper()
	projectsDir := t.TempDir()
	return NewFileDrudgerRepository(projectsDir), filepath.Join(projectsDir, drudgerTestProject, DrudgersFileName)
}

// storeDrudgers writes a pool through the repository.
func storeDrudgers(t *testing.T, repo *FileDrudgerRepository, pool []*drudger.Drudger) {
	t.Helper()
	err := repo.UpdateDrudgers(drudgerTestProject, func([]*drudger.Drudger) ([]*drudger.Drudger, error) {
		return pool, nil
	})
	if err != nil {
		t.Fatalf("UpdateDrudgers: %v", err)
	}
}

func TestFileDrudgerRepository_RoundTrip(t *testing.T) {
	lastChecked := time.Now().UTC().Truncate(time.Second)

	cases := []struct {
		name string
		pool []*drudger.Drudger
	}{
		{name: "a project with no Drudgers"},
		{
			name: "an idle Drudger",
			pool: []*drudger.Drudger{
				{Slot: 1, Sandbox: "drudge-claude-test-project-1", LastChecked: lastChecked},
			},
		},
		{
			name: "an occupied Drudger",
			pool: []*drudger.Drudger{
				{Slot: 1, Sandbox: "drudge-claude-test-project-1", TaskID: "task-1", LastChecked: lastChecked},
			},
		},
		{
			name: "a pool of several, out of order",
			pool: []*drudger.Drudger{
				{Slot: 3, Sandbox: "drudge-claude-test-project-3", TaskID: "task-3", LastChecked: lastChecked},
				{Slot: 1, Sandbox: "drudge-claude-test-project-1", LastChecked: lastChecked},
				{Slot: 2, Sandbox: "drudge-claude-test-project-2", TaskID: "task-2", LastChecked: lastChecked},
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo, _ := setupDrudgerRepo(t)
			storeDrudgers(t, repo, testCase.pool)

			read, err := repo.ListDrudgers(drudgerTestProject)
			if err != nil {
				t.Fatalf("ListDrudgers: %v", err)
			}
			if len(read) != len(testCase.pool) {
				t.Fatalf("expected %d Drudgers, got %d", len(testCase.pool), len(read))
			}

			for index, got := range read {
				if index > 0 && read[index-1].Slot > got.Slot {
					t.Errorf("expected the Drudgers to come back in slot order, got %d after %d", got.Slot, read[index-1].Slot)
				}
				want := drudgerOfSlot(testCase.pool, got.Slot)
				if want == nil {
					t.Fatalf("read back a Drudger of slot %d that was never stored", got.Slot)
				}
				if got.Sandbox != want.Sandbox {
					t.Errorf("expected sandbox %q, got %q", want.Sandbox, got.Sandbox)
				}
				if got.TaskID != want.TaskID {
					t.Errorf("expected task %q, got %q", want.TaskID, got.TaskID)
				}
				if !got.LastChecked.Equal(want.LastChecked) {
					t.Errorf("expected last checked %v, got %v", want.LastChecked, got.LastChecked)
				}
			}
		})
	}
}

func drudgerOfSlot(pool []*drudger.Drudger, slot int) *drudger.Drudger {
	for _, candidate := range pool {
		if candidate.Slot == slot {
			return candidate
		}
	}
	return nil
}

func TestFileDrudgerRepository_ListDrudgers_AbsentFileIsAnEmptyPool(t *testing.T) {
	repo, _ := setupDrudgerRepo(t)

	drudgers, err := repo.ListDrudgers(drudgerTestProject)
	if err != nil {
		t.Fatalf("expected a project without a Drudgers file to have an empty pool, got %v", err)
	}
	if len(drudgers) != 0 {
		t.Errorf("expected an empty pool, got %v", drudgers)
	}
}

func TestFileDrudgerRepository_MalformedFileNamesThePath(t *testing.T) {
	repo, path := setupDrudgerRepo(t)
	if err := common.EnsureDir(filepath.Dir(path)); err != nil {
		t.Fatalf("could not create the project directory: %v", err)
	}
	if err := common.WriteFile(path, "this is not json"); err != nil {
		t.Fatalf("could not write the Drudgers file: %v", err)
	}

	cases := []struct {
		name string
		read func() error
	}{
		{
			name: "listing",
			read: func() error {
				_, err := repo.ListDrudgers(drudgerTestProject)
				return err
			},
		},
		{
			name: "updating",
			read: func() error {
				return repo.UpdateDrudgers(drudgerTestProject, func(drudgers []*drudger.Drudger) ([]*drudger.Drudger, error) {
					return drudgers, nil
				})
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.read()
			if err == nil {
				t.Fatal("expected a malformed Drudgers file to surface as an error")
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("expected the error to name %q, got %q", path, err)
			}
		})
	}
}

func TestFileDrudgerRepository_UpdateDrudgers_ChangeErrorStoresNothing(t *testing.T) {
	repo, _ := setupDrudgerRepo(t)
	storeDrudgers(t, repo, []*drudger.Drudger{{Slot: 1, Sandbox: "drudge-claude-test-project-1"}})

	err := repo.UpdateDrudgers(drudgerTestProject, func(drudgers []*drudger.Drudger) ([]*drudger.Drudger, error) {
		drudgers[0].TaskID = "task-1"
		return drudgers, fmt.Errorf("the caller changed its mind")
	})
	if err == nil {
		t.Fatal("expected the error from the change to come back")
	}

	read, err := repo.ListDrudgers(drudgerTestProject)
	if err != nil {
		t.Fatalf("ListDrudgers: %v", err)
	}
	if len(read) != 1 || read[0].TaskID != "" {
		t.Errorf("expected the stored pool to be untouched, got %v", read[0])
	}
}

func TestFileDrudgerRepository_UpdateDrudgers_TwoClaimsCannotTakeTheSameSlot(t *testing.T) {
	const claimants = 8

	repo, _ := setupDrudgerRepo(t)

	// Every claimant takes the lowest free slot, the way allocation does. If
	// two of them read the same pool, they hand out one slot twice.
	var waiting sync.WaitGroup
	for claimant := range claimants {
		waiting.Add(1)
		go func() {
			defer waiting.Done()
			err := repo.UpdateDrudgers(drudgerTestProject, func(drudgers []*drudger.Drudger) ([]*drudger.Drudger, error) {
				return append(drudgers, &drudger.Drudger{
					Slot:   len(drudgers) + 1,
					TaskID: task.TaskID(fmt.Sprintf("task-%d", claimant)),
				}), nil
			})
			if err != nil {
				t.Errorf("UpdateDrudgers: %v", err)
			}
		}()
	}
	waiting.Wait()

	read, err := repo.ListDrudgers(drudgerTestProject)
	if err != nil {
		t.Fatalf("ListDrudgers: %v", err)
	}
	if len(read) != claimants {
		t.Fatalf("expected %d Drudgers, got %d", claimants, len(read))
	}

	taken := make(map[int]bool, len(read))
	for _, claimed := range read {
		if taken[claimed.Slot] {
			t.Errorf("slot %d was handed out twice", claimed.Slot)
		}
		taken[claimed.Slot] = true
	}
}
