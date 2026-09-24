package common

import "testing"

func TestJoinNames(t *testing.T) {
	cases := []struct {
		name  string
		names []string
		want  string
	}{
		{name: "no names", names: nil, want: ""},
		{name: "one name", names: []string{"--block"}, want: "--block"},
		{name: "two names", names: []string{"--block", "--unblock"}, want: "--block and --unblock"},
		{name: "three names", names: []string{"--unblock", "--block", "--blocked-by"}, want: "--unblock, --block and --blocked-by"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := JoinNames(testCase.names); got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}
