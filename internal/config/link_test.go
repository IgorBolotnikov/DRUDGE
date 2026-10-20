package config

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/project"
)

func TestLocalConfigLinker_LinkDirectory(t *testing.T) {
	tests := []struct {
		name         string
		slug         string
		repositories []project.Repository
	}{
		{
			name:         "a directory that is a repository",
			slug:         "test-project",
			repositories: []project.Repository{{Path: "."}},
		},
		{
			name:         "a directory holding repositories",
			slug:         "my-cool-app",
			repositories: []project.Repository{{Path: "api", DefaultBranch: "trunk"}, {Path: "ui"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupLocalDir(t)
			home := "/home/user"

			if err := NewLocalConfigLinker(home).LinkDirectory(test.slug, test.repositories); err != nil {
				t.Fatalf("LinkDirectory: %v", err)
			}

			loaded, err := LoadLocal()
			if err != nil {
				t.Fatalf("LoadLocal: %v", err)
			}
			want := LocalConfig{
				Schema:       filepath.Join(home, ".drudge", "schema", "local-config.json"),
				ProjectSlug:  test.slug,
				Repositories: test.repositories,
			}
			if !reflect.DeepEqual(*loaded, want) {
				t.Errorf("loaded = %+v, want %+v", *loaded, want)
			}
		})
	}
}

func TestLocalConfigLinker_LinkedSlug(t *testing.T) {
	tests := []struct {
		name           string
		raw            string
		hasLocalConfig bool
		wantSlug       string
		wantIsLinked   bool
		wantErr        bool
	}{
		{name: "a directory with no local config"},
		{name: "a project directory", raw: `{"projectSlug": "drudge"}`, hasLocalConfig: true, wantSlug: "drudge", wantIsLinked: true},
		{name: "a local config with no slug", raw: `{}`, hasLocalConfig: true, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupLocalDir(t)
			if test.hasLocalConfig {
				writeLocalConfig(t, test.raw)
			}

			slug, isLinked, err := NewLocalConfigLinker("/home/user").LinkedSlug()

			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("LinkedSlug: %v", err)
			}
			if slug != test.wantSlug || isLinked != test.wantIsLinked {
				t.Errorf("LinkedSlug = %q, %v, want %q, %v", slug, isLinked, test.wantSlug, test.wantIsLinked)
			}
		})
	}
}

func TestLocalConfigLinker_RecordSetup(t *testing.T) {
	tests := []struct {
		name         string
		repositories []project.Repository
		setups       map[string]project.DetectedSetup
		want         []project.Repository
	}{
		{
			name:         "a missing setup is filled",
			repositories: []project.Repository{{Path: "api", DefaultBranch: "trunk"}},
			setups:       map[string]project.DetectedSetup{"api": {Setup: []string{"npm ci"}}},
			want:         []project.Repository{{Path: "api", DefaultBranch: "trunk", Setup: []string{"npm ci"}}},
		},
		{
			name:         "an empty setup is kept as an empty list",
			repositories: []project.Repository{{Path: "api"}},
			setups:       map[string]project.DetectedSetup{"api": {Setup: []string{}, Copy: []string{}}},
			want:         []project.Repository{{Path: "api", Setup: []string{}, Copy: []string{}}},
		},
		{
			name:         "a repository setups does not name is left alone",
			repositories: []project.Repository{{Path: "api", Setup: []string{"make deps"}}, {Path: "ui"}},
			setups:       map[string]project.DetectedSetup{"ui": {Setup: []string{"npm ci"}}},
			want:         []project.Repository{{Path: "api", Setup: []string{"make deps"}}, {Path: "ui", Setup: []string{"npm ci"}}},
		},
		{
			name:         "copy paths replace the ones there",
			repositories: []project.Repository{{Path: "api", Copy: []string{"secrets.json"}}},
			setups:       map[string]project.DetectedSetup{"api": {Setup: []string{}, Copy: []string{".env"}}},
			want:         []project.Repository{{Path: "api", Setup: []string{}, Copy: []string{".env"}}},
		},
		{
			name:         "no copy paths keep the ones there",
			repositories: []project.Repository{{Path: "api", Copy: []string{"secrets.json"}}},
			setups:       map[string]project.DetectedSetup{"api": {Setup: []string{"npm ci"}}},
			want:         []project.Repository{{Path: "api", Setup: []string{"npm ci"}, Copy: []string{"secrets.json"}}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupLocalDir(t)
			linker := NewLocalConfigLinker("/home/user")
			if err := linker.LinkDirectory("demo", test.repositories); err != nil {
				t.Fatalf("LinkDirectory: %v", err)
			}

			if err := linker.RecordSetup(test.setups); err != nil {
				t.Fatalf("RecordSetup: %v", err)
			}

			loaded, err := LoadLocal()
			if err != nil {
				t.Fatalf("LoadLocal: %v", err)
			}
			if !reflect.DeepEqual(loaded.Repositories, test.want) {
				t.Errorf("repositories = %+v, want %+v", loaded.Repositories, test.want)
			}
		})
	}
}
