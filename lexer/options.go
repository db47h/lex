package lexer

import (
	"fmt"
	"unicode"

	"github.com/db47h/asm/token"
)

type options struct {
	isSeparator  func(token.Token, rune) bool
	isIdentifier func(rune, bool) bool
	errorHandler func(l *Lexer, err error)
}

// An Option is a configuration option for a new Lexer.
//
type Option func(*options)

// IsSeparator defines a custom IsSeparator function. The token parameter is
// a contextual hint. The function should return true if the rune is to be
// considered a token separator in the given context.
// When lexing identifiers, IsSeparator is always checked before IsIdentifier.
//
func IsSeparator(f func(token.Token, rune) bool) Option {
	return func(o *options) {
		o.isSeparator = f
	}
}

// IsIdentifier defines a custom IsIdentifier function.
// The boolean parameter first is true if the rune to be checked
// is the first rune of the identifier.
//
func IsIdentifier(f func(r rune, first bool) bool) Option {
	return func(o *options) {
		o.isIdentifier = f
	}
}

// ErrorHandler defines a custom error handler callback.
// It is called whenever an io error (other than io.EOF) occurs during lexing.
// If no error handler is defined, the lexer will panic on io errors.
//
func ErrorHandler(f func(l *Lexer, err error)) Option {
	return func(o *options) {
		o.errorHandler = f
	}
}

func defErrorHandler(l *Lexer, err error) {
	line, col := l.f.Position(l.n - 1)
	panic(fmt.Errorf("%s:%d:%d io error \"%s\"", l.f.Name(), line, col, err))
}

func defIsSeparator(_ token.Token, r rune) bool {
	// This needs updating if we add symbols to the syntax
	// these are valid characters immediately following (and marking the end of)
	// any token.
	switch r {
	case '(', ')', '[', ']', '\\', ',', ';', '+', '-', '*', '/', '%', '&', '|', '^', ':':
		return true
	case eof:
		return true
	default:
		return unicode.IsSpace(r)
	}
}

func defIsIdentifier(r rune, first bool) bool {
	if first {
		return unicode.IsLetter(r) || r == '_'
	}
	return unicode.IsPrint(r)
}
