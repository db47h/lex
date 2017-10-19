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
	EOF        Token = iota // end of file
	EOL                     // end of line
	Error                   // error
	LeftParen               // (
	RightParen              // )
	Colon                   // :
	Comma                   // ,
	Space                   //
	Comment                 // ; - and skip to EOL
	Directive               // .identifier
	BuiltIn                 // %identifier
	Identifier              // any valid identifier
	Immediate               // immediate
	LocalLabel              // local label (1f, 0b, etc.)
)
