package setup

import (
	"path/filepath"
	"slices"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
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
	modelDefaultLabel = "harness default"
	concurrencyLabel  = "Drudgers that may work on one project at once"
	taskStatusLabel   = "Status of a new task"
	providerLabel     = "Provider"
	pullRequestsLabel = "Open pull requests"
	draftLabel        = "Open pull requests as drafts"
)

// Questions returns the questions setup asks before it writes the global
// config, in the order they are asked. It returns none when the global config
// exists.
func (s *SetupService) Questions() ([]common.Question, error) {
	hasGlobalConfig, err := common.Exists(common.GlobalConfigPath(s.home))
	if err != nil || hasGlobalConfig {
		return nil, err
	}
	questions, err := globalConfigQuestions()
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(questions, func(question common.Question) bool { return !question.IsAsked() }), nil
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
			return common.NewNumberQuestion(schema, config.DrudgerMaxConcurrentKey, concurrencyLabel, config.ValidateMaxConcurrentDrudgers)
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
// overwritten on every run. The global config and the theme config are
// written only when they are missing. A missing global config gets the
// answers to the setup questions, and an unanswered question takes its
// default. Setup refuses an invalid answer before it writes anything.
func (s *SetupService) Setup(answers common.Answers) (*SetupResult, error) {
	questions, err := globalConfigQuestions()
	if err != nil {
		return nil, err
	}
	answers, err = common.ResolveAnswers(questions, answers)
	if err != nil {
		return nil, err
	}
	document, err := newGlobalConfigDocument(answers)
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

	harness, err := s.harness(document)
	if err != nil {
		return nil, err
	}
	skillPath, didInstall, err := skill.InstallDrudge(s.home, harness)
	if err != nil {
		return nil, err
	}
	if didInstall {
		result.SkillPath = skillPath
	}

	result.GlobalConfig, err = writeConfigFile(common.GlobalConfigPath(s.home), document)
	if err != nil {
		return nil, err
	}
	result.ThemeConfig, err = writeConfigFile(common.ThemeConfigPath(s.home), theme.NewDefaultDocument())
	if err != nil {
		return nil, err
	}
	return result, nil
}

// harness returns the harness of the global config, or the harness of
// document when there is no global config yet.
func (s *SetupService) harness(document config.DefaultDocument) (config.Harness, error) {
	hasGlobalConfig, err := common.Exists(common.GlobalConfigPath(s.home))
	if err != nil {
		return "", err
	}
	if !hasGlobalConfig {
		return document.Drudger.Harness, nil
	}
	globalConfig, err := config.Load()
	if err != nil {
		return "", err
	}
	return globalConfig.Drudger.Harness, nil
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

func writeConfigFile(path string, document any) (ConfigFile, error) {
	didWrite, err := common.WriteJSONIfNotExists(path, document)
	if err != nil {
		return ConfigFile{}, err
	}
	return ConfigFile{Path: path, HasExisted: !didWrite}, nil
}
