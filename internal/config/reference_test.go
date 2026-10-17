package config

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

var shouldUpdate = flag.Bool("update", false, "rewrite the generated tables of the configuration reference pages")

const (
	referenceDir        = "../../site/content/docs/configuration"
	regenerateCommand   = "go test ./internal/config -update"
	globalReferencePage = "config.md"
	localReferencePage  = "project-config.md"
)

// renderReferencePage returns the page at path with its generated table
// rendered from schema.
func renderReferencePage(t *testing.T, path string, schema []byte) (current string, rendered string) {
	t.Helper()
	page, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read the reference page: %v", err)
	}
	table, err := common.RenderSchemaTable(schema)
	if err != nil {
		t.Fatalf("could not render the schema: %v", err)
	}
	rendered, err = common.SpliceGenerated(string(page), table)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return string(page), rendered
}

func TestReferencePagesMatchSchemas(t *testing.T) {
	cases := []struct {
		page   string
		schema []byte
	}{
		{page: globalReferencePage, schema: Schema()},
		{page: localReferencePage, schema: LocalSchema()},
	}

	for _, testCase := range cases {
		t.Run(testCase.page, func(t *testing.T) {
			path := filepath.Join(referenceDir, testCase.page)
			current, rendered := renderReferencePage(t, path, testCase.schema)
			if *shouldUpdate {
				if err := os.WriteFile(path, []byte(rendered), common.DefaultFilePerm); err != nil {
					t.Fatalf("could not write the reference page: %v", err)
				}
				return
			}
			if current != rendered {
				t.Errorf("the generated table of %s does not match its schema, run `%s` to regenerate it", path, regenerateCommand)
			}
		})
	}
}

func TestReferencePageCatchesNewKey(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(Schema(), &schema); err != nil {
		t.Fatalf("could not parse the schema: %v", err)
	}
	schema["properties"].(map[string]any)["newKey"] = map[string]any{"type": "string"}
	changed, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("could not encode the schema: %v", err)
	}

	current, rendered := renderReferencePage(t, filepath.Join(referenceDir, globalReferencePage), changed)
	if current == rendered {
		t.Error("expected a new key in the schema to change the generated table")
	}
}
