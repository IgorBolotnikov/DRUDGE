package common

// Progress is the port a domain service reports its work through. Each event
// is a type its own package defines and carries the data of what happened.
// The port carries no wording, so a renderer decides how an event reads.
type Progress interface {
	Report(event any)
}
