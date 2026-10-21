package setup

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

const staleSchema = "stale"

func tempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func schemaPaths(home string) []string {
	schemaDir := filepath.Join(common.DrudgeDir(home), common.SchemaDirName)
	return []string{
		filepath.Join(schemaDir, common.ThemeConfigName),
		filepath.Join(schemaDir, common.GloablConfigName),
		filepath.Join(schemaDir, common.LocalSchemaName),
	}
}

func themeConfigPath(home string) string {
	return filepath.Join(common.DrudgeDir(home), common.ThemeConfigName)
}

func TestSetup(t *testing.T) {
	testCases := []struct {
		name               string
		runsBefore         int
		wantConfigsExisted bool
	}{
		{name: "fresh home creates every file", runsBefore: 0, wantConfigsExisted: false},
		{name: "second run skips the configs", runsBefore: 1, wantConfigsExisted: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			home := tempHome(t)
			service := NewSetupService(home)
			for range testCase.runsBefore {
				if _, err := service.Setup(nil); err != nil {
					t.Fatalf("Setup before: %v", err)
				}
				for _, path := range schemaPaths(home) {
					if err := os.WriteFile(path, []byte(staleSchema), common.DefaultFilePerm); err != nil {
						t.Fatalf("WriteFile: %v", err)
					}
				}
			}

			result, err := service.Setup(nil)
			if err != nil {
				t.Fatalf("Setup: %v", err)
			}

			if result.DrudgeDir != common.DrudgeDir(home) {
				t.Errorf("DrudgeDir = %q, want %q", result.DrudgeDir, common.DrudgeDir(home))
			}
			if !slices.Equal(result.SchemaPaths, schemaPaths(home)) {
				t.Errorf("SchemaPaths = %v, want %v", result.SchemaPaths, schemaPaths(home))
			}
			for _, path := range result.SchemaPaths {
				content, err := common.ReadFile(path)
				if err != nil {
					t.Fatalf("ReadFile: %v", err)
				}
				if content == staleSchema {
					t.Errorf("%s was not rewritten", path)
				}
			}
			if result.SkillPath == "" {
				t.Errorf("SkillPath is empty, want the installed skill")
			}
			wantConfigs := []struct {
				got  ConfigFile
				want ConfigFile
			}{
				{result.GlobalConfig, ConfigFile{Path: common.GlobalConfigPath(home), HasExisted: testCase.wantConfigsExisted}},
				{result.ThemeConfig, ConfigFile{Path: themeConfigPath(home), HasExisted: testCase.wantConfigsExisted}},
			}
			for _, configFile := range wantConfigs {
				if configFile.got != configFile.want {
					t.Errorf("config file = %+v, want %+v", configFile.got, configFile.want)
				}
			}
			for _, path := range []string{common.ProjectsDir(home), result.SkillPath, result.GlobalConfig.Path, result.ThemeConfig.Path} {
				if _, err := os.Stat(path); err != nil {
					t.Errorf("Stat(%s): %v", path, err)
				}
			}
		})
	}
}

func TestQuestions(t *testing.T) {
	testCases := []struct {
		name            string
		hasGlobalConfig bool
		wantKeys        []string
	}{
		{name: "a fresh home asks for the harness only", hasGlobalConfig: false, wantKeys: []string{config.DrudgerHarnessKey}},
		{name: "an existing global config asks nothing", hasGlobalConfig: true, wantKeys: nil},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			home := tempHome(t)
			service := NewSetupService(home)
			if testCase.hasGlobalConfig {
				if _, err := service.Setup(nil); err != nil {
					t.Fatalf("Setup: %v", err)
				}
			}

			questions, err := service.Questions()
			if err != nil {
				t.Fatalf("Questions: %v", err)
			}
			var gotKeys []string
			for _, question := range questions {
				gotKeys = append(gotKeys, question.Key)
			}
			if !slices.Equal(gotKeys, testCase.wantKeys) {
				t.Errorf("question keys = %v, want %v", gotKeys, testCase.wantKeys)
			}
		})
	}
}

func TestQuestions_Harness(t *testing.T) {
	questions, err := NewSetupService(tempHome(t)).Questions()
	if err != nil {
		t.Fatalf("Questions: %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("got %d questions, want 1", len(questions))
	}
	harness := questions[0]
	wantOptions := []common.QuestionOption{
		{Value: string(config.HarnessClaudeCode), Label: string(config.HarnessClaudeCode)},
		{Value: string(config.HarnessOpencode), Label: string(config.HarnessOpencode)},
	}
	if harness.Kind != common.QuestionSelect {
		t.Errorf("Kind = %q, want %q", harness.Kind, common.QuestionSelect)
	}
	if !slices.Equal(harness.Options, wantOptions) {
		t.Errorf("Options = %v, want %v", harness.Options, wantOptions)
	}
	if harness.Default != string(config.HarnessClaudeCode) {
		t.Errorf("Default = %q, want %q", harness.Default, config.HarnessClaudeCode)
	}
}

func TestSetup_Answers(t *testing.T) {
	testCases := []struct {
		name        string
		answers     common.Answers
		wantHarness config.Harness
		wantSkill   bool
	}{
		{name: "no answers take the defaults", answers: nil, wantHarness: config.HarnessClaudeCode, wantSkill: true},
		{name: "claude code gets the skill", answers: common.Answers{config.DrudgerHarnessKey: string(config.HarnessClaudeCode)}, wantHarness: config.HarnessClaudeCode, wantSkill: true},
		{name: "opencode gets no skill", answers: common.Answers{config.DrudgerHarnessKey: string(config.HarnessOpencode)}, wantHarness: config.HarnessOpencode, wantSkill: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			home := tempHome(t)

			result, err := NewSetupService(home).Setup(testCase.answers)
			if err != nil {
				t.Fatalf("Setup: %v", err)
			}

			var document config.DefaultDocument
			if err := common.ReadJSON(result.GlobalConfig.Path, &document); err != nil {
				t.Fatalf("ReadJSON: %v", err)
			}
			if want := config.NewDocument(config.EnvDockerSbx, testCase.wantHarness); document != want {
				t.Errorf("global config = %+v, want %+v", document, want)
			}
			if hasSkill := result.SkillPath != ""; hasSkill != testCase.wantSkill {
				t.Errorf("SkillPath = %q, want a skill: %v", result.SkillPath, testCase.wantSkill)
			}
		})
	}
}

func TestSetup_InvalidAnswerWritesNothing(t *testing.T) {
	home := tempHome(t)

	_, err := NewSetupService(home).Setup(common.Answers{config.DrudgerHarnessKey: "vim"})
	if err == nil {
		t.Fatal("Setup succeeded, want an error for an unknown harness")
	}
	isPresent, err := common.Exists(common.DrudgeDir(home))
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if isPresent {
		t.Errorf("%s exists after a refused answer", common.DrudgeDir(home))
	}
}

func TestSetup_ExistingGlobalConfigIsLeftAlone(t *testing.T) {
	home := tempHome(t)
	service := NewSetupService(home)
	if _, err := service.Setup(common.Answers{config.DrudgerHarnessKey: string(config.HarnessOpencode)}); err != nil {
		t.Fatalf("Setup before: %v", err)
	}
	before, err := common.ReadFile(common.GlobalConfigPath(home))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	result, err := service.Setup(common.Answers{config.DrudgerHarnessKey: string(config.HarnessClaudeCode)})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	after, err := common.ReadFile(common.GlobalConfigPath(home))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if after != before {
		t.Errorf("global config = %s, want it left as %s", after, before)
	}
	if !result.GlobalConfig.HasExisted {
		t.Errorf("GlobalConfig.HasExisted = false, want true")
	}
	if result.SkillPath != "" {
		t.Errorf("SkillPath = %q, want no skill for the opencode harness on disk", result.SkillPath)
	}
}

func TestSetup_DefaultConfigsLoad(t *testing.T) {
	home := tempHome(t)
	if _, err := NewSetupService(home).Setup(nil); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	if _, err := config.Load(); err != nil {
		t.Errorf("config.Load: %v", err)
	}

	var themeDocument theme.DefaultDocument
	if err := common.ReadJSON(themeConfigPath(home), &themeDocument); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if _, err := theme.Load(themeDocument.Theme); err != nil {
		t.Errorf("theme.Load(%q): %v", themeDocument.Theme, err)
	}
}

func TestSetup_ThemeConfigSharesLoadPath(t *testing.T) {
	home := tempHome(t)
	result, err := NewSetupService(home).Setup(nil)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	const wantTheme = "monokai"
	document := theme.NewDefaultDocument()
	document.Theme = wantTheme
	if err := common.WriteJSON(result.ThemeConfig.Path, document); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	got, err := theme.Load("")
	if err != nil {
		t.Fatalf("theme.Load: %v", err)
	}
	want := theme.NewTheme(wantTheme)
	if !maps.Equal(got.Colors(), want.Colors()) {
		t.Errorf("theme.Load(\"\") = %v, want %v", got.Colors(), want.Colors())
	}
}

func TestCleanup(t *testing.T) {
	testCases := []struct {
		name          string
		isSetUp       bool
		wantInstalled bool
		wantRemoved   bool
	}{
		{name: "removes the drudge home", isSetUp: true, wantInstalled: true, wantRemoved: true},
		{name: "nothing to remove", isSetUp: false, wantInstalled: false, wantRemoved: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			home := tempHome(t)
			service := NewSetupService(home)
			if testCase.isSetUp {
				if _, err := service.Setup(nil); err != nil {
					t.Fatalf("Setup: %v", err)
				}
			}

			isInstalled, err := service.IsInstalled()
			if err != nil {
				t.Fatalf("IsInstalled: %v", err)
			}
			if isInstalled != testCase.wantInstalled {
				t.Errorf("IsInstalled = %v, want %v", isInstalled, testCase.wantInstalled)
			}

			result, err := service.Cleanup()
			if err != nil {
				t.Fatalf("Cleanup: %v", err)
			}
			want := CleanupResult{DrudgeDir: common.DrudgeDir(home), HasRemoved: testCase.wantRemoved}
			if *result != want {
				t.Errorf("Cleanup = %+v, want %+v", *result, want)
			}
			isPresent, err := common.Exists(common.DrudgeDir(home))
			if err != nil {
				t.Fatalf("Exists: %v", err)
			}
			if isPresent {
				t.Errorf("%s still exists after Cleanup", common.DrudgeDir(home))
			}
		})
	}
}
