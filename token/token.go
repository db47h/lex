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
	LeftParen                 // (
	RightParen                // )
	LeftBracket               // [
	RightBracket              // ]
	Colon                     // :
	Comma                     // ,
	Space                     //
	Comment                   // ; -- and skip to EOL
	Dot                       // '.'
	Identifier                // any valid identifier
	Immediate                 // immediate
	LocalLabel                // local label (1f, 0b, etc.)
	Backslash                 // \
	OpPlus                    // +
	OpMinus                   // -
	OpFactor                  // *
	OpDiv                     // /
	OpMod                     // %
	OpXor                     // ^, also used as unary 'not'
	OpAnd                     // &
	OpOr                      // |
	BuiltIn                   // %identifier not identified by the lexer but by the parser
)
