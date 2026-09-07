package drudger

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/task"
)

func (service *DrudgerService) claimDrudger(projectSlug string, taskID task.TaskID, workspace string) (*Drudger, error) {
	var claimed *Drudger

	err := service.drudgers.UpdateDrudgers(projectSlug, func(drudgers []*Drudger) ([]*Drudger, error) {
		chosen, updated, err := service.pickDrudger(drudgers, projectSlug, taskID, workspace)
		if err != nil {
			return nil, err
		}
		claimed = chosen
		return updated, nil
	})
	if err != nil {
		return nil, err
	}

	return claimed, nil
}

// previewDrudger works out which Drudger a task would run on without
// recording anything. It is what a dry run reports.
func (service *DrudgerService) previewDrudger(projectSlug string, taskID task.TaskID, workspace string) (*Drudger, error) {
	drudgers, err := service.drudgers.ListDrudgers(projectSlug)
	if err != nil {
		return nil, err
	}

	chosen, _, err := service.pickDrudger(drudgers, projectSlug, taskID, workspace)
	if err != nil {
		return nil, err
	}
	return chosen, nil
}

func (service *DrudgerService) releaseDrudger(projectSlug string, slot int, taskID task.TaskID) error {
	return service.drudgers.UpdateDrudgers(projectSlug, func(drudgers []*Drudger) ([]*Drudger, error) {
		for _, candidate := range drudgers {
			if candidate.Slot == slot && candidate.TaskID == taskID {
				candidate.TaskID = ""
				candidate.LastChecked = time.Now().UTC()
			}
		}
		return drudgers, nil
	})
}

// recordSandboxHealth stores what drudge just saw of a Drudger's sandbox and
// records when it looked.
func (service *DrudgerService) recordSandboxHealth(projectSlug string, slot int, health SandboxHealth) {
	now := time.Now().UTC()

	err := service.drudgers.UpdateDrudgers(projectSlug, func(drudgers []*Drudger) ([]*Drudger, error) {
		for _, candidate := range drudgers {
			if candidate.Slot == slot {
				candidate.SandboxHealth = health
				candidate.LastChecked = now
			}
		}
		return drudgers, nil
	})
	if err != nil {
		service.logger.Error("The sandbox of Drudger %d of project %s is %s, but that could not be recorded: %v", slot, projectSlug, health, err)
	}
}

// recordAgentHealth stores what a finished Session just said about the agent
// that ran it and records when drudge looked.
//
// The Drudger holding the task is the one that ran it. Once that slot is freed
// the link between the two is gone, so there is nothing left to record
// against and the observation is dropped.
func (service *DrudgerService) recordAgentHealth(projectSlug string, taskID task.TaskID, health AgentHealth) {
	now := time.Now().UTC()

	err := service.drudgers.UpdateDrudgers(projectSlug, func(drudgers []*Drudger) ([]*Drudger, error) {
		holder := drudgerHoldingTask(drudgers, taskID)
		if holder == nil {
			return drudgers, nil
		}
		holder.AgentHealth = health
		holder.LastChecked = now
		return drudgers, nil
	})
	if err != nil {
		service.logger.Error("The agent that ran task %s of project %s is %s, but that could not be recorded: %v", taskID, projectSlug, health, err)
	}
}

// pickDrudger reclaims finished Sessions and hands the task the lowest free
// slot under the configured limit. If a free slot has no Drydger yet, then it
// creates a new one and hands it back.
func (service *DrudgerService) pickDrudger(drudgers []*Drudger, projectSlug string, taskID task.TaskID, workspace string) (chosen *Drudger, pool []*Drudger, err error) {
	now := time.Now().UTC()

	if err := reclaimFinished(drudgers, workspace, now); err != nil {
		return nil, nil, err
	}

	limit := config.ResolveMaxConcurrentDrudgers(service.localCfg, service.globalCfg)
	service.warnAboveLimit(drudgers, projectSlug, limit)

	bySlot := make(map[int]*Drudger, len(drudgers))
	for _, candidate := range drudgers {
		bySlot[candidate.Slot] = candidate
	}

	for slot := 1; slot <= limit; slot++ {
		existing, known := bySlot[slot]
		if !known {
			created := &Drudger{
				Slot:        slot,
				Sandbox:     formatDrudgerName(projectSlug, slot, service.globalCfg.Drudger.Harness),
				TaskID:      taskID,
				LastChecked: now,
			}
			return created, append(drudgers, created), nil
		}
		if existing.Idle() {
			existing.TaskID = taskID
			existing.LastChecked = now
			return existing, drudgers, nil
		}
	}

	return nil, nil, fmt.Errorf("all %d Drudgers of project %s are busy, wait for one to finish, run %s to free the slots whose agent is gone, or raise %s in the config", limit, projectSlug, reclaimCommand, config.MaxConcurrentDrudgersKey)
}

// warnAboveLimit names the Drudgers whose slot is above the configured limit.
func (service *DrudgerService) warnAboveLimit(drudgers []*Drudger, projectSlug string, limit int) {
	above := make([]*Drudger, 0, len(drudgers))
	for _, candidate := range drudgers {
		if candidate.Slot > limit {
			above = append(above, candidate)
		}
	}
	if len(above) == 0 {
		return
	}
	slices.SortFunc(above, func(first, second *Drudger) int {
		return cmp.Compare(first.Slot, second.Slot)
	})

	names := make([]string, 0, len(above))
	for _, candidate := range above {
		names = append(names, fmt.Sprintf("slot %d (%s)", candidate.Slot, candidate.Sandbox))
	}

	service.logger.Info("Project %s has Drudgers above the %s limit of %d: %s", projectSlug, config.MaxConcurrentDrudgersKey, limit, strings.Join(names, ", "))
	service.logger.Info("They are left alone and the task was not assigned to them. Raise %s to put them back to work, or nuke them if you are done with them.", config.MaxConcurrentDrudgersKey)
}

// reclaimFinished frees every Drudger whose Session has finished, and records
// what that Session said about the agent that ran it.
func reclaimFinished(drudgers []*Drudger, workspace string, now time.Time) error {
	for _, candidate := range drudgers {
		if candidate.Idle() {
			continue
		}

		report, err := readSessionReport(common.RunDir(workspace, string(candidate.TaskID)), now)
		if errors.Is(err, errNoRunDirectory) {
			continue
		}
		if err != nil {
			return err
		}
		if !report.Finished() {
			continue
		}

		candidate.TaskID = ""
		candidate.AgentHealth = agentHealthOf(report.Status)
		candidate.LastChecked = now
	}
	return nil
}

// reclaimForListing frees the Drudgers whose Session has ended, so a list
// reports the pool as it stands. It returns the pool it read, reclaimed as far
// as it could be.
//
// It reads run directories and runs no commands, which keeps a list fast.
// Slots stuck for any other reason are freed by ReclaimDrudgers.
func (service *DrudgerService) reclaimForListing(projectSlug string, workspace string) ([]*Drudger, error) {
	asRead, err := service.drudgers.ListDrudgers(projectSlug)
	if err != nil {
		return nil, err
	}
	// Nothing is claimed, so there is nothing to free and no reason to write.
	if !anyClaimed(asRead) {
		return asRead, nil
	}

	var reclaimed []*Drudger
	stored, err := service.drudgers.TryUpdateDrudgers(projectSlug, func(drudgers []*Drudger) ([]*Drudger, error) {
		if err := reclaimFinished(drudgers, workspace, time.Now().UTC()); err != nil {
			return nil, err
		}
		reclaimed = drudgers
		return drudgers, nil
	})
	if err != nil {
		return nil, err
	}
	if !stored {
		service.logger.Info("Another drudge command holds the Drudgers of project %s, so this list is what was last written and may be behind", projectSlug)
		return asRead, nil
	}
	return reclaimed, nil
}

// FreedSlot is one Drudger whose claim ReclaimDrudgers cleared, and the
// condition it was cleared on.
type FreedSlot struct {
	Slot    int
	Sandbox string
	TaskID  task.TaskID
	Reason  string
}

// Conditions that mean a claimed slot has no agent in it.
const (
	stuckNoRunDirectory    = "its launch never made a run directory"
	stuckSandboxNotRunning = "its sandbox is not running"
)

// ReclaimDrudgers clears the claim of every Drudger of a project whose agent
// is gone, and returns the slots it cleared.
//
// A slot is claimed by a launch and freed once its run directory holds an exit
// file. The last line of the launcher script writes that file, so an agent
// killed before that line leaves the slot claimed. A claimed slot counts
// against the concurrency limit.
//
// This only rewrites the Drudgers file. It kills no process, removes no
// sandbox, writes nothing in a run directory and leaves the task record as it
// is.
func (service *DrudgerService) ReclaimDrudgers(projectSlug string) ([]FreedSlot, error) {
	workspace, err := common.WorkDir()
	if err != nil {
		return nil, fmt.Errorf("could not work out where the Drudgers of project %s run: %w", projectSlug, err)
	}

	running, err := service.observeSandboxes(projectSlug)
	if err != nil {
		return nil, err
	}

	var freed []FreedSlot
	err = service.drudgers.UpdateDrudgers(projectSlug, func(drudgers []*Drudger) ([]*Drudger, error) {
		now := time.Now().UTC()
		if err := reclaimFinished(drudgers, workspace, now); err != nil {
			return nil, err
		}
		stuck, err := freeStuckSlots(drudgers, workspace, running, now)
		if err != nil {
			return nil, err
		}
		freed = stuck
		return drudgers, nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not reclaim the Drudgers of project %s: %w", projectSlug, err)
	}
	return freed, nil
}

// freeStuckSlots clears the claim of every Drudger that stuckClaimReason
// finds without an agent, and returns what it cleared.
func freeStuckSlots(drudgers []*Drudger, workspace string, running map[string]bool, now time.Time) ([]FreedSlot, error) {
	var freed []FreedSlot

	for _, candidate := range drudgers {
		if candidate.Idle() {
			continue
		}

		reason, err := stuckClaimReason(candidate, workspace, running, now)
		if err != nil {
			return nil, err
		}
		if reason == "" {
			continue
		}

		freed = append(freed, FreedSlot{
			Slot:    candidate.Slot,
			Sandbox: candidate.Sandbox,
			TaskID:  candidate.TaskID,
			Reason:  reason,
		})
		candidate.TaskID = ""
		candidate.LastChecked = now
	}
	return freed, nil
}

// stuckClaimReason returns the condition that proves a claimed Drudger has no
// agent in it, and an empty string when it has one. Callers run
// reclaimFinished first, so a Session with an exit file never reaches here.
//
// A stale stream is no proof of a dead agent. One tool call can run for
// minutes, which is what needs babysitting reports. The sandbox status is the
// proof, because a sandbox that is not running holds no process.
func stuckClaimReason(claimed *Drudger, workspace string, running map[string]bool, now time.Time) (string, error) {
	runDir := common.RunDir(workspace, string(claimed.TaskID))

	present, err := common.Exists(runDir)
	if err != nil {
		return "", err
	}
	if !present {
		// A launch creates the run directory right after it claims the slot,
		// so a claim older than the grace period has no launch behind it.
		if now.Sub(claimed.LastChecked) <= launchGracePeriod {
			return "", nil
		}
		return stuckNoRunDirectory, nil
	}

	// An empty stream means the launch has not started the agent yet and may
	// still be creating the sandbox, which takes minutes on a first run. The
	// sandbox status only proves a dead agent once the agent has written
	// something.
	written, err := streamHasContent(runDir)
	if err != nil {
		return "", err
	}
	if !written || running[claimed.Sandbox] {
		return "", nil
	}
	return stuckSandboxNotRunning, nil
}

// anyClaimed reports whether any Drudger of a pool is occupied by a task.
func anyClaimed(drudgers []*Drudger) bool {
	return slices.ContainsFunc(drudgers, func(candidate *Drudger) bool {
		return !candidate.Idle()
	})
}

// drudgerHoldingTask returns the Drudger a task occupies, or nil when no
// Drudger holds it.
func drudgerHoldingTask(drudgers []*Drudger, taskID task.TaskID) *Drudger {
	for _, candidate := range drudgers {
		if candidate.TaskID == taskID {
			return candidate
		}
	}
	return nil
}
