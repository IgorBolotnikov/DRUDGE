package project

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// EnvName is an environment variable name detection adds to the Drudger env,
// with the file in the repository root that names it.
type EnvName struct {
	Name    string
	Trigger string
}

// npmrcFile is the npm config file whose ${NAME} references detection reads.
const npmrcFile = ".npmrc"

// npmrcCommentPrefixes start the comment lines of an npm config file.
var npmrcCommentPrefixes = []string{"#", ";"}

// npmrcEnvReference matches a ${NAME} reference npm fills from the
// environment.
var npmrcEnvReference = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// detectEnvNames returns every name the npm config file in the root of a
// repository references and listed does not hold yet, and adds each one to
// listed. A repository with no npm config file gets none.
func detectEnvNames(dir string, listed map[string]bool) ([]EnvName, error) {
	path := filepath.Join(dir, npmrcFile)
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", path, err)
	}
	defer file.Close()

	var added []EnvName
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if slices.ContainsFunc(npmrcCommentPrefixes, func(prefix string) bool { return strings.HasPrefix(line, prefix) }) {
			continue
		}
		for _, match := range npmrcEnvReference.FindAllStringSubmatch(line, -1) {
			name := match[1]
			if listed[name] {
				continue
			}
			listed[name] = true
			added = append(added, EnvName{Name: name, Trigger: npmrcFile})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("could not read %s: %w", path, err)
	}
	return added, nil
}

// addedEnvNames returns the names detection added across every repository, in
// the order it found them.
func addedEnvNames(detections []SetupDetection) []string {
	var names []string
	for _, detection := range detections {
		for _, envName := range detection.EnvNames {
			names = append(names, envName.Name)
		}
	}
	return names
}
