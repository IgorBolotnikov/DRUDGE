package cmd

import (
	"flag"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var shouldUpdateReference = flag.Bool("update", false, "Rewrite the CLI reference pages of the docs site from the command tree")

// referenceDir holds the CLI reference pages, relative to this package.
const referenceDir = "../../site/content/docs/cli"

const (
	// referenceURLPath is where the docs site serves the reference directory.
	referenceURLPath = "/docs/cli"
	referenceTitle   = "CLI reference"
	// referenceWeight places the reference after Getting Started and Concepts in the docs sidebar.
	referenceWeight   = 3
	sectionIndexFile  = "_index.md"
	pageExtension     = ".md"
	regenerateCommand = "go test ./internal/cmd -run TestCLIReference -update"
)

// markdownEscaper escapes the characters of help text that markdown or a
// table cell would read as syntax. The backslash is replaced first, so an
// escape it introduces is not escaped again.
var markdownEscaper = strings.NewReplacer(
	`\`, `\\`,
	"`", "\\`",
	"*", `\*`,
	"_", `\_`,
	"<", `\<`,
	"[", `\[`,
	"|", `\|`,
)

func TestCLIReference(t *testing.T) {
	root := NewRoot("dev")
	root.Validate()
	want := map[string]string{}
	renderReference(root, root.Name, nil, referenceWeight, want)

	if *shouldUpdateReference {
		if err := os.RemoveAll(referenceDir); err != nil {
			t.Fatal(err)
		}
		for name, content := range want {
			pagePath := filepath.Join(referenceDir, name)
			if err := os.MkdirAll(filepath.Dir(pagePath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(pagePath, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return
	}

	got := map[string]string{}
	err := filepath.WalkDir(referenceDir, func(pagePath string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := os.ReadFile(pagePath)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(referenceDir, pagePath)
		if err != nil {
			return err
		}
		got[filepath.ToSlash(name)] = string(content)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	for _, name := range slices.Sorted(maps.Keys(want)) {
		content, ok := got[name]
		switch {
		case !ok:
			t.Errorf("%s is missing", name)
		case content != want[name]:
			t.Errorf("%s does not match the command tree", name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(got)) {
		if _, ok := want[name]; !ok {
			t.Errorf("%s belongs to no command", name)
		}
	}
	if t.Failed() {
		t.Errorf("the CLI reference in the docs site is out of date, regenerate it with %s", regenerateCommand)
	}
}

// renderReference renders the page of the command at path and of every
// command under it into pages, keyed by the file path in the reference
// directory. names holds the command path below the root.
func renderReference(command *Cmd, path string, names []string, weight int, pages map[string]string) {
	flagSet, _ := command.flagSet(path)
	var page strings.Builder

	page.WriteString("---\n")
	page.WriteString("# Generated from the drg command tree. Do not edit it, regenerate it with " + regenerateCommand + "\n")
	if len(names) == 0 {
		page.WriteString("title: " + referenceTitle + "\n")
	} else {
		page.WriteString("title: " + path + "\n")
		page.WriteString("linkTitle: " + command.Name + "\n")
	}
	fmt.Fprintf(&page, "weight: %d\n", weight)
	page.WriteString("generated: true\n")
	page.WriteString("---\n\n")

	page.WriteString("```text\n" + command.usage(path, flagSet) + "\n```\n")

	for _, line := range strings.Split(command.helpText(), "\n") {
		if line != "" {
			page.WriteString("\n" + markdownEscaper.Replace(line) + "\n")
		}
	}

	if len(command.Args) > 0 {
		page.WriteString("\n## Arguments\n\n")
		for _, arg := range command.Args {
			page.WriteString("- `<" + arg + ">`\n")
		}
	}

	if options := optionLines(flagSet); len(options) > 0 {
		page.WriteString("\n## Options\n\n")
		page.WriteString("| Option | Description | Default |\n")
		page.WriteString("| --- | --- | --- |\n")
		for _, option := range options {
			defaultValue := ""
			if option.defaultValue != "" {
				defaultValue = "`" + option.defaultValue + "`"
			}
			fmt.Fprintf(&page, "| `%s` | %s | %s |\n", option.label, markdownEscaper.Replace(option.usage), defaultValue)
		}
	}

	if len(command.Subcommands) > 0 {
		page.WriteString("\n## Subcommands\n\n")
		page.WriteString("| Command | Description |\n")
		page.WriteString("| --- | --- |\n")
		for _, sub := range command.Subcommands {
			subNames := append(slices.Clone(names), sub.Name)
			fmt.Fprintf(&page, "| [`%s`]({{< relref \"%s\" >}}) | %s |\n",
				path+pathSeparator+sub.Name, referenceURLPath+"/"+strings.Join(subNames, "/"), markdownEscaper.Replace(sub.Desc))
		}
	}

	pages[referencePageFile(command, names)] = page.String()
	for index, sub := range command.Subcommands {
		renderReference(sub, path+pathSeparator+sub.Name, append(slices.Clone(names), sub.Name), index+1, pages)
	}
}

// referencePageFile returns the file of the page of a command. A command with
// subcommands is a section of the docs site, so its subcommands nest under it
// in the sidebar.
func referencePageFile(command *Cmd, names []string) string {
	if len(command.Subcommands) > 0 {
		return strings.Join(append(slices.Clone(names), sectionIndexFile), "/")
	}
	return strings.Join(names, "/") + pageExtension
}
