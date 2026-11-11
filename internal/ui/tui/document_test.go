package tui

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNewDefaultDocument(t *testing.T) {
	document := NewDefaultDocument()
	if document.Schema != schemaRef {
		t.Errorf("expected $schema %q, got %q", schemaRef, document.Schema)
	}
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("could not marshal the default document: %v", err)
	}
	got, err := ParseConfig(data)
	if err != nil {
		t.Fatalf("the default document does not parse: %v", err)
	}
	if !reflect.DeepEqual(got, defaultConfig()) {
		t.Errorf("expected the default document to hold the bundled default\ngot:  %+v\nwant: %+v", got, defaultConfig())
	}
}
