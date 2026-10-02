package config

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
)

func TestResolveRemote(t *testing.T) {
	tests := []struct {
		name   string
		local  RemoteConfig
		global RemoteConfig
		want   RemoteSettings
		// wantErrText lists fragments the error must carry. A test with none
		// expects no error.
		wantErrText []string
	}{
		{
			name: "defaults",
			want: RemoteSettings{Timeout: 60 * time.Second},
		},
		{
			name: "global only",
			global: RemoteConfig{
				Provider:       remote.ProviderGitHub,
				TimeoutSeconds: 30,
				PullRequests:   PullRequestsConfig{IsEnabled: new(true), IsDraft: new(true)},
			},
			want: RemoteSettings{
				Provider:     remote.ProviderGitHub,
				Timeout:      30 * time.Second,
				PullRequests: PullRequestSettings{IsEnabled: true, IsDraft: true},
			},
		},
		{
			name:   "local provider",
			local:  RemoteConfig{Provider: remote.ProviderGitHub},
			global: RemoteConfig{PullRequests: PullRequestsConfig{IsEnabled: new(true)}},
			want: RemoteSettings{
				Provider:     remote.ProviderGitHub,
				Timeout:      60 * time.Second,
				PullRequests: PullRequestSettings{IsEnabled: true},
			},
		},
		{
			name:   "local timeout",
			local:  RemoteConfig{TimeoutSeconds: 90},
			global: RemoteConfig{TimeoutSeconds: 30},
			want:   RemoteSettings{Timeout: 90 * time.Second},
		},
		{
			name:   "local turns pull requests off",
			local:  RemoteConfig{PullRequests: PullRequestsConfig{IsEnabled: new(false)}},
			global: RemoteConfig{PullRequests: PullRequestsConfig{IsEnabled: new(true)}},
			want:   RemoteSettings{Timeout: 60 * time.Second},
		},
		{
			name:   "local turns pull requests on",
			local:  RemoteConfig{PullRequests: PullRequestsConfig{IsEnabled: new(true)}},
			global: RemoteConfig{Provider: remote.ProviderGitHub, PullRequests: PullRequestsConfig{IsEnabled: new(false)}},
			want: RemoteSettings{
				Provider:     remote.ProviderGitHub,
				Timeout:      60 * time.Second,
				PullRequests: PullRequestSettings{IsEnabled: true},
			},
		},
		{
			name:   "local turns drafts off",
			local:  RemoteConfig{PullRequests: PullRequestsConfig{IsDraft: new(false)}},
			global: RemoteConfig{PullRequests: PullRequestsConfig{IsDraft: new(true)}},
			want:   RemoteSettings{Timeout: 60 * time.Second},
		},
		{
			name:        "pull requests on with no provider",
			global:      RemoteConfig{PullRequests: PullRequestsConfig{IsEnabled: new(true)}},
			wantErrText: []string{remote.PullRequestsEnabledKey, remoteProviderKey},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ResolveRemote(&LocalConfig{Remote: test.local}, &GlobalConfig{Remote: test.global})
			if len(test.wantErrText) > 0 {
				assertErrorNames(t, err, test.wantErrText)
				return
			}
			if err != nil {
				t.Fatalf("ResolveRemote: %v", err)
			}
			if got != test.want {
				t.Errorf("ResolveRemote = %+v, want %+v", got, test.want)
			}
		})
	}
}

// remoteLoadCases are the remote sections both config files load or refuse.
var remoteLoadCases = []struct {
	name string
	// section is the JSON of the remote section, empty to leave it out.
	section string
	want    RemoteConfig
	// wantErrText lists fragments the error must carry. A case with none
	// expects no error.
	wantErrText []string
}{
	{name: "absent"},
	{
		name:    "every field",
		section: `{"provider": "github", "timeoutSeconds": 30, "pullRequests": {"isEnabled": true, "isDraft": false}}`,
		want: RemoteConfig{
			Provider:       remote.ProviderGitHub,
			TimeoutSeconds: 30,
			PullRequests:   PullRequestsConfig{IsEnabled: new(true), IsDraft: new(false)},
		},
	},
	{
		name:        "an unknown provider",
		section:     `{"provider": "gitlab"}`,
		wantErrText: []string{remoteProviderKey, "gitlab", "github"},
	},
	{
		name:        "a negative timeout",
		section:     `{"timeoutSeconds": -1}`,
		wantErrText: []string{remote.TimeoutKey, "-1"},
	},
}

// withRemoteSection adds a remote section to the fields of a config file.
func withRemoteSection(fields string, section string) string {
	if section == "" {
		return "{" + fields + "}"
	}
	if fields != "" {
		fields += ", "
	}
	return "{" + fields + `"remote": ` + section + "}"
}

func TestLoad_Remote(t *testing.T) {
	for _, test := range remoteLoadCases {
		t.Run(test.name, func(t *testing.T) {
			home := setupHome(t)
			if err := common.EnsureDir(common.DrudgeDir(home)); err != nil {
				t.Fatalf("could not create drudge dir: %v", err)
			}
			raw := withRemoteSection("", test.section)
			if err := os.WriteFile(common.GlobalConfigPath(home), []byte(raw), common.DefaultFilePerm); err != nil {
				t.Fatalf("could not write config: %v", err)
			}

			cfg, err := Load()
			if len(test.wantErrText) > 0 {
				assertErrorNames(t, err, append(test.wantErrText, common.GlobalConfigPath(home)))
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			assertRemoteConfig(t, cfg.Remote, test.want)
		})
	}
}

func TestLoadLocal_Remote(t *testing.T) {
	for _, test := range remoteLoadCases {
		t.Run(test.name, func(t *testing.T) {
			setupLocalDir(t)
			writeLocalConfig(t, withRemoteSection(`"projectSlug": "test-project"`, test.section))

			cfg, err := LoadLocal()
			if len(test.wantErrText) > 0 {
				assertErrorNames(t, err, append(test.wantErrText, common.LocalConfigPath()))
				return
			}
			if err != nil {
				t.Fatalf("LoadLocal: %v", err)
			}
			assertRemoteConfig(t, cfg.Remote, test.want)
		})
	}
}

func assertErrorNames(t *testing.T, err error, fragments []string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, fragment := range fragments {
		if !strings.Contains(err.Error(), fragment) {
			t.Errorf("error = %q, want it to name %q", err, fragment)
		}
	}
}

func assertRemoteConfig(t *testing.T, got RemoteConfig, want RemoteConfig) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Remote = %s, want %s", formatRemoteConfig(t, got), formatRemoteConfig(t, want))
	}
}

func formatRemoteConfig(t *testing.T, cfg RemoteConfig) string {
	t.Helper()
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("could not encode %+v: %v", cfg, err)
	}
	return string(encoded)
}

func TestSchema_DescribesTheRemoteConfig(t *testing.T) {
	schemas := []struct {
		name   string
		schema []byte
	}{
		{name: "global", schema: Schema()},
		{name: "local", schema: LocalSchema()},
	}

	for _, test := range schemas {
		t.Run(test.name, func(t *testing.T) {
			var schema map[string]any
			if err := json.Unmarshal(test.schema, &schema); err != nil {
				t.Fatalf("schema is not valid JSON: %v", err)
			}
			properties := schema["properties"].(map[string]any)
			remoteSchema, ok := properties["remote"].(map[string]any)
			if !ok {
				t.Fatal("schema has no entry for \"remote\"")
			}
			assertSchemaDescribes(t, remoteSchema, reflect.TypeFor[RemoteConfig](), "remote.")
		})
	}
}
