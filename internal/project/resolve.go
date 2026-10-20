package project

import (
	"errors"
	"fmt"
	"slices"
)

// initCommand is what a user runs to create a project.
const initCommand = "drg project init"

// errNoProjectHere refuses a command that names no project outside a project
// directory.
var errNoProjectHere = errors.New("no project in the current directory, pass -p <slug> or run from a project directory")

// ResolveProject finds the project a command works on. A non-empty
// explicitSlug wins, also inside a project directory. Without it the current
// directory names the project. It refuses a slug no project record carries.
func (p *ProjectService) ResolveProject(explicitSlug string) (*Project, error) {
	slug := explicitSlug
	if slug == "" {
		linkedSlug, isLinked, err := p.linker.LinkedSlug()
		if err != nil {
			return nil, err
		}
		if !isLinked {
			return nil, errNoProjectHere
		}
		slug = linkedSlug
	}

	projects, err := p.repo.ListProjects()
	if err != nil {
		return nil, err
	}
	index := slices.IndexFunc(projects, func(candidate *Project) bool { return candidate.Slug == slug })
	if index < 0 {
		return nil, fmt.Errorf("project %q does not exist, run %s <name> in its directory to create it", slug, initCommand)
	}
	return projects[index], nil
}
