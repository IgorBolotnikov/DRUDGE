package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// setupRule proposes setup lines for one repository root. A rule that does
// not match returns no lines.
type setupRule func(root repositoryRoot) ([]SetupLine, error)

// setupRules are the rules detection walks over, in the order their lines
// run. The toolchain goes first, so the package managers it pins are on PATH
// for the lines after it.
var setupRules = []setupRule{
	toolchainSetup,
	packageSetup,
}

// Files in a repository root the rules read.
const (
	toolVersionsFile     = ".tool-versions"
	miseConfigFile       = "mise.toml"
	hiddenMiseConfigFile = ".mise.toml"
	nvmrcFile            = ".nvmrc"
	nodeVersionFile      = ".node-version"
	pythonVersionFile    = ".python-version"
	npmLockfile          = "package-lock.json"
	yarnLockfile         = "yarn.lock"
	yarnBerryConfigFile  = ".yarnrc.yml"
	pnpmLockfile         = "pnpm-lock.yaml"
	uvLockfile           = "uv.lock"
	poetryLockfile       = "poetry.lock"
	pyprojectFile        = "pyproject.toml"
	pipRequirementsFile  = "requirements.txt"
	miseToolsTable       = "tools"
	poetryPyprojectTable = "tool.poetry"
	commentPrefix        = "#"
)

// toolchainPinFiles are the files that pin a toolchain mise installs, in the
// order the first one found names the trigger.
var toolchainPinFiles = []string{
	toolVersionsFile,
	miseConfigFile,
	hiddenMiseConfigFile,
	nvmrcFile,
	nodeVersionFile,
	pythonVersionFile,
}

// miseConfigFiles are the mise config files a tool is pinned in, next to the
// tool versions file.
var miseConfigFiles = []string{miseConfigFile, hiddenMiseConfigFile}

// Tools a package rule takes from mise when the repository pins them.
const (
	yarnTool   = "yarn"
	poetryTool = "poetry"
)

// Setup lines of the toolchain rule. mise reads the idiomatic version files of
// node and python only once the setting lists them.
const (
	installMiseLine         = "command -v mise >/dev/null 2>&1 || curl -fsSL https://mise.run | sh"
	enableIdiomaticPinsLine = "mise settings set idiomatic_version_file_enable_tools node,python"
	miseInstallLine         = "mise install"
	addMiseShimsLine        = `echo "$HOME/.local/share/mise/shims" >> "$DRUDGE_PATH"`
)

// Setup lines of the package rule. Corepack links yarn and pnpm into the user
// bin directory.
const (
	enableCorepackLine     = `corepack enable --install-directory "$HOME/.local/bin"`
	npmInstallLine         = "npm ci"
	yarnBerryInstallLine   = "yarn install --immutable"
	yarnClassicInstallLine = "yarn install --frozen-lockfile"
	pnpmInstallLine        = "pnpm install --frozen-lockfile"
	uvSyncLine             = "uv sync --frozen"
	installPoetryLine      = "uv tool install poetry"
	poetryInstallLine      = "poetry install"
	createVenvLine         = "python3 -m venv .venv"
	pipInstallLine         = ".venv/bin/pip install -r " + pipRequirementsFile
)

// toolchainSetup installs mise and the toolchains a repository pins.
// A go.mod and the engines of a package.json pin nothing.
func toolchainSetup(root repositoryRoot) ([]SetupLine, error) {
	trigger, err := root.firstPresent(toolchainPinFiles)
	if err != nil || trigger == "" {
		return nil, err
	}
	return linesFrom(trigger, installMiseLine, enableIdiomaticPinsLine, miseInstallLine, addMiseShimsLine), nil
}

// packageSetup installs the packages of the first lockfile found, in the order
// the package managers are checked below.
func packageSetup(root repositoryRoot) ([]SetupLine, error) {
	for _, rule := range []setupRule{npmSetup, yarnSetup, pnpmSetup, uvSetup, poetrySetup, pipSetup} {
		lines, err := rule(root)
		if err != nil || len(lines) > 0 {
			return lines, err
		}
	}
	return nil, nil
}

func npmSetup(root repositoryRoot) ([]SetupLine, error) {
	return linesIfPresent(root, npmLockfile, npmInstallLine)
}

// yarnSetup takes Berry yarn from corepack. Classic yarn comes from mise when
// the repository pins it and from corepack otherwise.
func yarnSetup(root repositoryRoot) ([]SetupLine, error) {
	hasLockfile, err := root.has(yarnLockfile)
	if err != nil || !hasLockfile {
		return nil, err
	}

	isBerry, err := root.has(yarnBerryConfigFile)
	if err != nil {
		return nil, err
	}
	if isBerry {
		return []SetupLine{
			{Command: enableCorepackLine, Trigger: yarnBerryConfigFile},
			{Command: yarnBerryInstallLine, Trigger: yarnLockfile},
		}, nil
	}

	isPinned, err := root.pins(yarnTool)
	if err != nil {
		return nil, err
	}
	if isPinned {
		return linesFrom(yarnLockfile, yarnClassicInstallLine), nil
	}
	return linesFrom(yarnLockfile, enableCorepackLine, yarnClassicInstallLine), nil
}

func pnpmSetup(root repositoryRoot) ([]SetupLine, error) {
	return linesIfPresent(root, pnpmLockfile, enableCorepackLine, pnpmInstallLine)
}

func uvSetup(root repositoryRoot) ([]SetupLine, error) {
	return linesIfPresent(root, uvLockfile, uvSyncLine)
}

// poetrySetup matches a poetry lockfile or a pyproject with a poetry table.
// Poetry comes from mise when the repository pins it and from uv otherwise.
func poetrySetup(root repositoryRoot) ([]SetupLine, error) {
	trigger, err := poetryTrigger(root)
	if err != nil || trigger == "" {
		return nil, err
	}

	isPinned, err := root.pins(poetryTool)
	if err != nil {
		return nil, err
	}
	if isPinned {
		return linesFrom(trigger, poetryInstallLine), nil
	}
	return linesFrom(trigger, installPoetryLine, poetryInstallLine), nil
}

// poetryTrigger names the file that makes a repository a poetry project, and
// is empty for one that is not.
func poetryTrigger(root repositoryRoot) (string, error) {
	hasLockfile, err := root.has(poetryLockfile)
	if err != nil {
		return "", err
	}
	if hasLockfile {
		return poetryLockfile, nil
	}

	pyproject, err := root.read(pyprojectFile)
	if err != nil {
		return "", err
	}
	for line := range strings.Lines(pyproject) {
		header, isHeader := tomlTableHeader(line)
		if isHeader && (header == poetryPyprojectTable || strings.HasPrefix(header, poetryPyprojectTable+".")) {
			return pyprojectFile, nil
		}
	}
	return "", nil
}

func pipSetup(root repositoryRoot) ([]SetupLine, error) {
	return linesIfPresent(root, pipRequirementsFile, createVenvLine, pipInstallLine)
}

// linesIfPresent returns the commands triggered by a file when the root holds
// it.
func linesIfPresent(root repositoryRoot, trigger string, commands ...string) ([]SetupLine, error) {
	isPresent, err := root.has(trigger)
	if err != nil || !isPresent {
		return nil, err
	}
	return linesFrom(trigger, commands...), nil
}

func linesFrom(trigger string, commands ...string) []SetupLine {
	lines := make([]SetupLine, 0, len(commands))
	for _, command := range commands {
		lines = append(lines, SetupLine{Command: command, Trigger: trigger})
	}
	return lines
}

// repositoryRoot is the root directory of a repository. The rules read files
// in it and never below it.
type repositoryRoot struct {
	dir string
}

// has tells whether the root holds a file of that name.
func (root repositoryRoot) has(name string) (bool, error) {
	info, err := os.Stat(filepath.Join(root.dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("could not read %s: %w", filepath.Join(root.dir, name), err)
	}
	return info.Mode().IsRegular(), nil
}

// firstPresent names the first of the files the root holds, and is empty when
// it holds none.
func (root repositoryRoot) firstPresent(names []string) (string, error) {
	for _, name := range names {
		isPresent, err := root.has(name)
		if err != nil {
			return "", err
		}
		if isPresent {
			return name, nil
		}
	}
	return "", nil
}

// read returns the content of a file in the root, and an empty string when
// the root does not hold it.
func (root repositoryRoot) read(name string) (string, error) {
	isPresent, err := root.has(name)
	if err != nil || !isPresent {
		return "", err
	}
	content, err := os.ReadFile(filepath.Join(root.dir, name))
	if err != nil {
		return "", fmt.Errorf("could not read %s: %w", filepath.Join(root.dir, name), err)
	}
	return string(content), nil
}

// pins tells whether the tool versions file or the tools table of a mise
// config file names a tool.
func (root repositoryRoot) pins(tool string) (bool, error) {
	toolVersions, err := root.read(toolVersionsFile)
	if err != nil {
		return false, err
	}
	for line := range strings.Lines(toolVersions) {
		line, _, _ = strings.Cut(line, commentPrefix)
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == tool {
			return true, nil
		}
	}

	for _, name := range miseConfigFiles {
		content, err := root.read(name)
		if err != nil {
			return false, err
		}
		if tomlTableHasKey(content, miseToolsTable, tool) {
			return true, nil
		}
	}
	return false, nil
}

// tomlTableHeader returns the name of the table a TOML line opens.
func tomlTableHeader(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "[[") {
		return "", false
	}
	header, _, isClosed := strings.Cut(trimmed[1:], "]")
	return strings.TrimSpace(header), isClosed
}

// tomlTableHasKey tells whether a TOML table sets a key. It reads bare and
// quoted keys and skips dotted keys of the table above it.
func tomlTableHasKey(content string, table string, key string) bool {
	isInTable := false
	for line := range strings.Lines(content) {
		if header, isHeader := tomlTableHeader(line); isHeader {
			isInTable = header == table
			continue
		}
		trimmed := strings.TrimSpace(line)
		if !isInTable || strings.HasPrefix(trimmed, commentPrefix) {
			continue
		}
		name, _, isAssignment := strings.Cut(trimmed, "=")
		if isAssignment && strings.Trim(strings.TrimSpace(name), `"'`) == key {
			return true
		}
	}
	return false
}
