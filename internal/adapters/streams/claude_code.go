// Package streams holds the stream decoders of the agent harnesses drudge
// supports.
package streams

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
)

// Event types and subtypes the Claude Code harness writes.
const (
	eventTypeSystem    = "system"
	eventTypeAssistant = "assistant"
	eventTypeUser      = "user"
	eventTypeResult    = "result"
	eventTypeRateLimit = "rate_limit_event"

	subtypeInit            = "init"
	subtypeThinkingTokens  = "thinking_tokens"
	subtypeCommandsChanged = "commands_changed"
	subtypeVCSStateChanged = "vcs_state_changed"
	// taskSubtypePrefix covers every system subtype a subagent dispatch
	// writes for its own bookkeeping: task_started, task_notification and
	// task_progress.
	taskSubtypePrefix = "task_"

	resultSubtypeSuccess = "success"
)

// Content block types of an assistant or a user event.
const (
	contentTypeText       = "text"
	contentTypeToolUse    = "tool_use"
	contentTypeToolResult = "tool_result"
)

// Tool names the compact view knows how to summarize.
const (
	toolBash  = "Bash"
	toolRead  = "Read"
	toolEdit  = "Edit"
	toolWrite = "Write"
	toolGrep  = "Grep"
	toolGlob  = "Glob"
	toolSkill = "Skill"
	toolTask  = "Task"
	toolAgent = "Agent"
)

// summaryCapLength is the longest a tool call summary may run before it is
// cut.
const summaryCapLength = 120

// summaryCutSuffix ends a summary that was cut short, and a session id a run
// shows abbreviated.
const summaryCutSuffix = "…"

// sessionIDDisplayLength is how many leading characters of a session id the
// init line shows.
const sessionIDDisplayLength = 8

// ClaudeCodeDecoder decodes the stream-json lines the Claude Code harness
// writes into harness-neutral Steps.
type ClaudeCodeDecoder struct{}

// NewClaudeCodeDecoder builds a decoder for the stream-json format the Claude
// Code harness writes.
func NewClaudeCodeDecoder() *ClaudeCodeDecoder {
	return &ClaudeCodeDecoder{}
}

// streamLine is one line of a Claude Code event stream. The harness writes
// many more fields than these; only the ones a Step needs are decoded.
type streamLine struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Message struct {
		Content []contentBlock `json:"content"`
	} `json:"message"`

	// Fields of a system/init event.
	SessionID string `json:"session_id"`
	CWD       string `json:"cwd"`
	Model     string `json:"model"`

	// Fields of the terminal result event.
	IsError      bool    `json:"is_error"`
	NumTurns     int     `json:"num_turns"`
	DurationMS   int64   `json:"duration_ms"`
	TotalCostUSD float64 `json:"total_cost_usd"`
}

// contentBlock is one block of an assistant or a user event's message.
type contentBlock struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input"`
	Content string          `json:"content"`
	IsError bool            `json:"is_error"`
}

// toolInput is the subset of a tool call's input the compact view summarizes.
type toolInput struct {
	Command     string `json:"command"`
	FilePath    string `json:"file_path"`
	Pattern     string `json:"pattern"`
	Skill       string `json:"skill"`
	Description string `json:"description"`
}

// Decode turns one line of a Claude Code event stream into the Steps it
// holds. A line that is not valid JSON, or whose event type is none of the
// ones below, decodes to one unrecognized Step holding the line as it is.
func (*ClaudeCodeDecoder) Decode(line string) []drudger.Step {
	var parsed streamLine
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		return unrecognizedStep(line)
	}

	switch parsed.Type {
	case eventTypeSystem:
		return systemSteps(parsed, line)
	case eventTypeAssistant, eventTypeUser:
		return contentSteps(parsed.Message.Content, line)
	case eventTypeResult:
		return []drudger.Step{resultStep(parsed, line)}
	case eventTypeRateLimit:
		return nil
	default:
		return unrecognizedStep(line)
	}
}

// systemSteps decodes a system event. An init event becomes a Step, the
// events a harness writes for its own bookkeeping decode to none, and any
// other subtype is unrecognized.
func systemSteps(parsed streamLine, line string) []drudger.Step {
	switch {
	case parsed.Subtype == subtypeInit:
		return []drudger.Step{initStep(parsed, line)}
	case isNoiseSubtype(parsed.Subtype):
		return nil
	default:
		return unrecognizedStep(line)
	}
}

// isNoiseSubtype reports whether a system subtype is written only for a
// harness's own bookkeeping, with nothing a compact view shows.
func isNoiseSubtype(subtype string) bool {
	switch {
	case subtype == subtypeThinkingTokens, subtype == subtypeCommandsChanged, subtype == subtypeVCSStateChanged:
		return true
	case strings.HasPrefix(subtype, taskSubtypePrefix):
		return true
	default:
		return false
	}
}

// initStep summarizes the Session id, the model and the working directory an
// init event reports.
func initStep(parsed streamLine, line string) drudger.Step {
	summary := fmt.Sprintf("session %s %s · %s", shortSessionID(parsed.SessionID), parsed.Model, parsed.CWD)
	return drudger.Step{Kind: drudger.StepInit, Summary: summary, Raw: line}
}

// shortSessionID cuts a session id down to the leading characters a compact
// view shows.
func shortSessionID(sessionID string) string {
	runes := []rune(sessionID)
	if len(runes) <= sessionIDDisplayLength {
		return sessionID
	}
	return string(runes[:sessionIDDisplayLength]) + summaryCutSuffix
}

// contentSteps decodes the content blocks of an assistant or a user event.
// A thinking block and a successful tool result decode to no Step.
func contentSteps(blocks []contentBlock, line string) []drudger.Step {
	var steps []drudger.Step
	for _, block := range blocks {
		switch block.Type {
		case contentTypeText:
			steps = append(steps, drudger.Step{Kind: drudger.StepAgentText, Summary: block.Text, Raw: line})
		case contentTypeToolUse:
			steps = append(steps, toolCallStep(block, line))
		case contentTypeToolResult:
			if block.IsError {
				steps = append(steps, toolErrorStep(block, line))
			}
		}
	}
	return steps
}

// toolCallStep summarizes a tool call by the argument that matters for the
// tool it names. An unknown tool gets no summary, so a compact view shows its
// name alone.
func toolCallStep(block contentBlock, line string) drudger.Step {
	var input toolInput
	// Input that does not parse leaves every field empty, which reads the same
	// as a tool call with nothing worth summarizing.
	_ = json.Unmarshal(block.Input, &input)
	return drudger.Step{Kind: drudger.StepToolCall, Tool: block.Name, Summary: capSummary(toolSummary(block.Name, input)), Raw: line}
}

func toolSummary(tool string, input toolInput) string {
	switch tool {
	case toolBash:
		return input.Command
	case toolRead, toolEdit, toolWrite:
		return input.FilePath
	case toolGrep, toolGlob:
		return input.Pattern
	case toolSkill:
		return input.Skill
	case toolTask, toolAgent:
		return input.Description
	default:
		return ""
	}
}

// toolErrorStep summarizes a tool result the agent flagged as an error.
func toolErrorStep(block contentBlock, line string) drudger.Step {
	return drudger.Step{Kind: drudger.StepToolError, Summary: capSummary(block.Content), Raw: line}
}

// resultStep summarizes the terminal event of a run: its verdict, how many
// turns it took, how long it ran and what it cost.
func resultStep(parsed streamLine, line string) drudger.Step {
	verdict := drudger.StatusFuckedUp
	if !parsed.IsError && parsed.Subtype == resultSubtypeSuccess {
		verdict = drudger.StatusGotShitDone
	}
	duration := (time.Duration(parsed.DurationMS) * time.Millisecond).Round(time.Second)
	summary := fmt.Sprintf("%s · %d turns · %s · $%.2f", verdict, parsed.NumTurns, duration, parsed.TotalCostUSD)
	return drudger.Step{Kind: drudger.StepFinalResult, Summary: summary, Raw: line}
}

// capSummary keeps the first line of a summary, up to summaryCapLength
// characters, and marks it cut when either limit drops anything.
func capSummary(raw string) string {
	firstLine := raw
	wasCut := false
	if newline := strings.IndexByte(raw, '\n'); newline >= 0 {
		firstLine = raw[:newline]
		wasCut = true
	}

	runes := []rune(firstLine)
	if len(runes) > summaryCapLength {
		runes = runes[:summaryCapLength]
		wasCut = true
	}

	capped := string(runes)
	if wasCut {
		capped += summaryCutSuffix
	}
	return capped
}

// unrecognizedStep wraps a line a compact view cannot interpret, printed as
// it is.
func unrecognizedStep(line string) []drudger.Step {
	return []drudger.Step{{Kind: drudger.StepUnrecognized, Raw: line}}
}
