package setup

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
	"github.com/IgorBolotnikov/DRUDGE/internal/skill"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

type SetupService struct {
	home string
}

func NewSetupService(home string) *SetupService {
	return &SetupService{home: home}
}

// DrudgeDir returns the drudge home directory Setup creates and Cleanup
// removes.
func (s *SetupService) DrudgeDir() string {
	return common.DrudgeDir(s.home)
}

// Labels of the setup questions.
const (
	environmentLabel  = "Environment"
	harnessLabel      = "Harness"
	modelLabel        = "Model the harness runs"
	modelClearLabel   = "Model the harness runs, - for the harness default"
	modelDefaultLabel = "harness default"
	concurrencyLabel  = "Drudgers that may work on one project at once"
	taskStatusLabel   = "Status of a new task"
	providerLabel     = "Provider"
	pullRequestsLabel = "Open pull requests"
	draftLabel        = "Open pull requests as drafts"
	themeLabel        = "Color theme"
)

// modelClearAnswer is the answer to the model question that removes the model
// from the global config.
const modelClearAnswer = "-"

// configFile is a config file setup asks about, with the questions about it.
type configFile struct {
	path      string
	document  map[string]any // Nil when the file is missing
	questions []common.Question
}

// Questions returns the questions setup asks before it writes the configs, in
// the order they are asked. The global config questions come first and the
// theme question last. A question about an existing config starts on the
// value the file holds, and on its default when the file does not have the
// key. It refuses a config file that does not load.
func (s *SetupService) Questions() ([]common.Question, error) {
	globalConfig, themeConfig, err := s.readConfigFiles()
	if err != nil {
		return nil, err
	}
	questions := slices.Concat(globalConfig.questions, themeConfig.questions)
	return slices.DeleteFunc(questions, func(question common.Question) bool { return !question.IsAsked() }), nil
}

// readConfigFiles reads the global config and the theme config and builds the
// questions about them, preselected from the files. It refuses a file its
// loader refuses.
func (s *SetupService) readConfigFiles() (globalConfig configFile, themeConfig configFile, err error) {
	globalConfig, err = readConfigFile(common.GlobalConfigPath(s.home), func(path string) error {
		_, err := config.LoadFile(path)
		return err
	})
	if err != nil {
		return configFile{}, configFile{}, err
	}
	if globalConfig.questions, err = globalConfigQuestions(); err != nil {
		return configFile{}, configFile{}, err
	}
	if err := globalConfig.preselect(); err != nil {
		return configFile{}, configFile{}, err
	}
	for index, question := range globalConfig.questions {
		if question.Key == config.DrudgerModelKey && question.Default != "" {
			globalConfig.questions[index].Label = modelClearLabel
		}
	}

	themeConfig, err = readConfigFile(common.ThemeConfigPath(s.home), func(path string) error {
		_, err := theme.LoadFile(path)
		return err
	})
	if err != nil {
		return configFile{}, configFile{}, err
	}
	themeQuestion, err := newThemeQuestion()
	if err != nil {
		return configFile{}, configFile{}, err
	}
	themeConfig.questions = []common.Question{themeQuestion}
	if err := themeConfig.preselect(); err != nil {
		return configFile{}, configFile{}, err
	}
	return globalConfig, themeConfig, nil
}

// readConfigFile reads the config file at path once load accepts it. A
// missing file reads as no document.
func readConfigFile(path string, load func(path string) error) (configFile, error) {
	isPresent, err := common.Exists(path)
	if err != nil || !isPresent {
		return configFile{path: path}, err
	}
	if err := load(path); err != nil {
		return configFile{}, err
	}
	var document map[string]any
	if err := common.ReadJSON(path, &document); err != nil {
		return configFile{}, err
	}
	if document == nil {
		return configFile{}, fmt.Errorf("%s holds null, it must hold a JSON object", path)
	}
	return configFile{path: path, document: document}, nil
}

// preselect sets the default of every question to the value the file holds
// for its key. It refuses a value the question does not take.
func (file *configFile) preselect() error {
	for index := range file.questions {
		question := &file.questions[index]
		answer, ok := currentAnswer(file.document, question.Key)
		if !ok {
			continue
		}
		if question.Validate != nil {
			if err := question.Validate(answer); err != nil {
				return fmt.Errorf("%s has %s = %q, %w", file.path, question.Key, answer, err)
			}
		}
		question.Default = answer
		if answer != "" {
			question.DefaultLabel = ""
		}
	}
	return nil
}

// globalConfigQuestions returns every question about the global config, the
// ones that are not asked included. The provider is asked only when pull
// requests are on, and the schema offers one provider, so it is taken
// without asking.
func globalConfigQuestions() ([]common.Question, error) {
	schema := config.Schema()
	pullRequestsOn := &common.QuestionCondition{Key: config.PullRequestsEnabledKey, Answer: common.AnswerYes}
	builders := []struct {
		build     func() (common.Question, error)
		condition *common.QuestionCondition
	}{
		{build: func() (common.Question, error) {
			return common.NewSelectQuestion(schema, config.DrudgerEnvironmentKey, environmentLabel)
		}},
		{build: func() (common.Question, error) {
			return common.NewSelectQuestion(schema, config.DrudgerHarnessKey, harnessLabel)
		}},
		{build: func() (common.Question, error) {
			return common.NewTextQuestion(schema, config.DrudgerModelKey, modelLabel, modelDefaultLabel)
		}},
		{build: func() (common.Question, error) {
			return common.NewNumberQuestion(schema, config.DrudgerMaxConcurrentKey, concurrencyLabel, project.ValidateMaxConcurrentDrudgers)
		}},
		{build: func() (common.Question, error) {
			return common.NewSelectQuestion(schema, config.DefaultTaskStatusKey, taskStatusLabel)
		}},
		{build: func() (common.Question, error) {
			return common.NewBoolQuestion(schema, config.PullRequestsEnabledKey, pullRequestsLabel)
		}},
		{build: func() (common.Question, error) {
			return common.NewSelectQuestion(schema, config.RemoteProviderKey, providerLabel)
		}, condition: pullRequestsOn},
		{build: func() (common.Question, error) {
			return common.NewBoolQuestion(schema, config.PullRequestsDraftKey, draftLabel)
		}, condition: pullRequestsOn},
	}
	questions := make([]common.Question, 0, len(builders))
	for _, builder := range builders {
		question, err := builder.build()
		if err != nil {
			return nil, err
		}
		question.Condition = builder.condition
		questions = append(questions, question)
	}
	return questions, nil
}

// newThemeQuestion returns the question about the palette of the theme config,
// with the palettes of the theme schema as its options.
func newThemeQuestion() (common.Question, error) {
	return common.NewSelectQuestion(theme.Schema(), theme.ThemeKey, themeLabel)
}

// newGlobalConfigDocument returns the global config file that holds the
// resolved answers. An empty model is left out, and the provider and the
// draft setting are left out when pull requests are off.
func newGlobalConfigDocument(answers common.Answers) (config.DefaultDocument, error) {
	maxConcurrentDrudgers, err := common.ParseNumberAnswer(answers[config.DrudgerMaxConcurrentKey])
	if err != nil {
		return config.DefaultDocument{}, err
	}
	document := config.DefaultDocument{
		Schema: config.SchemaRef(),
		Drudger: config.DefaultDrudgerDocument{
			Env:                   config.Env(answers[config.DrudgerEnvironmentKey]),
			Harness:               config.Harness(answers[config.DrudgerHarnessKey]),
			Model:                 answers[config.DrudgerModelKey],
			MaxConcurrentDrudgers: maxConcurrentDrudgers,
		},
		Task: config.DefaultTaskDocument{DefaultStatus: task.TaskStatus(answers[config.DefaultTaskStatusKey])},
		Remote: config.DefaultRemoteDocument{
			Provider:     remote.Provider(answers[config.RemoteProviderKey]),
			PullRequests: config.DefaultPullRequestsDocument{IsEnabled: answers[config.PullRequestsEnabledKey] == common.AnswerYes},
		},
	}
	if draftAnswer, ok := answers[config.PullRequestsDraftKey]; ok {
		document.Remote.PullRequests.IsDraft = new(draftAnswer == common.AnswerYes)
	}
	return document, nil
}

// Setup creates the drudge home directory, writes the schema files and
// installs the skill for the harness. The schemas and the skill are
// overwritten on every run. A missing config is created from the answers to
// the setup questions, and an unanswered question takes its default. An
// existing config with no answer in answers is left as it is. Otherwise the
// answered keys are merged into it and every other key stays as it is. Setup
// refuses an invalid answer and a config file that does not load before it
// writes anything.
func (s *SetupService) Setup(answers common.Answers) (*SetupResult, error) {
	globalConfig, themeConfig, err := s.readConfigFiles()
	if err != nil {
		return nil, err
	}
	globalAnswers, err := common.ResolveAnswers(globalConfig.questions, answers)
	if err != nil {
		return nil, err
	}
	if globalAnswers[config.DrudgerModelKey] == modelClearAnswer {
		globalAnswers[config.DrudgerModelKey] = ""
	}
	globalDocument, err := newGlobalConfigDocument(globalAnswers)
	if err != nil {
		return nil, err
	}
	globalWrite, err := globalConfig.plan(answers, globalAnswers, globalDocument)
	if err != nil {
		return nil, err
	}
	themeAnswers, err := common.ResolveAnswers(themeConfig.questions, answers)
	if err != nil {
		return nil, err
	}
	themeWrite, err := themeConfig.plan(answers, themeAnswers, theme.NewDocument(themeAnswers[theme.ThemeKey]))
	if err != nil {
		return nil, err
	}

	drudgeDir := s.DrudgeDir()
	result := &SetupResult{DrudgeDir: drudgeDir}

	if err := common.EnsureDir(common.ProjectsDir(s.home)); err != nil {
		return nil, err
	}

	schemaDir := filepath.Join(drudgeDir, common.SchemaDirName)
	if err := common.EnsureDir(schemaDir); err != nil {
		return nil, err
	}
	schemas := []struct {
		name    string
		content []byte
	}{
		{common.ThemeConfigName, theme.Schema()},
		{common.GloablConfigName, config.Schema()},
		{common.LocalSchemaName, config.LocalSchema()},
	}
	for _, schema := range schemas {
		path := filepath.Join(schemaDir, schema.name)
		if err := common.WriteFile(path, string(schema.content)); err != nil {
			return nil, err
		}
		result.SchemaPaths = append(result.SchemaPaths, path)
	}

	skillPath, didInstall, err := skill.InstallDrudge(s.home, globalDocument.Drudger.Harness)
	if err != nil {
		return nil, err
	}
	if didInstall {
		result.SkillPath = skillPath
	}

	if result.GlobalConfig, err = globalWrite.write(); err != nil {
		return nil, err
	}
	if result.ThemeConfig, err = themeWrite.write(); err != nil {
		return nil, err
	}
	return result, nil
}

// configWrite is what setup writes to one config file.
type configWrite struct {
	path    string
	content any // Nil when the file is left as it is
	outcome ConfigOutcome
}

// plan returns what setup writes to the file. A missing file gets document.
// An existing file is left as it is when answers has no answer for it.
// Otherwise every key of resolved takes its value in document, and a key
// document leaves out is removed from the file.
func (file *configFile) plan(answers common.Answers, resolved common.Answers, document any) (configWrite, error) {
	if file.document == nil {
		return configWrite{path: file.path, content: document, outcome: ConfigCreated}, nil
	}
	unchanged := configWrite{path: file.path, outcome: ConfigUnchanged}
	if !hasAnswerFor(answers, resolved) {
		return unchanged, nil
	}
	answered, err := toJSONObject(document)
	if err != nil {
		return configWrite{}, err
	}
	merged, err := toJSONObject(file.document)
	if err != nil {
		return configWrite{}, err
	}
	for key := range resolved {
		value, ok := lookupKey(answered, key)
		if !ok {
			deleteKey(merged, key)
			continue
		}
		if err := setKey(merged, key, value); err != nil {
			return configWrite{}, fmt.Errorf("%s %w", file.path, err)
		}
	}
	if reflect.DeepEqual(merged, file.document) {
		return unchanged, nil
	}
	return configWrite{path: file.path, content: merged, outcome: ConfigUpdated}, nil
}

func hasAnswerFor(answers common.Answers, resolved common.Answers) bool {
	for key := range resolved {
		if _, ok := answers[key]; ok {
			return true
		}
	}
	return false
}

func (write configWrite) write() (ConfigFile, error) {
	if write.content != nil {
		if err := common.WriteJSON(write.path, write.content); err != nil {
			return ConfigFile{}, err
		}
	}
	return ConfigFile{Path: write.path, Outcome: write.outcome}, nil
}

// IsInstalled reports whether the drudge home directory exists.
func (s *SetupService) IsInstalled() (bool, error) {
	return common.Exists(s.DrudgeDir())
}

// Cleanup removes the drudge home directory and everything in it.
func (s *SetupService) Cleanup() (*CleanupResult, error) {
	drudgeDir := s.DrudgeDir()
	isInstalled, err := s.IsInstalled()
	if err != nil {
		return nil, err
	}
	if !isInstalled {
		return &CleanupResult{DrudgeDir: drudgeDir}, nil
	}
	if err := common.RemoveAll(drudgeDir); err != nil {
		return nil, err
	}
	return &CleanupResult{DrudgeDir: drudgeDir, HasRemoved: true}, nil
}
