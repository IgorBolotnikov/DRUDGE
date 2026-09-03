// Package common contains common functions
package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	GloablConfigName = "config.json"
	LocalConfigName  = "config.json"
	DotDrudgeDirName = ".drudge"
	ProjectsDirName  = "projects"
	PromptsDirName   = "prompts"
	SchemaDirName    = "schema"
	DefaultFilePerm  = 0o644
	ThemeConfigName  = "theme.json"
)

// EnsureDir creates dir (and any parents) if it doesn't already exist.
func EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("could not create directory %s: %w", path, err)
	}
	return nil
}

// Exists reports whether a path exists (file or dir).
func Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("could not check %s: %w", path, err)
}

// WriteJSON marshals v as indented JSON and writes it to path.
func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal json for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, DefaultFilePerm); err != nil {
		return fmt.Errorf("could not write %s: %w", path, err)
	}
	return nil
}

// WriteJSONIfNotExists writes v as JSON to path only if path doesn't already
// exist. Returns whether it wrote the file.
func WriteJSONIfNotExists(path string, v any) (bool, error) {
	exists, err := Exists(path)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	if err := WriteJSON(path, v); err != nil {
		return false, err
	}
	return true, nil
}

// ReadJSON reads path and unmarshals it into v.
func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("could not parse %s: %w", path, err)
	}
	return nil
}

// RemoveAll deletes path and everything under it. No error if it doesn't exist.
func RemoveAll(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("could not remove %s: %w", path, err)
	}
	return nil
}

// FormatFrontMatter serializes metadata as a `---` delimited YAML-like block.
// Keys are sorted alphabetically for reproducibility.
func FormatFrontMatter(metadata map[string]string) string {
	var buf strings.Builder
	buf.WriteString("---\n")
	keys := make([]string, 0, len(metadata))
	for k := range metadata {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&buf, "%s: %s\n", k, metadata[k])
	}
	buf.WriteString("---\n")
	return buf.String()
}

// ParseFrontMatter reads the front-matter block from the beginning of data
// and returns the key-value pairs. Expects `---` delimited format.
func ParseFrontMatter(data string) (map[string]string, string) {
	result := make(map[string]string)

	// Find first ---
	_, after, ok := strings.Cut(data, "---")
	if !ok {
		return result, data
	}

	rest := strings.TrimSpace(after)
	before, after, ok := strings.Cut(rest, "---")
	if !ok {
		return result, data
	}

	metaBlock := before
	content := strings.TrimSpace(after)
	separator := ": "

	for line := range strings.SplitSeq(metaBlock, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if before, after, ok := strings.Cut(line, separator); ok {
			key := before

			val := after
			result[key] = val
		}
	}

	return result, content
}

// WriteFileWithFrontMatter writes metadata as front-matter followed by raw content.
func WriteFileWithFrontMatter(path string, metadata map[string]string, content string) error {
	data := FormatFrontMatter(metadata) + content
	return os.WriteFile(path, []byte(data), DefaultFilePerm)
}

// HomeDir returns the current user's home directory.
func HomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return home, nil
}

// SlugFrom returns a URL-safe slug from a given name.
func SlugFrom(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}

// DrudgeDir returns the path to the user's .drudge home directory.
func DrudgeDir(home string) string {
	return filepath.Join(home, DotDrudgeDirName)
}

// ProjectsDir returns the path to the user's drudge projects directory.
func ProjectsDir(home string) string {
	return filepath.Join(DrudgeDir(home), ProjectsDirName)
}

// LocalPromptsDir returns the path to the prompts directory of the local
// drudge dir.
func LocalPromptsDir() string {
	return filepath.Join(DotDrudgeDirName, PromptsDirName)
}

// PromptsDir returns the path to the prompts directory of the user's drudge
// home directory.
func PromptsDir(home string) string {
	return filepath.Join(DrudgeDir(home), PromptsDirName)
}

// LocalConfigPath returns the path to the local drudge config file.
func LocalConfigPath() string {
	return filepath.Join(DotDrudgeDirName, LocalConfigName)
}

func ResolveProjectDir(slug string) (string, error) {
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(ProjectsDir(home), slug), nil
}

// ThemeConfigPath returns a path to a global theme config file
func ThemeConfigPath(home string) string {
	return filepath.Join(home, ThemeConfigName)
}

// GlobalConfigPath returns the path to the global drudge config file.
func GlobalConfigPath(home string) string {
	return filepath.Join(DrudgeDir(home), GloablConfigName)
}
