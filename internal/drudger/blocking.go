package drudger

import (
	"fmt"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/git"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// unfinishedBlockerLine lays out one blocker in a refusal, the status padded
// to the longest status there is.
const unfinishedBlockerLine = "  %s  %-11s  %s"

// missingBlockerLine lays out a blocker id that names no stored task.
const missingBlockerLine = "  %s  no such task"

// unmergedBlockerLine lays out a blocker with unmerged work in a refusal, and
// the lines of FormatUnmergedWork follow it.
const unmergedBlockerLine = "  %s  %s"

// unmergedWorkIndent sets the lines of unmerged work under their blocker.
const unmergedWorkIndent = "    "

// UnmergedWork is work of a done blocker in one repository that has not
// reached the base the repository cuts work from.
type UnmergedWork struct {
	Repository string
	Branch     string
	BaseRef    string
	// Commits is how many commits the work holds that BaseRef does not.
	Commits int
}

// refuseBlocked refuses a task with a blocker that is not done or whose id
// names no stored task. The refusal lists every such blocker. When every
// blocker is done, it refuses a task with a blocker whose work has not reached
// its base, and lists that work.
func (service *DrudgerService) refuseBlocked(projectSlug string, layout projectLayout, dependent *task.Task) error {
	blockers, err := service.tasks.Blockers(projectSlug, dependent)
	if err != nil {
		return err
	}

	var lines []string
	for _, blocker := range blockers {
		switch {
		case blocker.Task == nil:
			lines = append(lines, fmt.Sprintf(missingBlockerLine, task.ShortID(blocker.ID)))
		case blocker.Task.Status != task.StatusDone:
			lines = append(lines, fmt.Sprintf(unfinishedBlockerLine, task.ShortID(blocker.ID), blocker.Task.Status, blocker.Task.Title))
		}
	}
	if len(lines) > 0 {
		return fmt.Errorf("task %s is blocked by:\n%s", task.ShortID(dependent.ID), strings.Join(lines, "\n"))
	}

	unmerged, err := service.unmergedWork(layout, blockers)
	if err != nil {
		return err
	}
	for _, blocker := range blockers {
		work := unmerged[blocker.ID]
		if len(work) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf(unmergedBlockerLine, task.ShortID(blocker.ID), blocker.Task.Title))
		for _, line := range FormatUnmergedWork(work) {
			lines = append(lines, unmergedWorkIndent+line)
		}
	}
	if len(lines) > 0 {
		return fmt.Errorf("task %s cannot run, work it depends on is not merged:\n%s", task.ShortID(dependent.ID), strings.Join(lines, "\n"))
	}
	return nil
}

// UnmergedWork returns the work of each done blocker that has not reached the
// base its repository cuts work from, keyed by the blocker's id. A blocker
// with all of its work merged has no entry. The base of every repository
// holding such work is fetched first.
func (service *DrudgerService) UnmergedWork(blockers []task.Blocker) (map[task.TaskID][]UnmergedWork, error) {
	layout, err := service.layout()
	if err != nil {
		return nil, err
	}
	return service.unmergedWork(layout, blockers)
}

func (service *DrudgerService) unmergedWork(layout projectLayout, blockers []task.Blocker) (map[task.TaskID][]UnmergedWork, error) {
	var landed []*task.Task
	for _, blocker := range blockers {
		if blocker.Task != nil && blocker.Task.Status == task.StatusDone && len(blocker.Task.Landings) > 0 {
			landed = append(landed, blocker.Task)
		}
	}
	if len(landed) == 0 {
		return nil, nil
	}

	// resolveRepositories reads git and refuses a project with no
	// repositories, so it runs only when there is landed work to check.
	repositories, err := service.resolveRepositories(layout)
	if err != nil {
		return nil, err
	}

	unmerged := map[task.TaskID][]UnmergedWork{}
	for _, repository := range repositories {
		if !hasLandingIn(landed, repository.Name) {
			continue
		}
		if !service.tryFetchBase(repository) {
			service.logger.Error("The work of blockers in repository %s is checked against %s as it is on disk", repository.Name, repository.BaseRef())
		}

		for _, blocker := range landed {
			landing, ok := blocker.Landings[repository.Name]
			if !ok {
				continue
			}
			commits, err := service.gitOps.CommitCount(repository.Dir, repository.BaseRef(), landingTip(landing))
			if err != nil {
				return nil, fmt.Errorf("could not tell whether the work of task %s in repository %s is merged: %w", blocker.ID, repository.Name, err)
			}
			if commits > 0 {
				unmerged[blocker.ID] = append(unmerged[blocker.ID], UnmergedWork{
					Repository: repository.Name,
					Branch:     landing.Branch,
					BaseRef:    repository.BaseRef(),
					Commits:    commits,
				})
			}
		}
	}
	return unmerged, nil
}

// landingTip is the commit a landing's work ends at. A landing close-out has
// not filled in yet only knows its branch.
func landingTip(landing task.Landing) string {
	if landing.Head == "" {
		return landing.Branch
	}
	return landing.Head
}

func hasLandingIn(tasks []*task.Task, repository string) bool {
	for _, landed := range tasks {
		if _, ok := landed.Landings[repository]; ok {
			return true
		}
	}
	return false
}

// FormatUnmergedWork lays out the unmerged work of one blocker, one line per
// repository, with the repository names padded to the widest of them.
func FormatUnmergedWork(work []UnmergedWork) []string {
	width := 0
	for _, entry := range work {
		width = max(width, len(entry.Repository))
	}

	lines := make([]string, 0, len(work))
	for _, entry := range work {
		lines = append(lines, fmt.Sprintf("%-*s  %s  %s not in %s", width, entry.Repository, entry.Branch, git.FormatCommitCount(entry.Commits), entry.BaseRef))
	}
	return lines
}
