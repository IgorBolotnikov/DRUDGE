package common

import (
	"crypto/rand"
	"fmt"
	"regexp"
)

// GenerateUUID generates a random UUID v4
func GenerateUUID() (string, error) {
	var b [16]byte

	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}

	// UUIDv4
	b[6] = (b[6] & 0x0f) | 0x40

	// RFC 9562 variant
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		b[0:4],
		b[4:6],
		b[6:8],
		b[8:10],
		b[10:16],
	), nil
}

// uuidPattern matches a UUID in its canonical hyphenated form.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsUUID reports whether value is a UUID in its canonical hyphenated form.
func IsUUID(value string) bool {
	return uuidPattern.MatchString(value)
}
