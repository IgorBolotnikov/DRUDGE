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
