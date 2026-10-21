package setup

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
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
		{
			name:            "a fresh home asks every question but the ones with one option",
			hasGlobalConfig: false,
			wantKeys: []string{
				config.DrudgerHarnessKey,
				config.DrudgerModelKey,
				config.DrudgerMaxConcurrentKey,
				config.DefaultTaskStatusKey,
				config.PullRequestsEnabledKey,
				config.PullRequestsDraftKey,
			},
		},
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

func TestQuestions_Shape(t *testing.T) {
	selectOptions := func(values ...string) []common.QuestionOption {
		var options []common.QuestionOption
		for _, value := range values {
			options = append(options, common.QuestionOption{Value: value, Label: value})
		}
		return options
	}
	testCases := []struct {
		key         string
		wantKind    common.QuestionKind
		wantOptions []common.QuestionOption
		wantDefault string
	}{
		{key: config.DrudgerHarnessKey, wantKind: common.QuestionSelect, wantOptions: selectOptions(string(config.HarnessClaudeCode), string(config.HarnessOpencode)), wantDefault: string(config.HarnessClaudeCode)},
		{key: config.DrudgerModelKey, wantKind: common.QuestionText, wantDefault: ""},
		{key: config.DrudgerMaxConcurrentKey, wantKind: common.QuestionNumber, wantDefault: "3"},
		{key: config.DefaultTaskStatusKey, wantKind: common.QuestionSelect, wantOptions: selectOptions("todo", "draft"), wantDefault: "todo"},
		{key: config.PullRequestsEnabledKey, wantKind: common.QuestionSelect, wantOptions: selectOptions(common.AnswerNo, common.AnswerYes), wantDefault: common.AnswerNo},
		{key: config.PullRequestsDraftKey, wantKind: common.QuestionSelect, wantOptions: selectOptions(common.AnswerNo, common.AnswerYes), wantDefault: common.AnswerNo},
	}

	questions, err := NewSetupService(tempHome(t)).Questions()
	if err != nil {
		t.Fatalf("Questions: %v", err)
	}
	for _, testCase := range testCases {
		t.Run(testCase.key, func(t *testing.T) {
			index := slices.IndexFunc(questions, func(question common.Question) bool { return question.Key == testCase.key })
			if index < 0 {
				t.Fatalf("no question for %s", testCase.key)
			}
			question := questions[index]
			if question.Kind != testCase.wantKind {
				t.Errorf("Kind = %q, want %q", question.Kind, testCase.wantKind)
			}
			if !slices.Equal(question.Options, testCase.wantOptions) {
				t.Errorf("Options = %v, want %v", question.Options, testCase.wantOptions)
			}
			if question.Default != testCase.wantDefault {
				t.Errorf("Default = %q, want %q", question.Default, testCase.wantDefault)
			}
		})
	}
}

func TestQuestions_DraftOnlyAfterPullRequests(t *testing.T) {
	testCases := []struct {
		name               string
		pullRequestsAnswer string
		wantIsAsked        bool
	}{
		{name: "asked after yes", pullRequestsAnswer: common.AnswerYes, wantIsAsked: true},
		{name: "skipped after no", pullRequestsAnswer: common.AnswerNo, wantIsAsked: false},
	}

	questions, err := NewSetupService(tempHome(t)).Questions()
	if err != nil {
		t.Fatalf("Questions: %v", err)
	}
	index := slices.IndexFunc(questions, func(question common.Question) bool { return question.Key == config.PullRequestsDraftKey })
	if index < 0 {
		t.Fatalf("no question for %s", config.PullRequestsDraftKey)
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			answers := common.Answers{config.PullRequestsEnabledKey: testCase.pullRequestsAnswer}
			if got := questions[index].IsAskedAfter(answers); got != testCase.wantIsAsked {
				t.Errorf("IsAskedAfter(%v) = %v, want %v", answers, got, testCase.wantIsAsked)
			}
		})
	}
}

func TestQuestions_ConcurrencyRefusal(t *testing.T) {
	testCases := []struct {
		answer  string
		wantErr string
	}{
		{answer: "0", wantErr: "maxConcurrentDrudgers = 0, it must be a positive number"},
		{answer: "-2", wantErr: "maxConcurrentDrudgers = -2, it must be a positive number"},
		{answer: "many", wantErr: `"many" is not a whole number`},
	}

	questions, err := NewSetupService(tempHome(t)).Questions()
	if err != nil {
		t.Fatalf("Questions: %v", err)
	}
	index := slices.IndexFunc(questions, func(question common.Question) bool { return question.Key == config.DrudgerMaxConcurrentKey })
	if index < 0 {
		t.Fatalf("no question for %s", config.DrudgerMaxConcurrentKey)
	}
	for _, testCase := range testCases {
		t.Run(testCase.answer, func(t *testing.T) {
			err := questions[index].Validate(testCase.answer)
			if err == nil || err.Error() != testCase.wantErr {
				t.Errorf("Validate(%q) = %v, want %q", testCase.answer, err, testCase.wantErr)
			}
		})
	}
}

func TestSetup_Answers(t *testing.T) {
	testCases := []struct {
		name       string
		answers    common.Answers
		wantConfig string
		wantSkill  bool
	}{
		{
			name:    "no answers take the defaults",
			answers: nil,
			wantConfig: `{
				"$schema": "./schema/config.json",
				"drudger": {"environment": "docker-sbx", "harness": "claude-code", "maxConcurrentDrudgers": 3},
				"task": {"defaultStatus": "todo"},
				"remote": {"pullRequests": {"isEnabled": false}}
			}`,
			wantSkill: true,
		},
		{
			name: "every answer changed",
			answers: common.Answers{
				config.DrudgerHarnessKey:       string(config.HarnessOpencode),
				config.DrudgerModelKey:         "opus",
				config.DrudgerMaxConcurrentKey: "5",
				config.DefaultTaskStatusKey:    "draft",
				config.PullRequestsEnabledKey:  common.AnswerYes,
				config.PullRequestsDraftKey:    common.AnswerYes,
			},
			wantConfig: `{
				"$schema": "./schema/config.json",
				"drudger": {"environment": "docker-sbx", "harness": "opencode", "model": "opus", "maxConcurrentDrudgers": 5},
				"task": {"defaultStatus": "draft"},
				"remote": {"provider": "github", "pullRequests": {"isEnabled": true, "isDraft": true}}
			}`,
			wantSkill: false,
		},
		{
			name:    "pull requests on take the provider and the draft default",
			answers: common.Answers{config.PullRequestsEnabledKey: common.AnswerYes},
			wantConfig: `{
				"$schema": "./schema/config.json",
				"drudger": {"environment": "docker-sbx", "harness": "claude-code", "maxConcurrentDrudgers": 3},
				"task": {"defaultStatus": "todo"},
				"remote": {"provider": "github", "pullRequests": {"isEnabled": true, "isDraft": false}}
			}`,
			wantSkill: true,
		},
		{
			name:    "pull requests off drop the draft answer",
			answers: common.Answers{config.PullRequestsEnabledKey: common.AnswerNo, config.PullRequestsDraftKey: common.AnswerYes},
			wantConfig: `{
				"$schema": "./schema/config.json",
				"drudger": {"environment": "docker-sbx", "harness": "claude-code", "maxConcurrentDrudgers": 3},
				"task": {"defaultStatus": "todo"},
				"remote": {"pullRequests": {"isEnabled": false}}
			}`,
			wantSkill: true,
		},
		{
			name:    "an empty model is left out",
			answers: common.Answers{config.DrudgerModelKey: ""},
			wantConfig: `{
				"$schema": "./schema/config.json",
				"drudger": {"environment": "docker-sbx", "harness": "claude-code", "maxConcurrentDrudgers": 3},
				"task": {"defaultStatus": "todo"},
				"remote": {"pullRequests": {"isEnabled": false}}
			}`,
			wantSkill: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			home := tempHome(t)

			result, err := NewSetupService(home).Setup(testCase.answers)
			if err != nil {
				t.Fatalf("Setup: %v", err)
			}

			var got, want any
			if err := common.ReadJSON(result.GlobalConfig.Path, &got); err != nil {
				t.Fatalf("ReadJSON: %v", err)
			}
			if err := json.Unmarshal([]byte(testCase.wantConfig), &want); err != nil {
				t.Fatalf("json.Unmarshal: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("global config = %v, want %v", got, want)
			}
			if _, err := config.Load(); err != nil {
				t.Errorf("config.Load: %v", err)
			}
			if hasSkill := result.SkillPath != ""; hasSkill != testCase.wantSkill {
				t.Errorf("SkillPath = %q, want a skill: %v", result.SkillPath, testCase.wantSkill)
			}
		})
	}
}

func TestSetup_InvalidAnswerWritesNothing(t *testing.T) {
	testCases := []struct {
		name    string
		answers common.Answers
	}{
		{name: "an unknown harness", answers: common.Answers{config.DrudgerHarnessKey: "vim"}},
		{name: "a concurrency limit of zero", answers: common.Answers{config.DrudgerMaxConcurrentKey: "0"}},
		{name: "a concurrency limit that is not a number", answers: common.Answers{config.DrudgerMaxConcurrentKey: "many"}},
		{name: "an unknown task status", answers: common.Answers{config.DefaultTaskStatusKey: "done"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			home := tempHome(t)

			_, err := NewSetupService(home).Setup(testCase.answers)
			if err == nil {
				t.Fatal("Setup succeeded, want an error")
			}
			isPresent, err := common.Exists(common.DrudgeDir(home))
			if err != nil {
				t.Fatalf("Exists: %v", err)
			}
			if isPresent {
				t.Errorf("%s exists after a refused answer", common.DrudgeDir(home))
			}
		})
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
