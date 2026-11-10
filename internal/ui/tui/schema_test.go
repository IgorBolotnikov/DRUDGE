package tui

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views/tasklist"
)

func TestSchema_ViewOptions(t *testing.T) {
	var schema struct {
		Definitions map[string]struct {
			AllOf []schemaCondition `json:"allOf"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(Schema(), &schema); err != nil {
		t.Fatalf("the schema is not valid JSON: %v", err)
	}
	conditions := schema.Definitions["node"].AllOf
	if len(conditions) != len(viewKinds) {
		t.Fatalf("expected one condition per view, got %d", len(conditions))
	}
	index := slices.IndexFunc(conditions, func(condition schemaCondition) bool {
		return condition.If.Properties[viewKey].Const == tasklist.Name
	})
	if index == -1 {
		t.Fatalf("expected a condition for view %q", tasklist.Name)
	}
	viewOptions := conditions[index].Then.Properties[viewOptionsKey]
	if viewOptions.AdditionalProperties == nil || *viewOptions.AdditionalProperties {
		t.Error("expected the view options to refuse an unknown option")
	}
	status := viewOptions.Properties[tasklist.StatusOption]
	if wantStatuses := tasklist.Options[0].Allowed; !slices.Equal(status.Enum, wantStatuses) {
		t.Errorf("expected the status option to allow %q, got %q", wantStatuses, status.Enum)
	}
}
