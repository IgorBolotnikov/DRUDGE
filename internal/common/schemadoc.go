package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const (
	// GeneratedBeginMarker opens the generated section of a docs page.
	GeneratedBeginMarker = "<!-- BEGIN GENERATED: edits between these markers are overwritten -->"
	// GeneratedEndMarker closes the generated section of a docs page.
	GeneratedEndMarker = "<!-- END GENERATED -->"

	schemaKeySeparator     = "."
	schemaArrayItemSuffix  = "[]"
	schemaAnyKeyName       = "<key>"
	schemaTableHeader      = "| Key | Type | Default | Allowed | Description |\n| --- | --- | --- | --- | --- |\n"
	schemaRequiredSentence = "Required."
	// schemaDefinitionRefPrefix starts a reference to a definition of the
	// schema.
	schemaDefinitionRefPrefix = "#/$defs/"
)

// schemaNode is the part of a JSON schema the reference table shows.
type schemaNode struct {
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Default     json.RawMessage   `json:"default"`
	Enum        []json.RawMessage `json:"enum"`
	Minimum     *json.Number      `json:"minimum"`
	// ExclusiveMinimum is the number every value is above.
	ExclusiveMinimum *json.Number     `json:"exclusiveMinimum"`
	Pattern          string           `json:"pattern"`
	Required         []string         `json:"required"`
	Properties       schemaProperties `json:"properties"`
	Items            *schemaNode      `json:"items"`
	Ref              string           `json:"$ref"`
	// Definitions holds the nodes a reference names. Only the root of the
	// schema has them.
	Definitions map[string]schemaNode `json:"$defs"`
	// AdditionalProperties holds either a boolean or the schema of every key
	// the properties leave out.
	AdditionalProperties json.RawMessage `json:"additionalProperties"`
}

type schemaProperty struct {
	name string
	node schemaNode
}

// schemaProperties keeps the properties of an object in the order the schema
// lists them.
type schemaProperties []schemaProperty

func (properties *schemaProperties) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return fmt.Errorf("properties must be an object, got %v", token)
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		var node schemaNode
		if err := decoder.Decode(&node); err != nil {
			return fmt.Errorf("property %q: %w", token, err)
		}
		*properties = append(*properties, schemaProperty{name: token.(string), node: node})
	}
	return nil
}

// RenderSchemaTable renders every key of a JSON schema as a row of a markdown
// table, in the order the schema lists them. A nested key reads as a dotted
// path, an item of an array as "[]" after the array and a key the schema does
// not name as "<key>". A key that refers to a definition of the schema shows
// the keys of the definition below it. A definition that refers to itself
// shows its keys once, at the outermost key that refers to it.
func RenderSchemaTable(schema []byte) (string, error) {
	var root schemaNode
	if err := json.Unmarshal(schema, &root); err != nil {
		return "", fmt.Errorf("could not parse the schema: %w", err)
	}
	writer := &schemaTableWriter{definitions: root.Definitions, expandingRefs: map[string]bool{}}
	writer.table.WriteString(schemaTableHeader)
	if err := writer.writeChildren("", "", root); err != nil {
		return "", err
	}
	return writer.table.String(), nil
}

// schemaTableWriter writes the rows of a schema table. It tracks the
// references it is expanding to stop at a definition that refers to itself.
type schemaTableWriter struct {
	table         strings.Builder
	definitions   map[string]schemaNode
	expandingRefs map[string]bool
}

// writeChildren writes a row for every key below node, with path as the path
// of node. ref is the reference node came from, or empty.
func (writer *schemaTableWriter) writeChildren(path string, ref string, node schemaNode) error {
	if ref != "" {
		if writer.expandingRefs[ref] {
			return nil
		}
		writer.expandingRefs[ref] = true
		defer delete(writer.expandingRefs, ref)
	}
	for _, property := range node.Properties {
		if err := writer.writeKey(joinSchemaPath(path, property.name), property.node, slices.Contains(node.Required, property.name)); err != nil {
			return err
		}
	}
	if len(node.AdditionalProperties) > 0 && node.AdditionalProperties[0] == '{' {
		var anyKey schemaNode
		if err := json.Unmarshal(node.AdditionalProperties, &anyKey); err != nil {
			return fmt.Errorf("additional properties of %q: %w", path, err)
		}
		if err := writer.writeKey(joinSchemaPath(path, schemaAnyKeyName), anyKey, false); err != nil {
			return err
		}
	}
	if node.Items == nil {
		return nil
	}
	itemsPath := path + schemaArrayItemSuffix
	items, err := writer.resolve(itemsPath, *node.Items)
	if err != nil {
		return err
	}
	return writer.writeChildren(itemsPath, node.Items.Ref, items)
}

// writeKey writes the row of one key and the rows of the keys below it. It
// refuses a key without a type, since the table would show it empty.
func (writer *schemaTableWriter) writeKey(path string, node schemaNode, isRequired bool) error {
	resolved, err := writer.resolve(path, node)
	if err != nil {
		return err
	}
	if resolved.Type == "" {
		return fmt.Errorf("key %q has no type", path)
	}
	description := resolved.Description
	if isRequired {
		description = strings.TrimSpace(schemaRequiredSentence + " " + description)
	}
	fmt.Fprintf(&writer.table, "| %s | %s | %s | %s | %s |\n",
		codeSpan(path), resolved.Type, formatSchemaDefault(resolved.Default), formatSchemaAllowed(resolved), escapeTableText(description))
	return writer.writeChildren(path, node.Ref, resolved)
}

// resolve returns the definition a node refers to, with the description of
// the node when it has one. A node without a reference is returned as it is.
// It refuses a reference outside the definitions of the schema.
func (writer *schemaTableWriter) resolve(path string, node schemaNode) (schemaNode, error) {
	if node.Ref == "" {
		return node, nil
	}
	name, isDefinitionRef := strings.CutPrefix(node.Ref, schemaDefinitionRefPrefix)
	definition, ok := writer.definitions[name]
	if !isDefinitionRef || !ok {
		return schemaNode{}, fmt.Errorf("key %q refers to %q, which is not a definition of the schema", path, node.Ref)
	}
	if node.Description != "" {
		definition.Description = node.Description
	}
	return definition, nil
}

func joinSchemaPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + schemaKeySeparator + name
}

func formatSchemaDefault(value json.RawMessage) string {
	if len(value) == 0 {
		return ""
	}
	return codeSpan(string(value))
}

// formatSchemaAllowed lists the enum values, the minimums and the pattern of a
// key.
func formatSchemaAllowed(node schemaNode) string {
	var allowed []string
	if len(node.Enum) > 0 {
		values := make([]string, len(node.Enum))
		for index, value := range node.Enum {
			values[index] = codeSpan(string(value))
		}
		allowed = append(allowed, strings.Join(values, ", "))
	}
	if node.Minimum != nil {
		allowed = append(allowed, "minimum "+node.Minimum.String())
	}
	if node.ExclusiveMinimum != nil {
		allowed = append(allowed, "above "+node.ExclusiveMinimum.String())
	}
	if node.Pattern != "" {
		allowed = append(allowed, "matches "+codeSpan(node.Pattern))
	}
	return strings.Join(allowed, "<br>")
}

// codeSpan wraps text in backticks. A table cell ends at an unescaped pipe
// even inside a code span, so pipes are escaped.
func codeSpan(text string) string {
	return "`" + strings.ReplaceAll(text, "|", `\|`) + "`"
}

// tableTextEscaper keeps a description from ending its table cell or opening
// an HTML tag. The docs site renders raw HTML.
var tableTextEscaper = strings.NewReplacer("|", `\|`, "<", "&lt;", ">", "&gt;", "\n", " ")

func escapeTableText(text string) string {
	return tableTextEscaper.Replace(text)
}

// SpliceGenerated returns page with the text between its generated markers
// replaced by generated. It refuses a page without exactly one begin marker
// followed by exactly one end marker.
func SpliceGenerated(page, generated string) (string, error) {
	if strings.Count(page, GeneratedBeginMarker) != 1 || strings.Count(page, GeneratedEndMarker) != 1 {
		return "", fmt.Errorf("page needs exactly one %q line and one %q line", GeneratedBeginMarker, GeneratedEndMarker)
	}
	begin := strings.Index(page, GeneratedBeginMarker) + len(GeneratedBeginMarker)
	end := strings.Index(page, GeneratedEndMarker)
	if end < begin {
		return "", fmt.Errorf("%q comes before %q", GeneratedEndMarker, GeneratedBeginMarker)
	}
	return page[:begin] + "\n\n" + strings.TrimSpace(generated) + "\n\n" + page[end:], nil
}
