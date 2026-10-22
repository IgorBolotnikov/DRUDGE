package setup

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
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
		name        string
		runsBefore  int
		wantOutcome ConfigOutcome
	}{
		{name: "fresh home creates every file", runsBefore: 0, wantOutcome: ConfigCreated},
		{name: "second run leaves the configs", runsBefore: 1, wantOutcome: ConfigUnchanged},
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
				{result.GlobalConfig, ConfigFile{Path: common.GlobalConfigPath(home), Outcome: testCase.wantOutcome}},
				{result.ThemeConfig, ConfigFile{Path: themeConfigPath(home), Outcome: testCase.wantOutcome}},
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

// writeConfigFiles writes the global config and the theme config of home.
// An empty content leaves the file missing.
func writeConfigFiles(t *testing.T, home string, globalConfig string, themeConfig string) {
	t.Helper()
	if err := common.EnsureDir(common.DrudgeDir(home)); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	for _, file := range []struct{ path, content string }{
		{common.GlobalConfigPath(home), globalConfig},
		{themeConfigPath(home), themeConfig},
	} {
		if file.content == "" {
			continue
		}
		if err := os.WriteFile(file.path, []byte(file.content), common.DefaultFilePerm); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
}

// pressEnter answers every question the way a frontend does when Enter takes
// each default, with changes typed in for their keys.
func pressEnter(t *testing.T, service *SetupService, changes common.Answers) common.Answers {
	t.Helper()
	questions, err := service.Questions()
	if err != nil {
		t.Fatalf("Questions: %v", err)
	}
	answers := common.Answers{}
	for _, question := range questions {
		if !question.IsAskedAfter(answers) {
			continue
		}
		answer, ok := changes[question.Key]
		if !ok {
			answer = question.Default
		}
		answers[question.Key] = answer
	}
	return answers
}

func TestQuestions(t *testing.T) {
	everyKey := []string{
		config.DrudgerHarnessKey,
		config.DrudgerModelKey,
		config.DrudgerMaxConcurrentKey,
		config.DefaultTaskStatusKey,
		config.PullRequestsEnabledKey,
		config.PullRequestsDraftKey,
		theme.ThemeKey,
	}
	defaults := map[string]string{
		config.DrudgerHarnessKey:       string(config.HarnessClaudeCode),
		config.DrudgerModelKey:         "",
		config.DrudgerMaxConcurrentKey: "3",
		config.DefaultTaskStatusKey:    "todo",
		config.PullRequestsEnabledKey:  common.AnswerNo,
		config.PullRequestsDraftKey:    common.AnswerNo,
		theme.ThemeKey:                 "system",
	}
	testCases := []struct {
		name                  string
		globalConfig          string
		themeConfig           string
		wantDefaults          map[string]string
		wantModelLabel        string
		wantModelDefaultLabel string
	}{
		{
			name:                  "a fresh home starts on the defaults",
			wantDefaults:          defaults,
			wantModelLabel:        modelLabel,
			wantModelDefaultLabel: modelDefaultLabel,
		},
		{
			name: "existing configs start on their values",
			globalConfig: `{
				"drudger": {"harness": "opencode", "model": "opus", "maxConcurrentDrudgers": 5},
				"task": {"defaultStatus": "draft"},
				"remote": {"provider": "github", "pullRequests": {"isEnabled": true, "isDraft": true}}
			}`,
			themeConfig: `{"theme": "nord"}`,
			wantDefaults: map[string]string{
				config.DrudgerHarnessKey:       string(config.HarnessOpencode),
				config.DrudgerModelKey:         "opus",
				config.DrudgerMaxConcurrentKey: "5",
				config.DefaultTaskStatusKey:    "draft",
				config.PullRequestsEnabledKey:  common.AnswerYes,
				config.PullRequestsDraftKey:    common.AnswerYes,
				theme.ThemeKey:                 "nord",
			},
			wantModelLabel:        modelClearLabel,
			wantModelDefaultLabel: "",
		},
		{
			name:                  "keys the configs do not have start on the defaults",
			globalConfig:          `{"drudger": {"harness": "opencode"}}`,
			themeConfig:           `{"overrides": {}}`,
			wantDefaults:          map[string]string{config.DrudgerHarnessKey: string(config.HarnessOpencode)},
			wantModelLabel:        modelLabel,
			wantModelDefaultLabel: modelDefaultLabel,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			home := tempHome(t)
			writeConfigFiles(t, home, testCase.globalConfig, testCase.themeConfig)

			questions, err := NewSetupService(home).Questions()
			if err != nil {
				t.Fatalf("Questions: %v", err)
			}
			var gotKeys []string
			for _, question := range questions {
				gotKeys = append(gotKeys, question.Key)
				wantDefault, ok := testCase.wantDefaults[question.Key]
				if !ok {
					wantDefault = defaults[question.Key]
				}
				if question.Default != wantDefault {
					t.Errorf("Default of %s = %q, want %q", question.Key, question.Default, wantDefault)
				}
				if question.Key != config.DrudgerModelKey {
					continue
				}
				if question.Label != testCase.wantModelLabel {
					t.Errorf("model Label = %q, want %q", question.Label, testCase.wantModelLabel)
				}
				if question.DefaultLabel != testCase.wantModelDefaultLabel {
					t.Errorf("model DefaultLabel = %q, want %q", question.DefaultLabel, testCase.wantModelDefaultLabel)
				}
			}
			if !slices.Equal(gotKeys, everyKey) {
				t.Errorf("question keys = %v, want %v", gotKeys, everyKey)
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
		{key: theme.ThemeKey, wantKind: common.QuestionSelect, wantOptions: selectOptions("system", "nord", "monokai", "catppuccin-mocha", "dracula"), wantDefault: "system"},
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

// The skill paths of each harness, relative to the home directory.
var (
	claudeCodeSkillPath = filepath.Join(".claude", "skills", "DRUDGE", "SKILL.md")
	opencodeSkillPath   = filepath.Join(".config", "opencode", "skills", "DRUDGE", "SKILL.md")
)

func TestSetup_Answers(t *testing.T) {
	testCases := []struct {
		name          string
		answers       common.Answers
		wantConfig    string
		wantSkillPath string
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
			wantSkillPath: claudeCodeSkillPath,
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
			wantSkillPath: opencodeSkillPath,
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
			wantSkillPath: claudeCodeSkillPath,
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
			wantSkillPath: claudeCodeSkillPath,
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
			wantSkillPath: claudeCodeSkillPath,
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
			if wantSkillPath := filepath.Join(home, testCase.wantSkillPath); result.SkillPath != wantSkillPath {
				t.Errorf("SkillPath = %q, want %q", result.SkillPath, wantSkillPath)
			}
			if _, err := os.Stat(result.SkillPath); err != nil {
				t.Errorf("Stat(%s): %v", result.SkillPath, err)
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
		{name: "an unknown theme", answers: common.Answers{theme.ThemeKey: "solarized"}},
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

func TestSetup_ThemeAnswer(t *testing.T) {
	testCases := []struct {
		name       string
		answers    common.Answers
		wantConfig string
	}{
		{
			name:       "no answer takes the system theme",
			answers:    nil,
			wantConfig: `{"$schema": "./schema/theme.json", "theme": "system", "overrides": {}}`,
		},
		{
			name:       "a picked palette",
			answers:    common.Answers{theme.ThemeKey: "dracula"},
			wantConfig: `{"$schema": "./schema/theme.json", "theme": "dracula", "overrides": {}}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := NewSetupService(tempHome(t)).Setup(testCase.answers)
			if err != nil {
				t.Fatalf("Setup: %v", err)
			}

			var got, want any
			if err := common.ReadJSON(result.ThemeConfig.Path, &got); err != nil {
				t.Fatalf("ReadJSON: %v", err)
			}
			if err := json.Unmarshal([]byte(testCase.wantConfig), &want); err != nil {
				t.Fatalf("json.Unmarshal: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("theme config = %v, want %v", got, want)
			}
		})
	}
}

// assertJSONFile fails the test when the file at path does not hold the JSON
// of want.
func assertJSONFile(t *testing.T, path string, want string) {
	t.Helper()
	var got, wantValue any
	if err := common.ReadJSON(path, &got); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, wantValue) {
		t.Errorf("%s = %v, want %v", path, got, wantValue)
	}
}

func TestSetup_Rerun(t *testing.T) {
	const setupGlobalConfig = `{
		"$schema": "./schema/config.json",
		"drudger": {"environment": "docker-sbx", "harness": "claude-code", "model": "opus", "maxConcurrentDrudgers": 3},
		"task": {"defaultStatus": "todo"},
		"remote": {"provider": "github", "pullRequests": {"isEnabled": true, "isDraft": false}}
	}`
	const setupThemeConfig = `{"$schema": "./schema/theme.json", "theme": "nord", "overrides": {"error": "#FF5555"}}`
	testCases := []struct {
		name              string
		globalConfig      string
		themeConfig       string
		changes           common.Answers
		wantGlobalConfig  string
		wantThemeConfig   string
		wantGlobalOutcome ConfigOutcome
		wantThemeOutcome  ConfigOutcome
	}{
		{
			name:              "Enter on every question leaves the files",
			globalConfig:      setupGlobalConfig,
			themeConfig:       setupThemeConfig,
			wantGlobalConfig:  setupGlobalConfig,
			wantThemeConfig:   setupThemeConfig,
			wantGlobalOutcome: ConfigUnchanged,
			wantThemeOutcome:  ConfigUnchanged,
		},
		{
			name: "unknown keys and keys setup does not ask stay",
			globalConfig: `{
				"$schema": "./custom/config.json",
				"custom": {"keep": true},
				"drudger": {"harness": "claude-code", "sandboxTimeouts": {"createSeconds": 900}},
				"project": {"pageSize": 50}
			}`,
			themeConfig: `{"$schema": "./custom/theme.json", "custom": 1, "theme": "nord", "overrides": {"error": "#FF5555"}}`,
			changes:     common.Answers{config.DrudgerHarnessKey: string(config.HarnessOpencode), theme.ThemeKey: "dracula"},
			wantGlobalConfig: `{
				"$schema": "./custom/config.json",
				"custom": {"keep": true},
				"drudger": {"environment": "docker-sbx", "harness": "opencode", "maxConcurrentDrudgers": 3, "sandboxTimeouts": {"createSeconds": 900}},
				"task": {"defaultStatus": "todo"},
				"project": {"pageSize": 50},
				"remote": {"pullRequests": {"isEnabled": false}}
			}`,
			wantThemeConfig:   `{"$schema": "./custom/theme.json", "custom": 1, "theme": "dracula", "overrides": {"error": "#FF5555"}}`,
			wantGlobalOutcome: ConfigUpdated,
			wantThemeOutcome:  ConfigUpdated,
		},
		{
			name:         "- removes the model",
			globalConfig: setupGlobalConfig,
			themeConfig:  setupThemeConfig,
			changes:      common.Answers{config.DrudgerModelKey: modelClearAnswer},
			wantGlobalConfig: `{
				"$schema": "./schema/config.json",
				"drudger": {"environment": "docker-sbx", "harness": "claude-code", "maxConcurrentDrudgers": 3},
				"task": {"defaultStatus": "todo"},
				"remote": {"provider": "github", "pullRequests": {"isEnabled": true, "isDraft": false}}
			}`,
			wantThemeConfig:   setupThemeConfig,
			wantGlobalOutcome: ConfigUpdated,
			wantThemeOutcome:  ConfigUnchanged,
		},
		{
			name:         "a typed model replaces the model",
			globalConfig: setupGlobalConfig,
			themeConfig:  setupThemeConfig,
			changes:      common.Answers{config.DrudgerModelKey: "sonnet"},
			wantGlobalConfig: `{
				"$schema": "./schema/config.json",
				"drudger": {"environment": "docker-sbx", "harness": "claude-code", "model": "sonnet", "maxConcurrentDrudgers": 3},
				"task": {"defaultStatus": "todo"},
				"remote": {"provider": "github", "pullRequests": {"isEnabled": true, "isDraft": false}}
			}`,
			wantThemeConfig:   setupThemeConfig,
			wantGlobalOutcome: ConfigUpdated,
			wantThemeOutcome:  ConfigUnchanged,
		},
		{
			name:         "pull requests off keep the provider and the draft setting",
			globalConfig: setupGlobalConfig,
			themeConfig:  setupThemeConfig,
			changes:      common.Answers{config.PullRequestsEnabledKey: common.AnswerNo},
			wantGlobalConfig: `{
				"$schema": "./schema/config.json",
				"drudger": {"environment": "docker-sbx", "harness": "claude-code", "model": "opus", "maxConcurrentDrudgers": 3},
				"task": {"defaultStatus": "todo"},
				"remote": {"provider": "github", "pullRequests": {"isEnabled": false, "isDraft": false}}
			}`,
			wantThemeConfig:   setupThemeConfig,
			wantGlobalOutcome: ConfigUpdated,
			wantThemeOutcome:  ConfigUnchanged,
		},
		{
			name:         "pull requests on add the provider and the draft setting",
			globalConfig: `{"drudger": {"harness": "claude-code"}}`,
			changes:      common.Answers{config.PullRequestsEnabledKey: common.AnswerYes, config.PullRequestsDraftKey: common.AnswerYes},
			wantGlobalConfig: `{
				"drudger": {"environment": "docker-sbx", "harness": "claude-code", "maxConcurrentDrudgers": 3},
				"task": {"defaultStatus": "todo"},
				"remote": {"provider": "github", "pullRequests": {"isEnabled": true, "isDraft": true}}
			}`,
			wantThemeConfig:   `{"$schema": "./schema/theme.json", "theme": "system", "overrides": {}}`,
			wantGlobalOutcome: ConfigUpdated,
			wantThemeOutcome:  ConfigCreated,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			home := tempHome(t)
			writeConfigFiles(t, home, testCase.globalConfig, testCase.themeConfig)
			service := NewSetupService(home)
			before, err := common.ReadFile(common.GlobalConfigPath(home))
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}

			result, err := service.Setup(pressEnter(t, service, testCase.changes))
			if err != nil {
				t.Fatalf("Setup: %v", err)
			}

			assertJSONFile(t, common.GlobalConfigPath(home), testCase.wantGlobalConfig)
			assertJSONFile(t, themeConfigPath(home), testCase.wantThemeConfig)
			if result.GlobalConfig.Outcome != testCase.wantGlobalOutcome {
				t.Errorf("GlobalConfig.Outcome = %q, want %q", result.GlobalConfig.Outcome, testCase.wantGlobalOutcome)
			}
			if result.ThemeConfig.Outcome != testCase.wantThemeOutcome {
				t.Errorf("ThemeConfig.Outcome = %q, want %q", result.ThemeConfig.Outcome, testCase.wantThemeOutcome)
			}
			after, err := common.ReadFile(common.GlobalConfigPath(home))
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if isRewritten := after != before; isRewritten != (testCase.wantGlobalOutcome == ConfigUpdated) {
				t.Errorf("global config rewritten: %v, want outcome %q", isRewritten, testCase.wantGlobalOutcome)
			}
			if _, err := config.Load(); err != nil {
				t.Errorf("config.Load: %v", err)
			}
		})
	}
}

func TestSetup_DefaultsLeaveExistingFiles(t *testing.T) {
	const globalConfig = `{"drudger":   {"harness": "opencode", "model": "opus"},
  "custom": 1}`
	const themeConfig = `{"theme": "nord"}`
	testCases := []struct {
		name              string
		globalConfig      string
		themeConfig       string
		wantGlobalOutcome ConfigOutcome
		wantThemeOutcome  ConfigOutcome
		wantSkillPath     string
	}{
		{name: "both files exist", globalConfig: globalConfig, themeConfig: themeConfig, wantGlobalOutcome: ConfigUnchanged, wantThemeOutcome: ConfigUnchanged, wantSkillPath: opencodeSkillPath},
		{name: "a missing theme config is created", globalConfig: globalConfig, wantGlobalOutcome: ConfigUnchanged, wantThemeOutcome: ConfigCreated, wantSkillPath: opencodeSkillPath},
		{name: "a missing global config is created", themeConfig: themeConfig, wantGlobalOutcome: ConfigCreated, wantThemeOutcome: ConfigUnchanged, wantSkillPath: claudeCodeSkillPath},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			home := tempHome(t)
			writeConfigFiles(t, home, testCase.globalConfig, testCase.themeConfig)

			result, err := NewSetupService(home).Setup(nil)
			if err != nil {
				t.Fatalf("Setup: %v", err)
			}

			files := []struct {
				got         ConfigFile
				before      string
				wantOutcome ConfigOutcome
			}{
				{result.GlobalConfig, testCase.globalConfig, testCase.wantGlobalOutcome},
				{result.ThemeConfig, testCase.themeConfig, testCase.wantThemeOutcome},
			}
			for _, file := range files {
				if file.got.Outcome != file.wantOutcome {
					t.Errorf("Outcome of %s = %q, want %q", file.got.Path, file.got.Outcome, file.wantOutcome)
				}
				if file.wantOutcome != ConfigUnchanged {
					continue
				}
				after, err := common.ReadFile(file.got.Path)
				if err != nil {
					t.Fatalf("ReadFile: %v", err)
				}
				if after != file.before {
					t.Errorf("%s = %q, want it left as %q", file.got.Path, after, file.before)
				}
			}
			if wantSkillPath := filepath.Join(home, testCase.wantSkillPath); result.SkillPath != wantSkillPath {
				t.Errorf("SkillPath = %q, want %q", result.SkillPath, wantSkillPath)
			}
		})
	}
}

func TestSetup_BrokenConfigWritesNothing(t *testing.T) {
	testCases := []struct {
		name         string
		globalConfig string
		themeConfig  string
		wantErr      string
	}{
		{name: "a global config that is not JSON", globalConfig: `{"drudger": `, wantErr: "could not parse global config"},
		{name: "a value the config loader refuses", globalConfig: `{"drudger": {"maxConcurrentDrudgers": -1}}`, wantErr: "it must be a positive number"},
		{name: "a value the question refuses", globalConfig: `{"drudger": {"harness": "vim"}}`, wantErr: `has drudger.harness = "vim"`},
		{name: "a theme config that is not JSON", themeConfig: `{"theme": `, wantErr: "could not parse theme config"},
		{name: "an unknown theme", themeConfig: `{"theme": "solarized"}`, wantErr: `unknown theme "solarized"`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			home := tempHome(t)
			writeConfigFiles(t, home, testCase.globalConfig, testCase.themeConfig)
			service := NewSetupService(home)

			if _, err := service.Questions(); err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Errorf("Questions error = %v, want it to contain %q", err, testCase.wantErr)
			}
			_, err := service.Setup(common.Answers{config.DrudgerHarnessKey: string(config.HarnessClaudeCode), theme.ThemeKey: "dracula"})
			if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Errorf("Setup error = %v, want it to contain %q", err, testCase.wantErr)
			}

			entries, err := os.ReadDir(common.DrudgeDir(home))
			if err != nil {
				t.Fatalf("ReadDir: %v", err)
			}
			var gotNames []string
			for _, entry := range entries {
				gotNames = append(gotNames, entry.Name())
			}
			var wantNames []string
			for _, file := range []struct{ name, content string }{
				{common.GloablConfigName, testCase.globalConfig},
				{common.ThemeConfigName, testCase.themeConfig},
			} {
				if file.content != "" {
					wantNames = append(wantNames, file.name)
				}
			}
			slices.Sort(wantNames)
			if !slices.Equal(gotNames, wantNames) {
				t.Errorf("drudge home holds %v, want only %v", gotNames, wantNames)
			}
			for _, file := range []struct{ path, content string }{
				{common.GlobalConfigPath(home), testCase.globalConfig},
				{themeConfigPath(home), testCase.themeConfig},
			} {
				if file.content == "" {
					continue
				}
				after, err := common.ReadFile(file.path)
				if err != nil {
					t.Fatalf("ReadFile: %v", err)
				}
				if after != file.content {
					t.Errorf("%s = %q, want it left as %q", file.path, after, file.content)
				}
			}
		})
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
	document := theme.NewDocument(wantTheme)
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
