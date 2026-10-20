package common

import "testing"

func TestIsUUID(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "a lowercase UUID", value: "006684e3-dbe9-4316-8aba-8a67a8f01f8f", want: true},
		{name: "an uppercase UUID", value: "006684E3-DBE9-4316-8ABA-8A67A8F01F8F", want: true},
		{name: "a short id", value: "006684e3"},
		{name: "a UUID without hyphens", value: "006684e3dbe943168aba8a67a8f01f8f"},
		{name: "a UUID with a path in front", value: "../006684e3-dbe9-4316-8aba-8a67a8f01f8f"},
		{name: "a UUID with a non-hex digit", value: "006684g3-dbe9-4316-8aba-8a67a8f01f8f"},
		{name: "an empty value", value: ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := IsUUID(testCase.value); got != testCase.want {
				t.Errorf("IsUUID(%q) = %v, want %v", testCase.value, got, testCase.want)
			}
		})
	}
}
