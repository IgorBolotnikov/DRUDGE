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

func TestStripANSI(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{name: "plain text", text: "pulling image", want: "pulling image"},
		{name: "a color and a reset", text: "\x1b[32mdone\x1b[0m", want: "done"},
		{name: "a 24-bit color", text: "\x1b[38;2;136;192;208mpulling\x1b[0m image", want: "pulling image"},
		{name: "bold and dim", text: "\x1b[1mbold\x1b[22m \x1b[2mdim\x1b[22m", want: "bold dim"},
		{name: "a cursor move and a line erase", text: "\x1b[2K\x1b[1Gpulling 40%", want: "pulling 40%"},
		{name: "a private mode", text: "\x1b[?25lpulling\x1b[?25h", want: "pulling"},
		{name: "a link ended with a bell", text: "\x1b]8;;https://example.com\x07docs\x1b]8;;\x07", want: "docs"},
		{name: "a title ended with a string terminator", text: "\x1b]0;sbx\x1b\\ready", want: "ready"},
		{name: "a two byte escape", text: "\x1bMup", want: "up"},
		{name: "an empty text", text: "", want: ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := StripANSI(testCase.text); got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}
