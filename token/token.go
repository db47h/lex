// Package token defines constants and types representing lexical tokens
// in assembler source text.
//
package token

//go:generate stringer -type Token

// Token represents a token's numeric ID
//
type Token int

// Token IDs
//
const (
	Invalid Token = iota
	EOF           // end of file
	Error         // error -- the associated value is a string
	RawChar       // unknown raw character
	Custom        // any token value >= Custom may be used as custom tokens
)

// Pos represents a token's position within a File
//
type Pos int

// IsValid returns true if p is a valid position.
//
func (p Pos) IsValid() bool {
	return p >= 0
}
