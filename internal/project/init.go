package project

import (
	"fmt"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
)

// MaxConcurrentDrudgersKey is the local config key of the Drudger limit of a
// project.
const MaxConcurrentDrudgersKey = "maxConcurrentDrudgers"

// Labels of the init questions.
const (
	concurrencyLabel  = "Drudgers that may work on this project at once"
	pullRequestsLabel = "Open pull requests"
	draftLabel        = "Open pull requests as drafts"
)

// initKeys are the local config keys init reads from the global config. The
// provider is never asked. InitProject writes it when pull requests are
// turned on with no provider in the global config.
var initKeys = []string{MaxConcurrentDrudgersKey, remote.PullRequestsEnabledKey, remote.PullRequestsDraftKey, remote.ProviderKey}

// PreparedInit is a project init with the repositories found and their setup
// detected, and the questions to ask before InitProject writes anything.
type PreparedInit struct {
	Name         string
	Repositories []Repository
	Detections   []SetupDetection
	// Questions start on the values the global config resolves to.
	Questions []common.Question
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
	questions, err := initQuestions(p.globals.LocalSchema())
	if err != nil {
		return nil, err
	}
	for index := range questions {
		questions[index].Default = globalAnswers[questions[index].Key]
	}
	return &PreparedInit{
		Name:          name,
		Repositories:  repositories,
		Detections:    detections,
		Questions:     questions,
		globalAnswers: globalAnswers,
	}, nil
}

// InitProject creates the project of prepared and links the current
// directory to it through the DirectoryLinker, together with the
// repositories, their setup and copy paths and the environment variable names
// they need. A question missing from answers takes its default. Only an
// answer that differs from the value the global config resolves to is
// written into the local config. Pull requests turned on with no provider in
// the global config write the provider too. It refuses an invalid answer
// before it writes anything.
func (p *ProjectService) InitProject(prepared *PreparedInit, answers common.Answers) (*Project, error) {
	resolved, err := common.ResolveAnswers(prepared.Questions, answers)
	if err != nil {
		return nil, err
	}
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
	if err := p.linker.LinkDirectory(created.Slug, prepared.Repositories, addedEnvNames(prepared.Detections), overrides); err != nil {
		return nil, err
	}
	return created, nil
}

// initQuestions returns the questions init asks, built from the local config
// schema. The draft question is asked only when pull requests are on.
func initQuestions(schema []byte) ([]common.Question, error) {
	concurrency, err := common.NewNumberQuestion(schema, MaxConcurrentDrudgersKey, concurrencyLabel, ValidateMaxConcurrentDrudgers)
	if err != nil {
		return nil, err
	}
	pullRequests, err := common.NewBoolQuestion(schema, remote.PullRequestsEnabledKey, pullRequestsLabel)
	if err != nil {
		return nil, err
	}
	draft, err := common.NewBoolQuestion(schema, remote.PullRequestsDraftKey, draftLabel)
	if err != nil {
		return nil, err
	}
	draft.Condition = &common.QuestionCondition{Key: remote.PullRequestsEnabledKey, Answer: common.AnswerYes}
	return []common.Question{concurrency, pullRequests, draft}, nil
}
