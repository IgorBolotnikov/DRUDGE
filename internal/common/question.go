package common

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// QuestionKind decides how a frontend asks a question and reads its answer.
type QuestionKind string

const (
	QuestionSelect QuestionKind = "select"
	QuestionNumber QuestionKind = "number"
	QuestionText   QuestionKind = "text"
)

// Question is one config setting a frontend asks the user for.
type Question struct {
	Key     string
	Label   string
	Kind    QuestionKind
	Options []QuestionOption // The choices of a select
	Default string
	// Validate refuses an answer the config key cannot take. Nil takes every
	// answer.
	Validate func(answer string) error
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
// the enum.
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
	if len(node.Default) > 0 {
		if err := json.Unmarshal(node.Default, &question.Default); err != nil {
			return Question{}, fmt.Errorf("default of key %q is %s, want a string: %w", key, node.Default, err)
		}
	}
	question.Validate = question.validateOption
	if err := question.Validate(question.Default); err != nil {
		return Question{}, fmt.Errorf("default of key %q: %w", key, err)
	}
	return question, nil
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

// ResolveAnswers returns the answer of every question. A question missing
// from answers takes its default, and a question that is not asked takes its
// only option. It refuses an answer the question does not validate.
func ResolveAnswers(questions []Question, answers Answers) (Answers, error) {
	resolved := make(Answers, len(questions))
	for _, question := range questions {
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
