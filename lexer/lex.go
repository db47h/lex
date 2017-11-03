// Package lexer implements a lexer for assembler source text.
//
// This is a concurrent lexer, similar to https://golang.org/src/text/template/parse/lex.go.
//
// Despite its concurrent architecture, it essentially behaves as any other
// lexer from an API standpoint.
//
package lexer

import (
	"errors"
	"fmt"
	"io"

	"github.com/db47h/asm/token"
)

// EOF is the return value from Next() when EOF is reached.
const EOF = -1

var errDone = errors.New("close requested")

// Item represents a token returned from the lexer.
//
type Item struct {
	token.Token
	token.Pos // Token start position within the file.
	Value     interface{}
}

// String returns a string representation of the item. This should be used only for debugging purposes as
// the output format is not guaranteed to be stable.
//
func (i *Item) String() string {
	switch v := i.Value.(type) {
	case string:
		return fmt.Sprintf("%s %s", i.Token, v)
	case interface {
		String() string
	}:
		return fmt.Sprintf("%s %s", i.Token, v.String())
	default:
		return i.Token.String()
	}
}

// A Lexer holds the internal state of the lexer while processing a given text.
// Note that the public member functions should only be accessed from custom StateFn
// functions. Parsers should only call Lex() and Close().
//
type Lexer struct {
	f    *token.File
	l    *Lang
	err  func(*Lexer, error)
	B    []rune    // token string buffer
	s    token.Pos // token start position
	n    token.Pos // position of next rune read
	c    chan Item
	done chan struct{}
	T    token.Token // current token guess
}

// A StateFn is a state function. When a StateFn is called, the input that lead top that
// state has already been scanned.
//
type StateFn func(l *Lexer) StateFn

// New creates a new lexer associated with the given source file.
//
func New(f *token.File, l *Lang, opts ...Option) *Lexer {
	o := options{
		errorHandler: defErrorHandler,
	}
	for _, f := range opts {
		f(&o)
	}
	lx := &Lexer{
		f:    f,
		l:    l,
		c:    make(chan Item),
		done: make(chan struct{}),
		err:  o.errorHandler,
	}
	go lx.run()
	return lx
}

func (l *Lexer) run() {
	for state := StateAny; state != nil; state = state(l) {
	}
	close(l.c)
}

// Lex reads source text and returns the next item until EOF. Once this
// function has returned an EOF token, any further calls to it will panic.
//
func (l *Lexer) Lex() Item {
	if i, ok := <-l.c; ok {
		return i
	}
	// l.c has been closed
	return Item{
		Token: token.EOF,
		Pos:   l.n - 1,
		Value: nil,
	}
}

// Close stops the lexer. This function should always be called once the lexer
// is no longer needed.
//
func (l *Lexer) Close() {
	if l.done != nil {
		close(l.done)
	}
}

// File returns the token.File used as input for the lexer.
//
func (l *Lexer) File() *token.File {
	return l.f
}

// Emit emits a single token. Returns the next state or nil depending on the success of the operation.
//
func (l *Lexer) Emit(t token.Token, value interface{}, nextState StateFn) StateFn {
	if err := l.EmitItem(Item{
		Token: t,
		Pos:   l.s,
		Value: value,
	}); err != nil {
		return nil
	}
	return nextState
}

// EmitItem emits the given item. Returns errDone if the lexer has been aborted
// or the last token is EOF, in which case the caller (usually a StateFn) should
// return nil to abort the lexer's loop.
//
func (l *Lexer) EmitItem(i Item) error {
	if i.Token == token.Invalid {
		panic("attempt to emit a None token")
	}
	for {
		select {
		case l.c <- i:
			// Reuse the l.t slice.
			// We could end up with a large-ish slice hanging around (i.e. as
			// big as the largest lexed token), but this limits garbage collection.
			l.Discard()
			if i.Token != token.EOF {
				return nil
			}
			return errDone
		case <-l.done:
			return errDone
		}
	}
}

// Errorf emits an error token. The Item value is set to a string representation
// of the error. Returns errDone if the lexer has been aborted (see EmitItem).
//
func (l *Lexer) Errorf(format string, args ...interface{}) error {
	i := Item{
		Token: token.Error,
		Pos:   l.n - 1, // that's where the error actually occurred
		Value: fmt.Sprintf(format, args...),
	}
	return l.EmitItem(i)
}

// Next returns the next rune in the input stream.
//
func (l *Lexer) Next() rune {
	r, s, err := l.f.ReadRune()
	switch {
	case s == 0 || err == io.EOF:
		r = EOF
	case err != nil:
		r = EOF
		defer l.err(l, err)
	}
	l.n++
	if r == '\n' {
		l.f.AddLine(l.n)
	}
	l.B = append(l.B, r)
	return r
}

// Backup reverts the last call to next().
//
func (l *Lexer) Backup() {
	ln := len(l.B) - 1
	if ln < 0 {
		panic("invalid use of backup")
	}
	l.B = l.B[:ln]
	l.n--
	if err := l.f.UnreadRune(); err != nil {
		panic(err)
	}
}

// BackupN reverts multiple calls to Next().
//
func (l *Lexer) BackupN(n int) {
	ln := len(l.B) - n
	if ln < 0 {
		panic("invalid use of undo")
	}
	l.B = l.B[:ln]
	l.n -= token.Pos(n)
	for ; n > 0; n-- {
		if err := l.f.UnreadRune(); err != nil {
			panic(err)
		}
	}
}

// Discard discards the current token
//
func (l *Lexer) Discard() {
	l.B = l.B[:0]
	l.T = token.Invalid
	l.s = l.n
}

// TokenString returns a string representation of the current token.
//
func (l *Lexer) TokenString() string {
	return string(l.B)
}

func (l *Lexer) search(r rune) *node {
	var match *node
	var i, mi = 0, 0

	for n := l.l.e.match(r); n != nil; n = n.match(r) {
		if n.s != nil {
			mi = i
			match = n
		}
		if len(n.c) == 0 {
			// skip unnecessary Next() / Backup() steps
			break
		}
		i++
		r = l.Next()
	}
	// backup runes not part of the match
	i -= mi
	if i > 0 {
		l.BackupN(i)
	}
	return match
}

// AcceptWhile accepts input while the f function returns true.
//
// The first rune for which f will return false will not be included in the token.
//
func (l *Lexer) AcceptWhile(f func(r rune) bool) {
	for r := l.Next(); f(r); r = l.Next() {
	}
	l.Backup()
}

// AcceptUpTo returns a StateFn that accepts input until it matches s.
//
func (l *Lexer) AcceptUpTo(s []rune) bool {
	here := len(l.B)
	for {
		r := l.Next()
		if r == EOF {
			l.Backup()
			return false
		}
		var i, j int
		for i, j = len(l.B)-1, len(s)-1; i >= here && j >= 0 && l.B[i] == s[j]; i, j = i-1, j-1 {
		}
		if j < 0 {
			return true
		}
	}
}

// TODO: add `Accept([]rune) bool` (consumes it) and Expect([]rune) bool (does not consume)
