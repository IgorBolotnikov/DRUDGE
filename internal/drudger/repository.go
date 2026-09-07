package drudger

// DrudgerRepository stores the Drudgers a project has. An entry appears when a
// Drudger is first claimed and leaves only by deliberate destructionl. The
// store usually does not grow beyond the bounds of the pool.
type DrudgerRepository interface {
	// ListDrudgers reads the Drudgers of a project.
	ListDrudgers(projectSlug string) ([]*Drudger, error)
	// UpdateDrudgers hands the project's Drudgers to change to the callback and
	// stores what it returns. The whole call holds an exclusive lock. An error
	// from change stores nothing and is returned back to the caller.
	UpdateDrudgers(projectSlug string, change func(drudgers []*Drudger) ([]*Drudger, error)) error
	// TryUpdateDrudgers works like UpdateDrudgers, but gives up when someone
	// else holds the lock. It reports whether it took the lock and ran change.
	// It is for callers that would rather do nothing than wait.
	TryUpdateDrudgers(projectSlug string, change func(drudgers []*Drudger) ([]*Drudger, error)) (bool, error)
}
