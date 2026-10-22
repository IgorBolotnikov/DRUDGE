package streams

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
)

// Event types the opencode harness writes with its JSON format.
const (
	opencodeEventStepStart  = "step_start"
	opencodeEventStepFinish = "step_finish"
	opencodeEventText       = "text"
	opencodeEventToolUse    = "tool_use"
	opencodeEventError      = "error"
)

// opencodeToolStatusError is the status of a tool call that failed.
const opencodeToolStatusError = "error"

// Tool names of the opencode harness the compact view knows how to summarize.
const (
	opencodeToolBash  = "bash"
	opencodeToolRead  = "read"
	opencodeToolEdit  = "edit"
	opencodeToolWrite = "write"
	opencodeToolGrep  = "grep"
	opencodeToolGlob  = "glob"
	opencodeToolSkill = "skill"
	opencodeToolTask  = "task"
)

// opencodeReasonStop is the reason of the step that ends a run the agent
// finished.
const opencodeReasonStop = "stop"

// Names of the opencode errors that say the provider turned the run away.
const (
	opencodeErrorAPI          = "APIError"
	opencodeErrorProviderAuth = "ProviderAuthError"
)

// HTTP statuses of an API error that say which kind of refusal it was.
const (
	opencodeStatusUnauthorized = 401
	opencodeStatusForbidden    = 403
	opencodeStatusRateLimited  = 429
	opencodeStatusUnavailable  = 503
	opencodeStatusOverloaded   = 529
)

// Error codes a provider refusal reads as. They hold the fragments the vendor
// error classes match.
const (
	opencodeCodeAuth      = "authentication_error"
	opencodeCodeRateLimit = "rate_limit_error"
	opencodeCodeOutage    = "overloaded_error"
	opencodeCodeUnknown   = "api_error"
)

// OpencodeReader reads the JSON event stream the opencode harness writes.
type OpencodeReader struct{}

// NewOpencodeReader builds a reader for the JSON format the opencode harness
// writes.
func NewOpencodeReader() *OpencodeReader {
	return &OpencodeReader{}
}

// opencodeLine is one line of an opencode event stream. Only the fields drudge
// reads are decoded.
type opencodeLine struct {
	Type string `json:"type"`
	// Timestamp is in milliseconds.
	Timestamp int64  `json:"timestamp"`
	SessionID string `json:"sessionID"`
	Part      struct {
		Reason string  `json:"reason"`
		Cost   float64 `json:"cost"`
		Text   string  `json:"text"`
		Tool   string  `json:"tool"`
		State  struct {
			Status string          `json:"status"`
			Input  json.RawMessage `json:"input"`
			Error  string          `json:"error"`
		} `json:"state"`
	} `json:"part"`
	Error struct {
		Name string `json:"name"`
		Data struct {
			Message string `json:"message"`
			// StatusCode is zero when the provider could not be reached.
			StatusCode int `json:"statusCode"`
		} `json:"data"`
	} `json:"error"`
}

// opencodeToolInput is the subset of a tool call's input the compact view
// summarizes.
type opencodeToolInput struct {
	Command     string `json:"command"`
	FilePath    string `json:"filePath"`
	Pattern     string `json:"pattern"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Decode turns one line of an opencode event stream into the Steps it holds.
// Every Step has depth 0, since a subagent writes its steps outside the
// stream. A line that is not valid JSON, or whose event type is none of the
// ones below, decodes to one unrecognized Step holding the line as it is.
func (*OpencodeReader) Decode(line string) []drudger.Step {
	var parsed opencodeLine
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		return unrecognizedStep(line)
	}

	switch parsed.Type {
	case opencodeEventText:
		return []drudger.Step{{Kind: drudger.StepAgentText, Summary: parsed.Part.Text, Raw: line}}
	case opencodeEventToolUse:
		return opencodeToolSteps(parsed, line)
	case opencodeEventError:
		summary := fmt.Sprintf("%s · %s: %s", drudger.StatusFuckedUp, parsed.Error.Name, parsed.Error.Data.Message)
		return []drudger.Step{{Kind: drudger.StepFinalResult, Summary: summary, Raw: line}}
	case opencodeEventStepStart, opencodeEventStepFinish:
		return nil
	default:
		return unrecognizedStep(line)
	}
}

// opencodeToolSteps summarizes a tool call by the argument that matters for
// the tool it names, and adds a tool error Step when the call failed.
func opencodeToolSteps(parsed opencodeLine, line string) []drudger.Step {
	var input opencodeToolInput
	// Input that does not parse leaves every field empty, which reads the same
	// as a tool call with nothing worth summarizing.
	_ = json.Unmarshal(parsed.Part.State.Input, &input)

	steps := []drudger.Step{{
		Kind:    drudger.StepToolCall,
		Tool:    parsed.Part.Tool,
		Summary: capSummary(opencodeToolSummary(parsed.Part.Tool, input)),
		Raw:     line,
	}}
	if parsed.Part.State.Status == opencodeToolStatusError {
		steps = append(steps, drudger.Step{Kind: drudger.StepToolError, Summary: capSummary(parsed.Part.State.Error), Raw: line})
	}
	return steps
}

func opencodeToolSummary(tool string, input opencodeToolInput) string {
	switch tool {
	case opencodeToolBash:
		return input.Command
	case opencodeToolRead, opencodeToolEdit, opencodeToolWrite:
		return input.FilePath
	case opencodeToolGrep, opencodeToolGlob:
		return input.Pattern
	case opencodeToolSkill:
		return input.Name
	case opencodeToolTask:
		return input.Description
	default:
		return ""
	}
}

// SessionID reads the session id off the first event that carries one. Every
// event of the stream carries it.
func (*OpencodeReader) SessionID(stream []byte) (string, error) {
	var sessionID string

	err := scanStream(stream, func(parsed opencodeLine) bool {
		sessionID = parsed.SessionID
		return sessionID == ""
	})
	return sessionID, err
}

// Outcome adds up the steps of a run. The stream has no terminal event, so the
// result exists once a step ends with the stop reason or an error event
// appears. An API error or a provider auth error is a vendor refusal.
func (*OpencodeReader) Outcome(stream []byte) (drudger.StreamOutcome, error) {
	var (
		outcome        drudger.StreamOutcome
		result         drudger.SessionResult
		hasEnded       bool
		hasTimestamp   bool
		firstTimestamp int64
		lastTimestamp  int64
		lastReason     string
		lastText       string
		lastError      string
		lastErrorName  string
	)

	err := scanStream(stream, func(parsed opencodeLine) bool {
		if !hasTimestamp {
			firstTimestamp = parsed.Timestamp
			hasTimestamp = true
		}
		lastTimestamp = parsed.Timestamp

		switch parsed.Type {
		case opencodeEventStepFinish:
			result.NumTurns++
			result.CostUSD += parsed.Part.Cost
			lastReason = parsed.Part.Reason
			result.HasSucceeded = lastReason == opencodeReasonStop
			if result.HasSucceeded {
				hasEnded = true
			}
		case opencodeEventText:
			lastText = parsed.Part.Text
		case opencodeEventError:
			result.IsError = true
			result.HasSucceeded = false
			lastErrorName = parsed.Error.Name
			lastError = parsed.Error.Data.Message
			outcome.ErrorCode, outcome.IsVendorRefusal = opencodeRefusalCode(parsed.Error.Name, parsed.Error.Data.StatusCode)
			hasEnded = true
		}
		return true
	})
	if err != nil || !hasEnded {
		return drudger.StreamOutcome{}, err
	}

	result.Subtype = lastReason
	result.Text = lastText
	if result.IsError {
		result.Subtype = lastErrorName
		result.Text = lastError
	}
	result.Duration = time.Duration(lastTimestamp-firstTimestamp) * time.Millisecond
	outcome.Result = &result
	return outcome, nil
}

// opencodeRefusalCode reads the error code of an opencode error and whether
// the provider refused the run. An API error without a status is a connection
// failure.
func opencodeRefusalCode(name string, statusCode int) (string, bool) {
	if name == opencodeErrorProviderAuth {
		return opencodeCodeAuth, true
	}
	if name != opencodeErrorAPI {
		return "", false
	}

	switch statusCode {
	case opencodeStatusUnauthorized, opencodeStatusForbidden:
		return opencodeCodeAuth, true
	case opencodeStatusRateLimited:
		return opencodeCodeRateLimit, true
	case opencodeStatusUnavailable, opencodeStatusOverloaded:
		return opencodeCodeOutage, true
	default:
		return opencodeCodeUnknown, true
	}
}
