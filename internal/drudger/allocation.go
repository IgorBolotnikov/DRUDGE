package drudger

import (
	"fmt"
	"time"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/task"
)

// claimDrudger picks the Drudger a task runs on and records the claim. It
// reclaims finished Sessions first, so a pool never wedges on Drudgers whose
// agents have already finished.
//
// Everything here happens inside the repository lock. Creating the sandbox and
// launching the agent happen after it, so a slow sandbox never blocks another
// allocation.
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

// releaseDrudger frees a claim whose run never got off the ground, so a slot
// does not leak because a launch went wrong. It only frees a Drudger still
// holding this task, so it can never take a Drudger off work someone else
// handed it.
//
// A failure to release is reported and swallowed, because the caller is
// already returning the error that made the release necessary.
func (service *DrudgerService) releaseDrudger(projectSlug string, slot int, taskID task.TaskID) {
	err := service.drudgers.UpdateDrudgers(projectSlug, func(drudgers []*Drudger) ([]*Drudger, error) {
		for _, candidate := range drudgers {
			if candidate.Slot == slot && candidate.TaskID == taskID {
				candidate.TaskID = ""
				candidate.LastChecked = time.Now().UTC()
			}
		}
		return drudgers, nil
	})
	if err != nil {
		service.logger.Error("Drudger %d of project %s stays claimed for a run that never started: %v", slot, projectSlug, err)
	}
}

// pickDrudger reclaims finished Sessions and hands the task the lowest free
// slot under the configured limit. A slot with no Drudger yet gets one, named
// for the sandbox that is about to be created, so the pool comes back grown
// alongside the Drudger that was chosen.
func (service *DrudgerService) pickDrudger(drudgers []*Drudger, projectSlug string, taskID task.TaskID, workspace string) (chosen *Drudger, pool []*Drudger, err error) {
	now := time.Now().UTC()

	if err := reclaimFinished(drudgers, workspace, now); err != nil {
		return nil, nil, err
	}

	limit := config.ResolveMaxConcurrentDrudgers(service.localCfg, service.globalCfg)
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

	return nil, nil, fmt.Errorf("all %d Drudgers of project %s are busy, wait for one to finish or raise %s in the config", limit, projectSlug, config.MaxConcurrentDrudgersKey)
}

// reclaimFinished frees every Drudger whose Session has finished. A Session is
// finished once its run directory holds an exit file, which the launcher
// writes last. The pool never exceeds a dozen, so this is a handful of checks.
func reclaimFinished(drudgers []*Drudger, workspace string, now time.Time) error {
	for _, candidate := range drudgers {
		if candidate.Idle() {
			continue
		}

		runDir := common.RunDir(workspace, string(candidate.TaskID))
		finished, err := common.Exists(common.RunExitPath(runDir))
		if err != nil {
			return fmt.Errorf("could not tell whether the Session of task %s has finished: %w", candidate.TaskID, err)
		}
		if !finished {
			continue
		}

		candidate.TaskID = ""
		candidate.LastChecked = now
	}
	return nil
}
