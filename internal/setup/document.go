package setup

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

// keySeparator joins the names of nested JSON keys in the key of a question.
const keySeparator = "."

// toJSONObject returns value as the generic JSON object it encodes to.
func toJSONObject(value any) (map[string]any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("could not marshal a config file: %w", err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, fmt.Errorf("could not unmarshal a config file: %w", err)
	}
	return object, nil
}

// lookupKey returns the value of a dotted key in object.
func lookupKey(object map[string]any, key string) (any, bool) {
	names := strings.Split(key, keySeparator)
	for _, name := range names[:len(names)-1] {
		child, ok := object[name].(map[string]any)
		if !ok {
			return nil, false
		}
		object = child
	}
	value, ok := object[names[len(names)-1]]
	return value, ok
}

// setKey sets a dotted key in object and creates the missing objects on its
// path. It refuses a path through a value that is not an object.
func setKey(object map[string]any, key string, value any) error {
	names := strings.Split(key, keySeparator)
	for _, name := range names[:len(names)-1] {
		if object[name] == nil {
			object[name] = map[string]any{}
		}
		child, ok := object[name].(map[string]any)
		if !ok {
			return fmt.Errorf("has %s = %v, it must be an object to hold %s", name, object[name], key)
		}
		object = child
	}
	object[names[len(names)-1]] = value
	return nil
}

// deleteKey removes a dotted key from object.
func deleteKey(object map[string]any, key string) {
	names := strings.Split(key, keySeparator)
	for _, name := range names[:len(names)-1] {
		child, ok := object[name].(map[string]any)
		if !ok {
			return
		}
		object = child
	}
	delete(object, names[len(names)-1])
}

// currentAnswer returns the answer a question gets from the value of key in
// object. A key that is missing or holds null or a nested value has no answer.
func currentAnswer(object map[string]any, key string) (string, bool) {
	value, _ := lookupKey(object, key)
	switch typed := value.(type) {
	case string:
		return typed, true
	case bool:
		return common.FormatBoolAnswer(typed), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	default:
		return "", false
	}
}
