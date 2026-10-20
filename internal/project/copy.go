package project

import (
	"fmt"
	"strings"
)

// Names in a repository root detection proposes to copy. A direnv file runs
// on the host and is left out.
const (
	envFilePrefix = ".env"
	envsDir       = ".envs"
	envrcFile     = ".envrc"
)

// detectCopyPaths proposes the local environment files in the root of a
// repository that git ignores: files whose name starts with .env and a .envs
// directory. A repository with none gets an empty list.
func (p *ProjectService) detectCopyPaths(dir string) ([]string, error) {
	entries, err := p.files.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", dir, err)
	}

	candidates := []string{}
	for _, entry := range entries {
		name := entry.Name()
		isEnvFile := entry.Type().IsRegular() && strings.HasPrefix(name, envFilePrefix) && name != envrcFile
		isEnvsDir := entry.IsDir() && name == envsDir
		if isEnvFile || isEnvsDir {
			candidates = append(candidates, name)
		}
	}
	if len(candidates) == 0 {
		return candidates, nil
	}

	ignored, err := p.gitOps.IgnoredPaths(dir, candidates)
	if err != nil {
		return nil, err
	}
	if ignored == nil {
		return []string{}, nil
	}
	return ignored, nil
}
