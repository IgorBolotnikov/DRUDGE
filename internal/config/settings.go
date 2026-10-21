package config

import (
	"fmt"
	"strconv"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

// GlobalSettingsReader implements project.GlobalSettings over the global
// config file.
type GlobalSettingsReader struct {
	home string
}

// NewGlobalSettingsReader returns a reader of the global config file under the
// drudge home directory of home.
func NewGlobalSettingsReader(home string) *GlobalSettingsReader {
	return &GlobalSettingsReader{home: home}
}

// LocalSchema returns the bundled JSON schema of the local config.
func (reader *GlobalSettingsReader) LocalSchema() []byte {
	return LocalSchema()
}

// ResolvedAnswers loads the global config and returns the answer for every
// local config key of keys that matches the value the global config resolves
// to. It refuses a global config that does not load and a key it cannot
// resolve.
func (reader *GlobalSettingsReader) ResolvedAnswers(keys []string) (common.Answers, error) {
	global, err := LoadFile(common.GlobalConfigPath(reader.home))
	if err != nil {
		return nil, err
	}
	pullRequests := global.Remote.PullRequests
	answers := make(common.Answers, len(keys))
	for _, key := range keys {
		switch key {
		case maxConcurrentDrudgersKey:
			answers[key] = strconv.Itoa(ResolveMaxConcurrentDrudgers(&LocalConfig{}, global))
		case branchFormatKey:
			answers[key] = ResolveBranchFormat(&LocalConfig{}, global)
		case remoteProviderKey:
			answers[key] = string(global.Remote.Provider)
		case PullRequestsEnabledKey:
			answers[key] = common.FormatBoolAnswer(*firstSet(pullRequests.IsEnabled, new(false)))
		case PullRequestsDraftKey:
			answers[key] = common.FormatBoolAnswer(*firstSet(pullRequests.IsDraft, new(false)))
		default:
			return nil, fmt.Errorf("the global config cannot resolve %s", key)
		}
	}
	return answers, nil
}
