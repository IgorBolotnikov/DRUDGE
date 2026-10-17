package drudger

// StepKind is what one Step of a run's event stream represents.
type StepKind int

// The kinds a harness-neutral Step can take.
const (
	StepInit StepKind = iota
	StepAgentText
	StepToolCall
	StepToolError
	StepFinalResult
	StepUnrecognized
)

// Step is one harness-neutral unit of a run's event stream. A line can hold
// several content blocks, so decoding one line can produce several Steps.
type Step struct {
	Kind StepKind
	// Tool is the name of the tool a StepToolCall names, empty for every
	// other kind.
	Tool string
	// Summary is what the step did, one line for every kind but
	// StepAgentText, which carries the agent's text in full.
	Summary string
	// Depth is how many levels of subagent nesting the step ran at. The
	// Drudger's own session is 0.
	Depth int
	// Raw is the line the step was decoded from.
	Raw string
}
