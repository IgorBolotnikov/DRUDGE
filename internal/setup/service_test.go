package setup

import (
	"os"
	"path/filepath"
	"slices"
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
		name               string
		runsBefore         int
		wantConfigsExisted bool
	}{
		{name: "fresh home creates every file", runsBefore: 0, wantConfigsExisted: false},
		{name: "second run skips the configs", runsBefore: 1, wantConfigsExisted: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			home := tempHome(t)
			service := NewSetupService(home)
			for range testCase.runsBefore {
				if _, err := service.Setup(); err != nil {
					t.Fatalf("Setup before: %v", err)
				}
				for _, path := range schemaPaths(home) {
					if err := os.WriteFile(path, []byte(staleSchema), common.DefaultFilePerm); err != nil {
						t.Fatalf("WriteFile: %v", err)
					}
				}
			}

			result, err := service.Setup()
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
				{result.GlobalConfig, ConfigFile{Path: common.GlobalConfigPath(home), HasExisted: testCase.wantConfigsExisted}},
				{result.ThemeConfig, ConfigFile{Path: themeConfigPath(home), HasExisted: testCase.wantConfigsExisted}},
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

func TestSetup_DefaultConfigsLoad(t *testing.T) {
	home := tempHome(t)
	if _, err := NewSetupService(home).Setup(); err != nil {
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
				if _, err := service.Setup(); err != nil {
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
