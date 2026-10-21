package config

import (
	"os"
	"reflect"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

func TestGlobalSettingsReader_ResolvedAnswers(t *testing.T) {
	keys := []string{maxConcurrentDrudgersKey, PullRequestsEnabledKey, PullRequestsDraftKey, RemoteProviderKey}
	tests := []struct {
		name string
		// raw is the global config file. Empty leaves the file missing.
		raw         string
		keys        []string
		wantAnswers common.Answers
		wantErr     bool
	}{
		{
			name: "no global config",
			keys: keys,
			wantAnswers: common.Answers{
				maxConcurrentDrudgersKey: "3",
				PullRequestsEnabledKey:   common.AnswerNo,
				PullRequestsDraftKey:     common.AnswerNo,
				RemoteProviderKey:        "",
			},
		},
		{
			name: "a global config with its own values",
			raw:  `{"drudger": {"maxConcurrentDrudgers": 5}, "remote": {"provider": "github", "pullRequests": {"isEnabled": true, "isDraft": true}}}`,
			keys: keys,
			wantAnswers: common.Answers{
				maxConcurrentDrudgersKey: "5",
				PullRequestsEnabledKey:   common.AnswerYes,
				PullRequestsDraftKey:     common.AnswerYes,
				RemoteProviderKey:        "github",
			},
		},
		{
			name:    "a key the global config cannot resolve",
			keys:    []string{DrudgerHarnessKey},
			wantErr: true,
		},
		{
			name:    "a global config that does not load",
			raw:     `{"drudger": {"maxConcurrentDrudgers": -1}}`,
			keys:    keys,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := setupHome(t)
			if test.raw != "" {
				if err := common.EnsureDir(common.DrudgeDir(home)); err != nil {
					t.Fatalf("could not create drudge dir: %v", err)
				}
				if err := os.WriteFile(common.GlobalConfigPath(home), []byte(test.raw), common.DefaultFilePerm); err != nil {
					t.Fatalf("could not write config: %v", err)
				}
			}

			answers, err := NewGlobalSettingsReader(home).ResolvedAnswers(test.keys)

			if test.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %v", answers)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(answers, test.wantAnswers) {
				t.Errorf("answers = %v, want %v", answers, test.wantAnswers)
			}
		})
	}
}
