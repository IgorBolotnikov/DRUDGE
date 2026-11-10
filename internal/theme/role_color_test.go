package theme

import "testing"

func TestRoleColor(t *testing.T) {
	cases := []struct {
		name      string
		themeFile string
		role      string
		want      RoleColor
		wantOk    bool
	}{
		{name: "a hex theme", themeFile: `{"theme": "nord"}`, role: RoleError, want: RoleColor{Kind: ColorHex, Hex: "#BF616A"}, wantOk: true},
		{name: "an ANSI color of the system theme", themeFile: `{"theme": "system"}`, role: RoleError, want: RoleColor{Kind: ColorANSI, ANSI: 1}, wantOk: true},
		{name: "white of the system theme", themeFile: `{"theme": "system"}`, role: RoleSecondary, want: RoleColor{Kind: ColorANSI, ANSI: 7}, wantOk: true},
		{name: "dim of the system theme", themeFile: `{"theme": "system"}`, role: RoleMuted, want: RoleColor{Kind: ColorDim}, wantOk: true},
		{name: "an override of the system theme", themeFile: `{"theme": "system", "overrides": {"error": "#ff0000"}}`, role: RoleError, want: RoleColor{Kind: ColorHex, Hex: "#ff0000"}, wantOk: true},
		{name: "the selection of a hex theme", themeFile: `{"theme": "nord"}`, role: RoleSelection, want: RoleColor{Kind: ColorHex, Hex: "#3B4252"}, wantOk: true},
		{name: "the selection of the system theme is bright black", themeFile: `{"theme": "system"}`, role: RoleSelection, want: RoleColor{Kind: ColorANSI, ANSI: 8}, wantOk: true},
		{name: "a selection override", themeFile: `{"theme": "nord", "overrides": {"selection": "#112233"}}`, role: RoleSelection, want: RoleColor{Kind: ColorHex, Hex: "#112233"}, wantOk: true},
		{name: "a selection override of the system theme", themeFile: `{"theme": "system", "overrides": {"selection": "#112233"}}`, role: RoleSelection, want: RoleColor{Kind: ColorHex, Hex: "#112233"}, wantOk: true},
		{name: "an unknown role", themeFile: `{"theme": "nord"}`, role: "nope", wantOk: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			setupTempHome(t, testCase.themeFile)
			theme, err := Load("")
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			got, ok := theme.RoleColor(testCase.role)
			if got != testCase.want || ok != testCase.wantOk {
				t.Errorf("RoleColor(%q) = %+v, %v, want %+v, %v", testCase.role, got, ok, testCase.want, testCase.wantOk)
			}
		})
	}
}
