package drudger

// DrudgerRepository stores the Drudgers a project has. An entry appears when a
// Drudger is first claimed and leaves only by deliberate destruction, so the
// store stays bounded by the size of the pool.
type DrudgerRepository interface {
	// ListDrudgers reads the Drudgers of a project. A project that has never
	// run a task has none.
	ListDrudgers(projectSlug string) ([]*Drudger, error)
	// UpdateDrudgers hands the project's Drudgers to change and stores what it
	// returns. The whole call holds an exclusive lock, so the caller gets a
	// read-modify-write transaction and never has to think about locking. An
	// error from change stores nothing and comes back to the caller.
	UpdateDrudgers(projectSlug string, change func(drudgers []*Drudger) ([]*Drudger, error)) error
}
