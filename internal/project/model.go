// Package project provides a project
package project

import "time"

type Project struct {
	Slug     string
	Name     string
	Location string

	CreatedAt time.Time
}

// Repository is one git repository of a project. `drg project init` writes the
// list and a user may edit it afterwards.
type Repository struct {
	// Path is where the repository sits relative to the project directory. A
	// project directory that is itself a repository records ".".
	Path string `json:"path"`
	// DefaultBranch is the branch work is cut from. An empty value means
	// drudge reads it from origin/HEAD.
	DefaultBranch string `json:"defaultBranch,omitempty"`
}
