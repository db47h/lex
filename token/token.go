// Package token defines constants and types representing lexical tokens
// as well as a wrapper for input files.
//
package token

//go:generate stringer -type Type

// Type represents a token's type. Custom lexers can use token types
// greater than zero.
//
type Type int

// Reserved token types.
//
const (
	Invalid Type = -1 - iota // invalid token. Emitting a token of this type is illegal.
	EOF                      // end of file
	Error                    // error -- the associated value is a string
)

// Pos represents a token's position within a File. This is a rune index rather
// than a byte index.
// For error reporting, this is not really an issue even for editors that do not
// support rune-indexing since after conversion to the line:column based
// Position, the line number is accurate and the column might be off by only a
// few bytes.
//
type Pos int

// IsValid returns true if p is a valid position (i.e. p >= 0).
//
func (p Pos) IsValid() bool {
	return p >= 0
}
