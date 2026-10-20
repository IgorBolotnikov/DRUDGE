package project

import (
	"fmt"
	"path/filepath"
)

// SetupLine is one setup command detection proposes, with the file in the
// repository root that triggered it.
type SetupLine struct {
	Command string
	Trigger string
}

// SetupDetection is what detection did for one repository. A kept repository
// already had a setup key and its setup and copy paths stay. Otherwise Lines
// is the setup proposed for it, and empty when no rule matched.
type SetupDetection struct {
	Repository string
	Lines      []SetupLine
	IsKept     bool
	CopyPaths  []string
	// IsCopyKept means the repository had copy paths and they stay.
	IsCopyKept bool
	// EnvNames are the names the repository adds to the Drudger env, kept
	// repositories included.
	EnvNames []EnvName
}

// DetectSetupOptions says which setups detection replaces and whether it
// writes them.
type DetectSetupOptions struct {
	// IsForced replaces the setup and the copy paths of every repository.
	// Otherwise only a repository with no setup key gets a setup, and copy
	// paths when it has no copy key.
	IsForced bool
	// IsDryRun proposes setups and writes nothing.
	IsDryRun bool
}

// DetectSetup reads the repositories and the Drudger env of a project through
// the SetupRecorder, proposes setup commands and copy paths for each
// repository from the files in its root, and records them through the
// SetupRecorder. A repository that has a setup key keeps it and its copy
// paths, an empty list included, unless the options force new ones. Every
// environment variable name a repository needs that the Drudger env does not
// list is recorded too. A name is never removed from the Drudger env.
func (p *ProjectService) DetectSetup(projectDir string, options DetectSetupOptions) ([]SetupDetection, error) {
	repositories, envNames, err := p.setups.RecordedSetup()
	if err != nil {
		return nil, err
	}
	if len(repositories) == 0 {
		return nil, fmt.Errorf("the local config records no %s, add them under %q", RepositoriesKey, RepositoriesKey)
	}

	detections, setups, err := p.proposeSetups(projectDir, repositories, envNames, options.IsForced)
	if err != nil {
		return nil, err
	}

	added := addedEnvNames(detections)
	if options.IsDryRun || (len(setups) == 0 && len(added) == 0) {
		return detections, nil
	}
	if err := p.setups.RecordSetup(setups, added); err != nil {
		return nil, fmt.Errorf("could not write the detected setup into the local config: %w", err)
	}
	return detections, nil
}

// proposeSetups runs setup detection on every repository with no setup key,
// or on every repository when forced, and environment detection on every
// repository. It returns what it did for each repository and the setup it
// proposed, keyed by repository path. A name in envNames is not added again.
func (p *ProjectService) proposeSetups(projectDir string, repositories []Repository, envNames []string, isForced bool) ([]SetupDetection, map[string]DetectedSetup, error) {
	listed := map[string]bool{}
	for _, name := range envNames {
		listed[name] = true
	}

	detections := make([]SetupDetection, 0, len(repositories))
	setups := map[string]DetectedSetup{}
	for _, repository := range repositories {
		dir := filepath.Join(projectDir, repository.Path)
		added, err := p.detectEnvNames(dir, listed)
		if err != nil {
			return nil, nil, err
		}
		if repository.Setup != nil && !isForced {
			detections = append(detections, SetupDetection{Repository: repository.Path, IsKept: true, EnvNames: added})
			continue
		}

		lines, err := detectSetupLines(repositoryRoot{files: p.files, dir: dir})
		if err != nil {
			return nil, nil, err
		}
		detection := SetupDetection{Repository: repository.Path, Lines: lines, EnvNames: added}
		detected := DetectedSetup{Setup: setupCommands(lines)}
		if repository.Copy != nil && !isForced {
			detection.IsCopyKept = true
		} else {
			detected.Copy, err = p.detectCopyPaths(dir)
			if err != nil {
				return nil, nil, err
			}
			detection.CopyPaths = detected.Copy
		}
		detections = append(detections, detection)
		setups[repository.Path] = detected
	}
	return detections, setups, nil
}

// detectSetupLines walks the setup rules over the root of one repository.
func detectSetupLines(root repositoryRoot) ([]SetupLine, error) {
	lines := []SetupLine{}
	for _, rule := range setupRules {
		proposed, err := rule(root)
		if err != nil {
			return nil, err
		}
		lines = append(lines, proposed...)
	}
	return lines, nil
}

// setupCommands returns the commands of setup lines. No lines make an empty
// list, which a saved config keeps.
func setupCommands(lines []SetupLine) []string {
	commands := make([]string, 0, len(lines))
	for _, line := range lines {
		commands = append(commands, line.Command)
	}
	return commands
}
