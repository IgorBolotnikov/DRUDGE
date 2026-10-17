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
)

// schemaNode is the part of a JSON schema the reference table shows.
type schemaNode struct {
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Default     json.RawMessage   `json:"default"`
	Enum        []json.RawMessage `json:"enum"`
	Minimum     *json.Number      `json:"minimum"`
	Pattern     string            `json:"pattern"`
	Required    []string          `json:"required"`
	Properties  schemaProperties  `json:"properties"`
	Items       *schemaNode       `json:"items"`
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
// not name as "<key>".
func RenderSchemaTable(schema []byte) (string, error) {
	var root schemaNode
	if err := json.Unmarshal(schema, &root); err != nil {
		return "", fmt.Errorf("could not parse the schema: %w", err)
	}
	var table strings.Builder
	table.WriteString(schemaTableHeader)
	if err := writeSchemaRows(&table, "", root); err != nil {
		return "", err
	}
	return table.String(), nil
}

// writeSchemaRows writes a row for every key below node, with prefix as the
// path of node.
func writeSchemaRows(table *strings.Builder, prefix string, node schemaNode) error {
	for _, property := range node.Properties {
		path := joinSchemaPath(prefix, property.name)
		if err := writeSchemaKey(table, path, property.node, slices.Contains(node.Required, property.name)); err != nil {
			return err
		}
	}
	if len(node.AdditionalProperties) > 0 && node.AdditionalProperties[0] == '{' {
		var anyKey schemaNode
		if err := json.Unmarshal(node.AdditionalProperties, &anyKey); err != nil {
			return fmt.Errorf("additional properties of %q: %w", prefix, err)
		}
		if err := writeSchemaKey(table, joinSchemaPath(prefix, schemaAnyKeyName), anyKey, false); err != nil {
			return err
		}
	}
	return nil
}

// writeSchemaKey writes the row of one key and the rows of the keys below it.
// It refuses a key without a type, since the table would show it empty.
func writeSchemaKey(table *strings.Builder, path string, node schemaNode, isRequired bool) error {
	if node.Type == "" {
		return fmt.Errorf("key %q has no type", path)
	}
	description := node.Description
	if isRequired {
		description = strings.TrimSpace(schemaRequiredSentence + " " + description)
	}
	fmt.Fprintf(table, "| %s | %s | %s | %s | %s |\n",
		codeSpan(path), node.Type, formatSchemaDefault(node.Default), formatSchemaAllowed(node), escapeTableText(description))

	if err := writeSchemaRows(table, path, node); err != nil {
		return err
	}
	if node.Items != nil {
		return writeSchemaRows(table, path+schemaArrayItemSuffix, *node.Items)
	}
	return nil
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

// formatSchemaAllowed lists the enum values, the minimum and the pattern of a
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
