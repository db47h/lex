package lexer

import (
	"fmt"
)

type options struct {
	errorHandler func(l *Lexer, err error)
}

// An Option is a configuration option for a new Lexer.
//
type Option func(*options)

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
	panic(fmt.Errorf("%s io error \"%s\"", l.f.Position(l.n-1).String(), err))
}
