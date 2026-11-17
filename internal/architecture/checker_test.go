// Package architecture checks the import rules of the layers of drudge.
package architecture

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const (
	modulePath   = "github.com/IgorBolotnikov/DRUDGE/"
	goFileSuffix = ".go"
	testSuffix   = "_test.go"
	goModFile    = "go.mod"
	testdataDir  = "testdata"
	tuitestDir   = "internal/ui/tui/tuitest"
	rulesDoc     = "see the Interfaces block of CLAUDE.md"
)

// rule forbids the files under some directories some imports and some names
// of imported packages. Directories are slash separated prefixes of paths
// relative to the module root.
type rule struct {
	name       string
	dirs       []string
	exemptDirs []string
	// forbiddenImports holds prefixes of import paths.
	forbiddenImports []string
	// allowedSelectors holds, per import path, the only names a file may use
	// from that package.
	allowedSelectors map[string][]string
	// forbiddenSelectors holds, per import path, prefixes of names a file may
	// not use from that package.
	forbiddenSelectors map[string][]string
	reason             string
}

// violation is one use of something a rule forbids.
type violation struct {
	position token.Position
	used     string
	rule     rule
}

func (found violation) String() string {
	return fmt.Sprintf("%s:%d: uses %s (%s: %s, %s)", found.position.Filename, found.position.Line, found.used, found.rule.name, found.rule.reason, rulesDoc)
}

func (checked rule) covers(relPath string) bool {
	if hasAnyPrefix(relPath, checked.exemptDirs) {
		return false
	}
	return hasAnyPrefix(relPath, checked.dirs)
}

// checkModule checks every non-test Go file of the module at root. It skips
// the files Go itself skips, nested modules and the TUI test helpers.
func checkModule(root string, rules []rule) ([]violation, error) {
	var violations []violation
	err := filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		if entry.IsDir() {
			if relPath != "." && isSkippedDir(filePath, relPath, entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(relPath, goFileSuffix) || strings.HasSuffix(relPath, testSuffix) {
			return nil
		}
		found, err := checkFile(filePath, relPath, rules)
		if err != nil {
			return err
		}
		violations = append(violations, found...)
		return nil
	})
	return violations, err
}

// isSkippedDir reports whether the walk leaves a directory out: a directory Go
// ignores, the TUI test helpers or the root of a nested module like the docs
// site.
func isSkippedDir(dirPath string, relPath string, name string) bool {
	isIgnoredByGo := strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == testdataDir
	if isIgnoredByGo || relPath == tuitestDir {
		return true
	}
	_, err := os.Stat(filepath.Join(dirPath, goModFile))
	return err == nil
}

// checkFile parses a Go file and returns what it uses that the rules covering
// it forbid.
func checkFile(filePath string, relPath string, rules []rule) ([]violation, error) {
	var applying []rule
	for _, candidate := range rules {
		if candidate.covers(relPath) {
			applying = append(applying, candidate)
		}
	}
	if len(applying) == 0 {
		return nil, nil
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filePath, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	position := func(node ast.Node) token.Position {
		at := fileSet.Position(node.Pos())
		at.Filename = relPath
		return at
	}

	var violations []violation
	importsByName := make(map[string]string, len(file.Imports))
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		importsByName[importName(spec, importPath)] = importPath
		for _, applied := range applying {
			if hasAnyPrefix(importPath, applied.forbiddenImports) {
				violations = append(violations, violation{position: position(spec), used: importPath, rule: applied})
			}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		importPath, ok := importsByName[ident.Name]
		if !ok {
			return true
		}
		name := selector.Sel.Name
		for _, applied := range applying {
			if isForbiddenSelector(applied, importPath, name) {
				used := importPath + "." + name
				violations = append(violations, violation{position: position(selector), used: used, rule: applied})
			}
		}
		return true
	})
	return violations, nil
}

// isForbiddenSelector reports whether the rule forbids the name of the package
// at importPath.
func isForbiddenSelector(applied rule, importPath string, name string) bool {
	if allowed, ok := applied.allowedSelectors[importPath]; ok && !slices.Contains(allowed, name) {
		return true
	}
	return hasAnyPrefix(name, applied.forbiddenSelectors[importPath])
}

// importName returns the name a file refers to an import by. A package without
// an alias goes by the last element of its path, with a major version suffix
// dropped.
func importName(spec *ast.ImportSpec, importPath string) string {
	if spec.Name != nil {
		return spec.Name.Name
	}
	name := path.Base(importPath)
	if isMajorVersion(name) {
		name = path.Base(path.Dir(importPath))
	}
	return name
}

// isMajorVersion reports whether a path element is a major version suffix,
// like v2.
func isMajorVersion(element string) bool {
	version, ok := strings.CutPrefix(element, "v")
	if !ok {
		return false
	}
	_, err := strconv.Atoi(version)
	return err == nil
}

func hasAnyPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
