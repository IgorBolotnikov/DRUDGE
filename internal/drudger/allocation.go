package drudger

import (
	"cmp"
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

// recordHealth stores what drudge just saw of a Drudger's sandbox and records
// when it looked.
func (service *DrudgerService) recordHealth(projectSlug string, slot int, health Health) {
	now := time.Now().UTC()

	err := service.drudgers.UpdateDrudgers(projectSlug, func(drudgers []*Drudger) ([]*Drudger, error) {
		for _, candidate := range drudgers {
			if candidate.Slot == slot {
				candidate.Health = health
				candidate.LastChecked = now
			}
		}
		return drudgers, nil
	})
	if err != nil {
		service.logger.Error("Drudger %d of project %s is %s, but that could not be recorded: %v", slot, projectSlug, health, err)
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

	return nil, nil, fmt.Errorf("all %d Drudgers of project %s are busy, wait for one to finish or raise %s in the config", limit, projectSlug, config.MaxConcurrentDrudgersKey)
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

// reclaimFinished frees every Drudger whose Session has finished. A Session is
// finished once its run directory holds an exit file.
func reclaimFinished(drudgers []*Drudger, workspace string, now time.Time) error {
	for _, candidate := range drudgers {
		if candidate.Idle() {
			continue
		}

		finished, err := sessionFinished(common.RunDir(workspace, string(candidate.TaskID)))
		if err != nil {
			return err
		}
		if !finished {
			continue
		}

		candidate.TaskID = ""
		candidate.LastChecked = now
	}
	return nil
}
