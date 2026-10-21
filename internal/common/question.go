package common

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// QuestionKind decides how a frontend asks a question and reads its answer.
type QuestionKind string

const (
	QuestionSelect QuestionKind = "select"
	QuestionNumber QuestionKind = "number"
	QuestionText   QuestionKind = "text"
)

// JSON schema types a question sets.
const (
	schemaBooleanType = "boolean"
	schemaIntegerType = "integer"
	schemaStringType  = "string"
)

// Answers of a yes or no question.
const (
	AnswerNo  = "no"
	AnswerYes = "yes"
)

// Question is one config setting a frontend asks the user for.
type Question struct {
	Key     string
	Label   string
	Kind    QuestionKind
	Options []QuestionOption // The choices of a select
	Default string
	// DefaultLabel is what a typed question shows for its default. Empty
	// shows the default itself.
	DefaultLabel string
	// Condition is the answer an earlier question needs for this question to
	// be asked. Nil asks it every time.
	Condition *QuestionCondition
	// Validate refuses an answer the config key cannot take. Nil takes every
	// answer.
	Validate func(answer string) error
}

// QuestionCondition is the answer of the question that sets Key.
type QuestionCondition struct {
	Key    string
	Answer string
}

// QuestionOption is one choice of a select.
type QuestionOption struct {
	Value string
	Label string
}

// Answers maps the config key of a question to its answer.
type Answers map[string]string

// NewSelectQuestion builds a select that sets key, with the enum of key in
// schema as its options and the default of key as its default. It refuses a
// key the schema does not have, a key without an enum and a default outside
// the enum. A key without a default leaves the default empty.
func NewSelectQuestion(schema []byte, key string, label string) (Question, error) {
	node, err := findSchemaKey(schema, key)
	if err != nil {
		return Question{}, err
	}
	if len(node.Enum) == 0 {
		return Question{}, fmt.Errorf("key %q of the schema has no enum to select from", key)
	}
	question := Question{Key: key, Label: label, Kind: QuestionSelect}
	for _, rawValue := range node.Enum {
		var value string
		if err := json.Unmarshal(rawValue, &value); err != nil {
			return Question{}, fmt.Errorf("enum of key %q holds %s, want strings: %w", key, rawValue, err)
		}
		question.Options = append(question.Options, QuestionOption{Value: value, Label: value})
	}
	question.Validate = question.validateOption
	if len(node.Default) > 0 {
		if err := json.Unmarshal(node.Default, &question.Default); err != nil {
			return Question{}, fmt.Errorf("default of key %q is %s, want a string: %w", key, node.Default, err)
		}
		if err := question.Validate(question.Default); err != nil {
			return Question{}, fmt.Errorf("default of key %q: %w", key, err)
		}
	}
	return question, nil
}

// NewBoolQuestion builds a select of no and yes that sets the boolean key,
// with the default of key in schema as its default. It refuses a key the
// schema does not have and a key that is not a boolean.
func NewBoolQuestion(schema []byte, key string, label string) (Question, error) {
	node, err := findSchemaKey(schema, key)
	if err != nil {
		return Question{}, err
	}
	if node.Type != schemaBooleanType {
		return Question{}, fmt.Errorf("key %q of the schema is a %s, want a %s", key, node.Type, schemaBooleanType)
	}
	question := Question{
		Key:     key,
		Label:   label,
		Kind:    QuestionSelect,
		Options: []QuestionOption{{Value: AnswerNo, Label: AnswerNo}, {Value: AnswerYes, Label: AnswerYes}},
		Default: AnswerNo,
	}
	if len(node.Default) > 0 {
		var isOn bool
		if err := json.Unmarshal(node.Default, &isOn); err != nil {
			return Question{}, fmt.Errorf("default of key %q is %s, want a boolean: %w", key, node.Default, err)
		}
		question.Default = FormatBoolAnswer(isOn)
	}
	question.Validate = question.validateOption
	return question, nil
}

// NewNumberQuestion builds a typed question that sets the integer key, with
// the default of key in schema as its default. Its answer must be a whole
// number that validate takes. It refuses a key the schema does not have, a
// key that is not an integer and a default validate refuses.
func NewNumberQuestion(schema []byte, key string, label string, validate func(value int) error) (Question, error) {
	node, err := findSchemaKey(schema, key)
	if err != nil {
		return Question{}, err
	}
	if node.Type != schemaIntegerType {
		return Question{}, fmt.Errorf("key %q of the schema is a %s, want an %s", key, node.Type, schemaIntegerType)
	}
	question := Question{
		Key:   key,
		Label: label,
		Kind:  QuestionNumber,
		Validate: func(answer string) error {
			value, err := ParseNumberAnswer(answer)
			if err != nil {
				return err
			}
			return validate(value)
		},
	}
	if len(node.Default) > 0 {
		var value int
		if err := json.Unmarshal(node.Default, &value); err != nil {
			return Question{}, fmt.Errorf("default of key %q is %s, want a whole number: %w", key, node.Default, err)
		}
		question.Default = strconv.Itoa(value)
	}
	if err := question.Validate(question.Default); err != nil {
		return Question{}, fmt.Errorf("default of key %q: %w", key, err)
	}
	return question, nil
}

// NewTextQuestion builds a typed question that sets the string key and takes
// every answer. Its default is the default of key in schema, shown as
// defaultLabel when that is not empty. It refuses a key the schema does not
// have and a key that is not a string.
func NewTextQuestion(schema []byte, key string, label string, defaultLabel string) (Question, error) {
	node, err := findSchemaKey(schema, key)
	if err != nil {
		return Question{}, err
	}
	if node.Type != schemaStringType {
		return Question{}, fmt.Errorf("key %q of the schema is a %s, want a %s", key, node.Type, schemaStringType)
	}
	question := Question{Key: key, Label: label, Kind: QuestionText, DefaultLabel: defaultLabel}
	if len(node.Default) > 0 {
		if err := json.Unmarshal(node.Default, &question.Default); err != nil {
			return Question{}, fmt.Errorf("default of key %q is %s, want a string: %w", key, node.Default, err)
		}
	}
	return question, nil
}

// ParseNumberAnswer returns the whole number an answer holds.
func ParseNumberAnswer(answer string) (int, error) {
	value, err := strconv.Atoi(answer)
	if err != nil {
		return 0, fmt.Errorf("%q is not a whole number", answer)
	}
	return value, nil
}

// FormatBoolAnswer returns the answer of a yes or no question for isOn.
func FormatBoolAnswer(isOn bool) string {
	if isOn {
		return AnswerYes
	}
	return AnswerNo
}

// IsAskedAfter reports whether the condition of the question holds for the
// answers given so far.
func (q Question) IsAskedAfter(answers Answers) bool {
	return q.Condition == nil || answers[q.Condition.Key] == q.Condition.Answer
}

// IsAsked reports whether a frontend asks the question. A select with one
// option is not asked, and that option is its answer.
func (q Question) IsAsked() bool {
	return q.Kind != QuestionSelect || len(q.Options) != 1
}

func (q Question) validateOption(answer string) error {
	values := make([]string, 0, len(q.Options))
	for _, option := range q.Options {
		if option.Value == answer {
			return nil
		}
		values = append(values, option.Value)
	}
	return fmt.Errorf("%q is not one of %s", answer, strings.Join(values, ", "))
}

// ResolveAnswers returns the answer of every question whose condition holds,
// resolving the questions in order. A question missing from answers takes its
// default, and a question that is not asked takes its only option. It refuses
// an answer the question does not validate.
func ResolveAnswers(questions []Question, answers Answers) (Answers, error) {
	resolved := make(Answers, len(questions))
	for _, question := range questions {
		if !question.IsAskedAfter(resolved) {
			continue
		}
		answer, ok := answers[question.Key]
		switch {
		case !question.IsAsked():
			answer = question.Options[0].Value
		case !ok:
			answer = question.Default
		}
		if question.Validate != nil {
			if err := question.Validate(answer); err != nil {
				return nil, fmt.Errorf("answer for %s: %w", question.Key, err)
			}
		}
		resolved[question.Key] = answer
	}
	return resolved, nil
}

// findSchemaKey returns the node of a dotted key in schema.
func findSchemaKey(schema []byte, key string) (schemaNode, error) {
	var node schemaNode
	if err := json.Unmarshal(schema, &node); err != nil {
		return schemaNode{}, fmt.Errorf("could not parse the schema: %w", err)
	}
	for name := range strings.SplitSeq(key, schemaKeySeparator) {
		index := slices.IndexFunc(node.Properties, func(property schemaProperty) bool { return property.name == name })
		if index < 0 {
			return schemaNode{}, fmt.Errorf("the schema has no key %q", key)
		}
		node = node.Properties[index].node
	}
	return node, nil
}
