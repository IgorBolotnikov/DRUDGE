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

// DetectSetup proposes setup commands and copy paths for the repositories of a
// project from the files in the root of each repository, and records them
// through the SetupRecorder. A repository that has a setup key keeps it and
// its copy paths, an empty list included, unless the options force new ones.
func (p *ProjectService) DetectSetup(projectDir string, repositories []Repository, options DetectSetupOptions) ([]SetupDetection, error) {
	if len(repositories) == 0 {
		return nil, fmt.Errorf("the local config records no %s, add them under %q", RepositoriesKey, RepositoriesKey)
	}

	detections, setups, err := p.proposeSetups(projectDir, repositories, options.IsForced)
	if err != nil {
		return nil, err
	}

	if options.IsDryRun || len(setups) == 0 {
		return detections, nil
	}
	if err := p.setups.RecordSetup(setups); err != nil {
		return nil, fmt.Errorf("could not write the detected setup into the local config: %w", err)
	}
	return detections, nil
}

// proposeSetups runs detection on every repository with no setup key, or on
// every repository when forced. It returns what it did for each repository and
// what it proposed, keyed by repository path.
func (p *ProjectService) proposeSetups(projectDir string, repositories []Repository, isForced bool) ([]SetupDetection, map[string]DetectedSetup, error) {
	detections := make([]SetupDetection, 0, len(repositories))
	setups := map[string]DetectedSetup{}
	for _, repository := range repositories {
		if repository.Setup != nil && !isForced {
			detections = append(detections, SetupDetection{Repository: repository.Path, IsKept: true})
			continue
		}

		dir := filepath.Join(projectDir, repository.Path)
		lines, err := detectSetupLines(dir)
		if err != nil {
			return nil, nil, err
		}
		detection := SetupDetection{Repository: repository.Path, Lines: lines}
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
func detectSetupLines(dir string) ([]SetupLine, error) {
	root := repositoryRoot{dir: dir}
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
