// Package lexer implements a customizable lexer.
//
// The lexer state is implemented as state functions and it supports asynchronous
// token emission.
//
// Besides the fact that it can be customized, the implementation is similar to
// https://golang.org/src/text/template/parse/lex.go.
// Se also Rob Pike's talk about it: https://talks.golang.org/2011/lex.slide.
// The key difference with the lexer in the Go template package is that it does
// not use Go channels to emit tokens. The asynchronous token emission is
// implemented via a FIFO queue. This is about 5 to 6 times faster than channels
// in this specific case (channels are great, just not here).
//
package lexer

import (
	"fmt"
	"io"

	"github.com/db47h/asm/token"
)

type queue struct {
	items []Item
	head  int
	tail  int
	count int
}

func (q *queue) push(i Item) {
	if q.head == q.tail && q.count > 0 {
		items := make([]Item, len(q.items)*2)
		copy(items, q.items[q.head:])
		copy(items[len(q.items)-q.head:], q.items[:q.head])
		q.head = 0
		q.tail = len(q.items)
		q.items = items
	}
	q.items[q.tail] = i
	q.tail = (q.tail + 1) % len(q.items)
	q.count++
}

// check that q.count > 0 before calling pop
func (q *queue) pop() Item {
	i := q.head
	q.head = (q.head + 1) % len(q.items)
	q.count--
	return q.items[i]
}

// EOF is the return value from Next() when EOF is reached.
const EOF = -1

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
	f     *token.File
	l     *Lang
	B     []rune      // token string buffer
	s     token.Pos   // token start position
	n     token.Pos   // position of next rune read
	T     token.Token // current token guess
	q     *queue      // Item queue
	state StateFn
}

// A StateFn is a state function. When a StateFn is called, the input that lead top that
// state has already been scanned.
//
type StateFn func(l *Lexer) StateFn

// New creates a new lexer associated with the given source file.
//
func New(f *token.File, l *Lang) *Lexer {
	return &Lexer{
		f: f,
		l: l,
		// initial q size must be an exponent of 2
		q:     &queue{items: make([]Item, 1)},
		state: StateAny,
	}
}

// Lex reads source text and returns the next item until EOF. Once this
// function has returned an EOF token, any further calls to it will panic.
//
func (l *Lexer) Lex() Item {
	for l.q.count == 0 {
		l.state = l.state(l)
	}
	return l.q.pop()
}

// File returns the token.File used as input for the lexer.
//
func (l *Lexer) File() *token.File {
	return l.f
}

// Emit emits a single token.
//
func (l *Lexer) Emit(t token.Token, value interface{}) {
	l.q.push(Item{
		Token: t,
		Pos:   l.s,
		Value: value,
	})
}

// Errorf emits an error token. The Item value is set to a string representation
// of the error.
//
func (l *Lexer) Errorf(format string, args ...interface{}) {
	l.q.push(Item{
		Token: token.Error,
		Pos:   l.n - 1, // that's where the error actually occurred
		Value: fmt.Sprintf(format, args...),
	})
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
		l.Errorf(err.Error())
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

// Discard discards the current token.
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
