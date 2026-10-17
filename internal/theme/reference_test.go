package theme

import (
	"flag"
	"os"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

var shouldUpdate = flag.Bool("update", false, "rewrite the generated table of the theme reference page")

const (
	referencePage     = "../../site/content/docs/configuration/theme.md"
	regenerateCommand = "go test ./internal/theme -update"
)

func TestReferencePageMatchesSchema(t *testing.T) {
	page, err := os.ReadFile(referencePage)
	if err != nil {
		t.Fatalf("could not read the reference page: %v", err)
	}
	table, err := common.RenderSchemaTable(Schema())
	if err != nil {
		t.Fatalf("could not render the schema: %v", err)
	}
	rendered, err := common.SpliceGenerated(string(page), table)
	if err != nil {
		t.Fatalf("%s: %v", referencePage, err)
	}

	if *shouldUpdate {
		if err := os.WriteFile(referencePage, []byte(rendered), common.DefaultFilePerm); err != nil {
			t.Fatalf("could not write the reference page: %v", err)
		}
		return
	}
	if string(page) != rendered {
		t.Errorf("the generated table of %s does not match its schema, run `%s` to regenerate it", referencePage, regenerateCommand)
	}
}
