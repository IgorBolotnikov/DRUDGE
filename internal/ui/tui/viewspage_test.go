package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
)

const (
	viewsPage = "../../../site/content/docs/tui/views.md"
	// viewsPageWeight places the views page first below the TUI page in the
	// docs sidebar.
	viewsPageWeight        = 1
	viewsRegenerateCommand = "go test ./internal/ui/tui -run TestViewsPage -update"
)

// viewsPageEscaper escapes the characters of a description that a table cell
// reads as syntax. The backslash is replaced first, so an escape it
// introduces is not escaped again.
var viewsPageEscaper = strings.NewReplacer(`\`, `\\`, "|", `\|`)

func TestViewsPage(t *testing.T) {
	want := renderViewsPage()

	if tuitest.ShouldUpdate() {
		if err := os.WriteFile(viewsPage, []byte(want), common.DefaultFilePerm); err != nil {
			t.Fatalf("could not write the views page: %v", err)
		}
		return
	}
	got, err := os.ReadFile(viewsPage)
	if err != nil {
		t.Fatalf("could not read the views page: %v, regenerate it with %s", err, viewsRegenerateCommand)
	}
	if string(got) != want {
		t.Errorf("%s does not match the View registry, regenerate it with %s", viewsPage, viewsRegenerateCommand)
	}
}

// renderViewsPage renders a section per View of the registry with its
// description, its view options and its keys.
func renderViewsPage() string {
	var page strings.Builder
	page.WriteString("---\n")
	page.WriteString("# Generated from the View registry. Do not edit it, regenerate it with " + viewsRegenerateCommand + "\n")
	page.WriteString("title: Views\n")
	fmt.Fprintf(&page, "weight: %d\n", viewsPageWeight)
	page.WriteString("generated: true\n")
	page.WriteString("---\n\n")
	page.WriteString("A tile shows the view its `view` key names in the [TUI config]({{< relref \"/docs/configuration/tui\" >}}). `viewOptions` of the tile sets the options of the view. Press `?` in `drg tui` to list the keys of the focused tile.\n")

	for _, kind := range viewKinds {
		page.WriteString("\n## `" + kind.name + "`\n\n")
		page.WriteString(kind.description + "\n")

		page.WriteString("\n### View options\n\n")
		if len(kind.options) == 0 {
			page.WriteString("The view takes no options.\n")
		} else {
			page.WriteString("| Option | Type | Default | Allowed | Description |\n")
			page.WriteString("| --- | --- | --- | --- | --- |\n")
			for _, option := range kind.options {
				fmt.Fprintf(&page, "| `%s` | %s | %s | %s | %s |\n",
					option.Name, option.Type, codeSpans([]string{option.Default}), codeSpans(option.Allowed), viewsPageEscaper.Replace(option.Description))
			}
		}

		page.WriteString("\n### Keys\n\n")
		page.WriteString("| Key | Does |\n")
		page.WriteString("| --- | --- |\n")
		for _, binding := range kind.keys {
			help := binding.Help()
			fmt.Fprintf(&page, "| `%s` | %s |\n", help.Key, viewsPageEscaper.Replace(help.Desc))
		}
	}
	return page.String()
}

// codeSpans renders each value that is not empty as code, joined by commas.
func codeSpans(values []string) string {
	var spans []string
	for _, value := range values {
		if value != "" {
			spans = append(spans, "`"+value+"`")
		}
	}
	return strings.Join(spans, ", ")
}
