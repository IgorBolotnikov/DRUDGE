package tui

import (
	"encoding/json"
	"maps"
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

func TestSchema_RefreshSecondsListsEveryFeed(t *testing.T) {
	var schema struct {
		Properties struct {
			RefreshSeconds struct {
				Properties map[string]struct {
					Minimum float64 `json:"minimum"`
					Default float64 `json:"default"`
				} `json:"properties"`
				AdditionalProperties bool `json:"additionalProperties"`
			} `json:"refreshSeconds"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(Schema(), &schema); err != nil {
		t.Fatalf("the schema is not valid JSON: %v", err)
	}
	properties := schema.Properties.RefreshSeconds.Properties
	if got := slices.Sorted(maps.Keys(properties)); !slices.Equal(got, slices.Sorted(slices.Values(feedKindNames()))) {
		t.Fatalf("expected the schema to list the feeds %q, got %q", feedKindNames(), got)
	}
	for _, kind := range feedKinds {
		property := properties[string(kind.Name)]
		if want := kind.MinInterval.Seconds(); property.Minimum != want {
			t.Errorf("expected feed %q to have the minimum %g, got %g", kind.Name, want, property.Minimum)
		}
		if want := kind.DefaultInterval.Seconds(); property.Default != want {
			t.Errorf("expected feed %q to have the default %g, got %g", kind.Name, want, property.Default)
		}
	}
	if schema.Properties.RefreshSeconds.AdditionalProperties {
		t.Error("expected the schema to refuse a feed it does not list")
	}
}
