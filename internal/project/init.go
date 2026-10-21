package project

import (
	"fmt"
	"slices"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// MaxConcurrentDrudgersKey is the local config key of the Drudger limit of a
// project.
const MaxConcurrentDrudgersKey = "maxConcurrentDrudgers"

// Labels of the init questions.
const (
	concurrencyLabel        = "Drudgers that may work on this project at once"
	branchFormatLabel       = "Branch format of tasks"
	customBranchFormatLabel = "Custom branch format"
	pullRequestsLabel       = "Open pull requests"
	draftLabel              = "Open pull requests as drafts"
)

// Custom branch formats are asked for by a typed question that follows the
// select of presets.
const (
	customBranchFormatKey    = task.BranchFormatKey + ".custom"
	customBranchFormatAnswer = "custom"
	customBranchFormatOption = "Custom…"
)

// branchFormatPresets are the branch formats init offers to pick from.
var branchFormatPresets = []string{
	task.DefaultBranchFormat,
	task.BranchPlaceholderTicketID + "-" + task.BranchPlaceholderSlug,
	"feat/" + task.BranchPlaceholderTicketID + "/" + task.BranchPlaceholderSlug,
}

// Answers of the setup question of a repository.
const (
	keepSetupAnswer  = "keep"
	leaveSetupAnswer = "leave empty for later"
)

// setupQuestionKeyPrefix starts the key of the setup question of a
// repository, followed by its path.
const setupQuestionKeyPrefix = "setup:"

// initKeys are the local config keys init reads from the global config. The
// provider is never asked. InitProject writes it when pull requests are
// turned on with no provider in the global config.
var initKeys = []string{MaxConcurrentDrudgersKey, task.BranchFormatKey, remote.PullRequestsEnabledKey, remote.PullRequestsDraftKey, remote.ProviderKey}

// PreparedInit is a project init with the repositories found and their setup
// detected, and the questions to ask before InitProject writes anything.
type PreparedInit struct {
	Name         string
	Repositories []Repository
	Detections   []SetupDetection
	// Questions start on the values the global config resolves to.
	Questions []common.Question
	// SetupQuestions ask whether to keep the detected setup of each
	// repository that has one, in config order.
	SetupQuestions []common.Question
	// globalAnswers are the answers that match the values the global config
	// resolves to, keyed by local config key.
	globalAnswers common.Answers
}

// ValidateMaxConcurrentDrudgers rejects a Drudger limit below 1.
func ValidateMaxConcurrentDrudgers(value int) error {
	if value < 1 {
		return fmt.Errorf("%s = %d, it must be a positive number", MaxConcurrentDrudgersKey, value)
	}
	return nil
}

// PrepareInit finds the repositories of projectDir, detects the setup and
// copy paths of each of them and the environment variable names they need,
// and builds the init questions. It writes nothing. It refuses an empty
// name, a name another project goes by and a directory holding no
// repository.
func (p *ProjectService) PrepareInit(name string, projectDir string) (*PreparedInit, error) {
	if err := p.refuseNewName(name); err != nil {
		return nil, err
	}
	repositories, err := p.DiscoverRepositories(projectDir)
	if err != nil {
		return nil, err
	}

	detections, setups, err := p.proposeSetups(projectDir, repositories, nil, false)
	if err != nil {
		return nil, err
	}
	for index, repository := range repositories {
		repositories[index].Setup = setups[repository.Path].Setup
		repositories[index].Copy = setups[repository.Path].Copy
	}

	globalAnswers, err := p.globals.ResolvedAnswers(initKeys)
	if err != nil {
		return nil, err
	}
	questions, err := initQuestions(p.globals.LocalSchema(), globalAnswers)
	if err != nil {
		return nil, err
	}
	return &PreparedInit{
		Name:           name,
		Repositories:   repositories,
		Detections:     detections,
		Questions:      questions,
		SetupQuestions: setupQuestions(detections),
		globalAnswers:  globalAnswers,
	}, nil
}

// InitProject creates the project of prepared and links the current
// directory to it through the DirectoryLinker, together with the
// repositories, their setup and copy paths and the environment variable names
// they need. A question missing from answers takes its default. Only an
// answer that differs from the value the global config resolves to is
// written into the local config. A custom branch format is written as the
// branch format. Pull requests turned on with no provider in
// the global config write the provider too. A repository whose detected
// setup is left for later gets no setup and no copy paths. It refuses an
// invalid answer before it writes anything.
func (p *ProjectService) InitProject(prepared *PreparedInit, answers common.Answers) (*Project, error) {
	resolved, err := common.ResolveAnswers(slices.Concat(prepared.Questions, prepared.SetupQuestions), answers)
	if err != nil {
		return nil, err
	}
	repositories := slices.Clone(prepared.Repositories)
	for index, repository := range repositories {
		key := setupQuestionKey(repository.Path)
		if resolved[key] == leaveSetupAnswer {
			repositories[index].Setup = nil
			repositories[index].Copy = nil
		}
		delete(resolved, key)
	}
	if resolved[task.BranchFormatKey] == customBranchFormatAnswer {
		resolved[task.BranchFormatKey] = resolved[customBranchFormatKey]
	}
	delete(resolved, customBranchFormatKey)
	overrides := common.Answers{}
	for key, answer := range resolved {
		if answer != prepared.globalAnswers[key] {
			overrides[key] = answer
		}
	}
	if resolved[remote.PullRequestsEnabledKey] == common.AnswerYes && prepared.globalAnswers[remote.ProviderKey] == "" {
		overrides[remote.ProviderKey] = string(remote.ProviderGitHub)
	}

	created, err := p.CreateProject(prepared.Name)
	if err != nil {
		return nil, err
	}
	if err := p.linker.LinkDirectory(created.Slug, repositories, addedEnvNames(prepared.Detections), overrides); err != nil {
		return nil, err
	}
	return created, nil
}

// initQuestions returns the questions init asks, built from the local config
// schema and starting on the global answers. The draft question is asked only
// when pull requests are on.
func initQuestions(schema []byte, globalAnswers common.Answers) ([]common.Question, error) {
	concurrency, err := common.NewNumberQuestion(schema, MaxConcurrentDrudgersKey, concurrencyLabel, ValidateMaxConcurrentDrudgers)
	if err != nil {
		return nil, err
	}
	concurrency.Default = globalAnswers[MaxConcurrentDrudgersKey]
	branchFormat, customBranchFormat, err := branchFormatQuestions(schema, globalAnswers[task.BranchFormatKey])
	if err != nil {
		return nil, err
	}
	pullRequests, err := common.NewBoolQuestion(schema, remote.PullRequestsEnabledKey, pullRequestsLabel)
	if err != nil {
		return nil, err
	}
	pullRequests.Default = globalAnswers[remote.PullRequestsEnabledKey]
	draft, err := common.NewBoolQuestion(schema, remote.PullRequestsDraftKey, draftLabel)
	if err != nil {
		return nil, err
	}
	draft.Default = globalAnswers[remote.PullRequestsDraftKey]
	draft.Condition = &common.QuestionCondition{Key: remote.PullRequestsEnabledKey, Answer: common.AnswerYes}
	return []common.Question{concurrency, branchFormat, customBranchFormat, pullRequests, draft}, nil
}

// setupQuestions returns a select of keeping or leaving for later the
// detected setup of each repository with setup lines, starting on keeping it.
func setupQuestions(detections []SetupDetection) []common.Question {
	var questions []common.Question
	for _, detection := range detections {
		if len(detection.Lines) == 0 {
			continue
		}
		question := common.Question{
			Key:     setupQuestionKey(detection.Repository),
			Label:   fmt.Sprintf("Setup of %s", detection.Repository),
			Kind:    common.QuestionSelect,
			Options: []common.QuestionOption{{Value: keepSetupAnswer, Label: keepSetupAnswer}, {Value: leaveSetupAnswer, Label: leaveSetupAnswer}},
			Default: keepSetupAnswer,
		}
		question.Validate = question.ValidateOption
		questions = append(questions, question)
	}
	return questions
}

func setupQuestionKey(repositoryPath string) string {
	return setupQuestionKeyPrefix + repositoryPath
}

// branchFormatQuestions returns the select of the branch format presets and
// the typed question a pick of the custom option asks. The select starts on
// the preset equal to globalFormat, and on the custom option when no preset
// is. The typed question starts on globalFormat and refuses a format the
// config loader refuses.
func branchFormatQuestions(schema []byte, globalFormat string) (common.Question, common.Question, error) {
	custom, err := common.NewTextQuestion(schema, task.BranchFormatKey, customBranchFormatLabel, "")
	if err != nil {
		return common.Question{}, common.Question{}, err
	}
	custom.Key = customBranchFormatKey
	custom.Default = globalFormat
	custom.Condition = &common.QuestionCondition{Key: task.BranchFormatKey, Answer: customBranchFormatAnswer}
	custom.Validate = task.ValidateBranchFormat

	presets := common.Question{Key: task.BranchFormatKey, Label: branchFormatLabel, Kind: common.QuestionSelect, Default: customBranchFormatAnswer}
	for _, format := range branchFormatPresets {
		presets.Options = append(presets.Options, common.QuestionOption{Value: format, Label: format})
		if format == globalFormat {
			presets.Default = format
		}
	}
	presets.Options = append(presets.Options, common.QuestionOption{Value: customBranchFormatAnswer, Label: customBranchFormatOption})
	presets.Validate = presets.ValidateOption
	return presets, custom, nil
}
