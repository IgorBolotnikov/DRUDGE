package cmd

import "testing"

func TestRunSetup(t *testing.T) {
	cases := []struct {
		name       string
		runsBefore int
		want       func(home string) string
	}{
		{
			name:       "a fresh home creates every file",
			runsBefore: 0,
			want: func(home string) string {
				return "Setting up DRUDGE at " + home + "/.drudge\n" +
					"  ✓ Created " + home + "/.drudge/schema/theme.json\n" +
					"  ✓ Created " + home + "/.drudge/schema/config.json\n" +
					"  ✓ Created " + home + "/.drudge/schema/local-config.json\n" +
					"  ✓ Created " + home + "/.claude/skills/DRUDGE/SKILL.md\n" +
					"  ✓ Created " + home + "/.drudge/config.json\n" +
					"  ✓ Created " + home + "/.drudge/theme.json\n" +
					"✓ DRUDGE is set up, run drg project init <name> in a project directory\n"
			},
		},
		{
			name:       "a home with the configs skips them",
			runsBefore: 1,
			want: func(home string) string {
				return "Setting up DRUDGE at " + home + "/.drudge\n" +
					"  ✓ Created " + home + "/.drudge/schema/theme.json\n" +
					"  ✓ Created " + home + "/.drudge/schema/config.json\n" +
					"  ✓ Created " + home + "/.drudge/schema/local-config.json\n" +
					"  ✓ Created " + home + "/.claude/skills/DRUDGE/SKILL.md\n" +
					"  · " + home + "/.drudge/config.json already exists\n" +
					"  · " + home + "/.drudge/theme.json already exists\n" +
					"✓ DRUDGE is set up, run drg project init <name> in a project directory\n"
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("NO_COLOR", "1")
			for range testCase.runsBefore {
				captureOutput(func() {
					if err := runSetup(nil); err != nil {
						t.Fatalf("runSetup before: %v", err)
					}
				})
			}

			var err error
			var stderr string
			stdout := captureOutput(func() {
				stderr = captureStderr(func() { err = runSetup(nil) })
			})

			if err != nil {
				t.Fatalf("runSetup: %v", err)
			}
			if want := testCase.want(home); stdout != want {
				t.Errorf("stdout = %q, want %q", stdout, want)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want nothing", stderr)
			}
		})
	}
}
