// Package common contains common functions
package common

import (
	"encoding/json"
	"fmt"
	"os"
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
	if err := os.WriteFile(path, data, 0o644); err != nil {
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
