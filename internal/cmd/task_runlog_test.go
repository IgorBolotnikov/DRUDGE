package cmd

import "testing"

func TestFormatLogLine(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{
			name: "a JSON object",
			line: `{"type":"system","tools":["Bash"]}`,
			want: "{\n  \"type\": \"system\",\n  \"tools\": [\n    \"Bash\"\n  ]\n}",
		},
		{
			name: "a JSON object with whitespace around it",
			line: "  {\"type\":\"result\"}\r",
			want: "{\n  \"type\": \"result\"\n}",
		},
		{
			name: "plain text",
			line: "error: could not connect",
			want: "error: could not connect",
		},
		{
			name: "a line with a percent sign",
			line: "progress: 100%d done",
			want: "progress: 100%d done",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := formatLogLine(testCase.line); got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}
