// Package token defines constants and types representing lexical tokens
// in assembler source text.
//
package token

//go:generate stringer -type Token

// Token represents a token's numeric ID
//
type Token uint

// Token IDs
//
const (
	EOF          Token = iota // end of file
	EOL                       // end of line
	Error                     // error -- the associated value is a string
	Identifier                // any valid identifier
	Immediate                 // immediate
	Space                     // unicode.IsSpace() == true
	Comment                   // ; -- and skip to EOL
	LeftParen                 // (
	RightParen                // )
	LeftBracket               // [
	RightBracket              // ]
	Colon                     // :
	Comma                     // ,
	Dot                       // .
	Backslash                 // \
	OpPlus                    // +
	OpMinus                   // -
	OpFactor                  // *
	OpDiv                     // /
	OpMod                     // %
	OpXor                     // ^
	OpAnd                     // &
	OpOr                      // |
)

// Pos represents a token's position within a File
//
type Pos int

// IsValid returns true if p is a valid position.
//
func (p Pos) IsValid() bool {
	return p >= 0
}
