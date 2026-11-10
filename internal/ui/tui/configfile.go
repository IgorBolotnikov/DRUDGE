package tui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
)

// Keys of the TUI config file.
const (
	schemaKey             = "$schema"
	tabsKey               = tabsPath
	dataRefreshSecondsKey = "dataRefreshSeconds"
	titleKey              = "title"
	layoutKey             = layoutPath
	arrangementKey        = "arrangement"
	childrenKey           = "children"
	viewKey               = "view"
	weightKey             = "weight"
	showStatusKey         = "showStatus"
	viewOptionsKey        = "viewOptions"
)

// Keys each object of the TUI config file may hold, in the order the errors
// list them.
var (
	rootKeys = []string{schemaKey, tabsKey, dataRefreshSecondsKey}
	tabKeys  = []string{titleKey, layoutKey}
	nodeKeys = []string{arrangementKey, childrenKey, viewKey, titleKey, weightKey, showStatusKey, viewOptionsKey}
)

// Kinds of JSON values, as the errors name them.
const (
	jsonObject  = "an object"
	jsonArray   = "an array"
	jsonString  = "a string"
	jsonNumber  = "a number"
	jsonBoolean = "a boolean"
	jsonNull    = "null"
)

const (
	keyPathSeparator = "."
	keyNameSeparator = ", "
)

// LoadConfig reads the TUI config file at path. A missing file gives the
// bundled default. An error starts with the path of the file.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return defaultConfig(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("could not read %s: %w", path, err)
	}
	config, err := ParseConfig(data)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return config, nil
}

// ParseConfig decodes a TUI config file. It refuses JSON that does not parse,
// a value of the wrong kind, an unknown key, a weight that is not above 0 and
// a data refresh below its minimum.
// An error starts with the path of the value to change, like
// tabs[0].layout.children[1].weight. BuildTabs checks the rest.
func ParseConfig(data []byte) (Config, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return Config{}, fmt.Errorf("the file is not valid JSON: %w", err)
	}
	root, err := decodeObject(data, "", rootKeys)
	if err != nil {
		return Config{}, err
	}
	if rawSchema, ok := root[schemaKey]; ok {
		if _, err := decodeValue[string](rawSchema, schemaKey, jsonString); err != nil {
			return Config{}, err
		}
	}
	var config Config
	if rawSeconds, ok := root[dataRefreshSecondsKey]; ok {
		if config.DataRefreshSeconds, err = decodeValue[float64](rawSeconds, dataRefreshSecondsKey, jsonNumber); err != nil {
			return Config{}, err
		}
		if config.DataRefreshSeconds < minDataRefreshSeconds {
			return Config{}, fmt.Errorf("%s: %g is below the minimum of %g, use %g or more seconds", dataRefreshSecondsKey, config.DataRefreshSeconds, minDataRefreshSeconds, minDataRefreshSeconds)
		}
	}
	rawTabs, ok := root[tabsKey]
	if !ok {
		return config, nil
	}
	tabs, err := decodeValue[[]json.RawMessage](rawTabs, tabsKey, jsonArray)
	if err != nil {
		return Config{}, err
	}
	for index, rawTab := range tabs {
		tab, err := parseTab(rawTab, tabPath(index))
		if err != nil {
			return Config{}, err
		}
		config.Tabs = append(config.Tabs, tab)
	}
	return config, nil
}

func parseTab(raw json.RawMessage, path string) (TabConfig, error) {
	object, err := decodeObject(raw, path, tabKeys)
	if err != nil {
		return TabConfig{}, err
	}
	var tab TabConfig
	if rawTitle, ok := object[titleKey]; ok {
		if tab.Title, err = decodeValue[string](rawTitle, joinKeyPath(path, titleKey), jsonString); err != nil {
			return TabConfig{}, err
		}
	}
	if rawLayout, ok := object[layoutKey]; ok {
		if tab.Layout, err = parseNode(rawLayout, joinKeyPath(path, layoutKey)); err != nil {
			return TabConfig{}, err
		}
	}
	return tab, nil
}

// parseNode decodes a node of a layout. It refuses a weight that is not above
// 0, since a node config takes a weight of 0 as the default weight.
func parseNode(raw json.RawMessage, path string) (NodeConfig, error) {
	object, err := decodeObject(raw, path, nodeKeys)
	if err != nil {
		return NodeConfig{}, err
	}
	var node NodeConfig
	if rawArrangement, ok := object[arrangementKey]; ok {
		arrangement, err := decodeValue[string](rawArrangement, joinKeyPath(path, arrangementKey), jsonString)
		if err != nil {
			return NodeConfig{}, err
		}
		node.Arrangement = Arrangement(arrangement)
	}
	if rawChildren, ok := object[childrenKey]; ok {
		children, err := decodeValue[[]json.RawMessage](rawChildren, joinKeyPath(path, childrenKey), jsonArray)
		if err != nil {
			return NodeConfig{}, err
		}
		for index, rawChild := range children {
			child, err := parseNode(rawChild, childPath(path, index))
			if err != nil {
				return NodeConfig{}, err
			}
			node.Children = append(node.Children, child)
		}
	}
	if rawView, ok := object[viewKey]; ok {
		if node.View, err = decodeValue[string](rawView, joinKeyPath(path, viewKey), jsonString); err != nil {
			return NodeConfig{}, err
		}
	}
	if rawTitle, ok := object[titleKey]; ok {
		if node.Title, err = decodeValue[string](rawTitle, joinKeyPath(path, titleKey), jsonString); err != nil {
			return NodeConfig{}, err
		}
	}
	if rawWeight, ok := object[weightKey]; ok {
		if node.Weight, err = decodeValue[float64](rawWeight, joinKeyPath(path, weightKey), jsonNumber); err != nil {
			return NodeConfig{}, err
		}
		if err := validateWeight(node.Weight, path); err != nil {
			return NodeConfig{}, err
		}
	}
	if rawShowStatus, ok := object[showStatusKey]; ok {
		isStatusShown, err := decodeValue[bool](rawShowStatus, joinKeyPath(path, showStatusKey), jsonBoolean)
		if err != nil {
			return NodeConfig{}, err
		}
		node.ShowStatus = &isStatusShown
	}
	if rawViewOptions, ok := object[viewOptionsKey]; ok {
		if node.ViewOptions, err = decodeValue[map[string]any](rawViewOptions, joinKeyPath(path, viewOptionsKey), jsonObject); err != nil {
			return NodeConfig{}, err
		}
	}
	return node, nil
}

// decodeObject decodes a JSON object at path and refuses a key outside keys.
func decodeObject(raw json.RawMessage, path string, keys []string) (map[string]json.RawMessage, error) {
	object, err := decodeValue[map[string]json.RawMessage](raw, path, jsonObject)
	if err != nil {
		return nil, err
	}
	for _, key := range slices.Sorted(maps.Keys(object)) {
		if !slices.Contains(keys, key) {
			return nil, fmt.Errorf("%s: unknown key, use one of: %s", joinKeyPath(path, key), strings.Join(keys, keyNameSeparator))
		}
	}
	return object, nil
}

// decodeValue decodes the JSON value at path into a T. It refuses null and a
// value of another kind than wantKind.
func decodeValue[T any](raw json.RawMessage, path string, wantKind string) (T, error) {
	var value T
	gotKind := jsonKind(raw)
	if gotKind == jsonNull || json.Unmarshal(raw, &value) != nil {
		return value, pathError(path, fmt.Sprintf("expected %s, got %s", wantKind, gotKind))
	}
	return value, nil
}

// jsonKind names the kind of a valid JSON value from its first character.
func jsonKind(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return jsonNull
	}
	switch trimmed[0] {
	case '{':
		return jsonObject
	case '[':
		return jsonArray
	case '"':
		return jsonString
	case 't', 'f':
		return jsonBoolean
	case 'n':
		return jsonNull
	default:
		return jsonNumber
	}
}

// pathError returns an error that starts with path. An error about the whole
// file has an empty path and is the message alone.
func pathError(path string, message string) error {
	if path == "" {
		return errors.New(message)
	}
	return fmt.Errorf("%s: %s", path, message)
}

func joinKeyPath(path string, key string) string {
	if path == "" {
		return key
	}
	return path + keyPathSeparator + key
}
