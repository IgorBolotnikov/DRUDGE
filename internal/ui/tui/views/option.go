package views

// OptionType is the kind of value an option of a View takes.
type OptionType string

// OptionString takes a JSON string.
const OptionString OptionType = "string"

// Option is one setting of a View that a Tile sets in its view options. An
// option with allowed values refuses any other value. An empty default leaves
// the option unset.
type Option struct {
	Name        string
	Type        OptionType
	Allowed     []string
	Default     string
	Description string
}

// OptionValues holds the value of every option of a View by option name. An
// option the Tile leaves out holds its default.
type OptionValues map[string]string
