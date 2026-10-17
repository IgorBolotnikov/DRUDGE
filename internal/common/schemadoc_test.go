package common

import (
	"strings"
	"testing"
)

func TestRenderSchemaTable(t *testing.T) {
	cases := []struct {
		name     string
		schema   string
		wantRows []string
		wantErr  string
	}{
		{
			name:     "a key with a default and an enum",
			schema:   `{"properties": {"harness": {"type": "string", "enum": ["a", "b"], "default": "a", "description": "The harness."}}}`,
			wantRows: []string{"| `harness` | string | `\"a\"` | `\"a\"`, `\"b\"` | The harness. |"},
		},
		{
			name:     "a key with a minimum",
			schema:   `{"properties": {"size": {"type": "integer", "minimum": 0, "default": 20}}}`,
			wantRows: []string{"| `size` | integer | `20` | minimum 0 |  |"},
		},
		{
			name:     "a key with a pattern",
			schema:   `{"properties": {"color": {"type": "string", "pattern": "^#[0-9a-f]{6}$"}}}`,
			wantRows: []string{"| `color` | string |  | matches `^#[0-9a-f]{6}$` |  |"},
		},
		{
			name:   "nested objects as dotted paths in schema order",
			schema: `{"properties": {"drudger": {"type": "object", "description": "Drudgers.", "properties": {"timeouts": {"type": "object", "properties": {"zSeconds": {"type": "integer"}, "aSeconds": {"type": "integer"}}}}}}}`,
			wantRows: []string{
				"| `drudger` | object |  |  | Drudgers. |",
				"| `drudger.timeouts` | object |  |  |  |",
				"| `drudger.timeouts.zSeconds` | integer |  |  |  |",
				"| `drudger.timeouts.aSeconds` | integer |  |  |  |",
			},
		},
		{
			name:   "array items with a required key",
			schema: `{"properties": {"repositories": {"type": "array", "items": {"type": "object", "required": ["path"], "properties": {"path": {"type": "string", "description": "Where it sits."}}}}}}`,
			wantRows: []string{
				"| `repositories` | array |  |  |  |",
				"| `repositories[].path` | string |  |  | Required. Where it sits. |",
			},
		},
		{
			name:   "keys the schema does not name",
			schema: `{"properties": {"overrides": {"type": "object", "additionalProperties": {"type": "string"}}}}`,
			wantRows: []string{
				"| `overrides` | object |  |  |  |",
				"| `overrides.<key>` | string |  |  |  |",
			},
		},
		{
			name:     "a description with a pipe and a tag",
			schema:   `{"properties": {"title": {"type": "string", "default": "<a|b>", "description": "Text | <b>"}}}`,
			wantRows: []string{"| `title` | string | `\"<a\\|b>\"` |  | Text \\| &lt;b&gt; |"},
		},
		{
			name:    "a key without a type",
			schema:  `{"properties": {"task": {"properties": {"pageSize": {"type": "integer"}}}}}`,
			wantErr: `key "task" has no type`,
		},
		{
			name:    "a schema that is not JSON",
			schema:  `{"properties":`,
			wantErr: "could not parse the schema",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := RenderSchemaTable([]byte(testCase.schema))
			if testCase.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
					t.Fatalf("expected an error containing %q, got %v", testCase.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := schemaTableHeader + strings.Join(testCase.wantRows, "\n") + "\n"
			if got != want {
				t.Errorf("expected\n%s\ngot\n%s", want, got)
			}
		})
	}
}

func TestSpliceGenerated(t *testing.T) {
	cases := []struct {
		name    string
		page    string
		want    string
		wantErr string
	}{
		{
			name: "replaces the old table and keeps the rest",
			page: "intro\n\n" + GeneratedBeginMarker + "\n\nold\n\n" + GeneratedEndMarker + "\n",
			want: "intro\n\n" + GeneratedBeginMarker + "\n\nnew\n\n" + GeneratedEndMarker + "\n",
		},
		{
			name: "fills empty markers",
			page: GeneratedBeginMarker + GeneratedEndMarker,
			want: GeneratedBeginMarker + "\n\nnew\n\n" + GeneratedEndMarker,
		},
		{
			name:    "refuses a page without markers",
			page:    "intro\n",
			wantErr: "exactly one",
		},
		{
			name:    "refuses a page with two begin markers",
			page:    GeneratedBeginMarker + GeneratedBeginMarker + GeneratedEndMarker,
			wantErr: "exactly one",
		},
		{
			name:    "refuses markers in the wrong order",
			page:    GeneratedEndMarker + GeneratedBeginMarker,
			wantErr: "comes before",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := SpliceGenerated(testCase.page, "new\n")
			if testCase.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
					t.Fatalf("expected an error containing %q, got %v", testCase.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}
