// Package lexer implements a configurable lexer.
//
// The lexer state is implemented with state functions and it supports
// asynchronous token emission.
//
// The implementation is similar to https://golang.org/src/text/template/parse/lex.go.
// Se also Rob Pike's talk about it: https://talks.golang.org/2011/lex.slide.
//
// The key difference with the lexer of the Go text template package is that the
// asynchronous token emission is implemented with a FIFO queue instead of using
// Go channels. Benchmarks with an earlier implementation that used a channel
// showed that the using FIFO is about 5 times faster.
//
// The drawback of using a FIFO is that once Emit() has been called from a state
// function, the sent item will be received by the caller (parser) only when the
// state function returns, so it must return as soon as possible.
//
// It performs at about a third of the speed of the Go lexer (for Go source
// code) and it's on-par with the Go text template lexer where the performance
// gain from using a FIFO is counter-balanced by the language configuration
// overhead.
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
	token.Type
	token.Pos // Token start position within the file.
	Value     interface{}
}

// String returns a string representation of the item. This should be used only
// for debugging purposes as the output format is not guaranteed to be stable.
//
func (i *Item) String() string {
	switch v := i.Value.(type) {
	case string:
		return fmt.Sprintf("%v %s", i.Type, v)
	case interface {
		String() string
	}:
		return fmt.Sprintf("%v %s", i.Type, v.String())
	default:
		return fmt.Sprintf("%v", i.Type) // i.Token.String()
	}
}

// A Lexer holds the internal state of the lexer while processing a given text.
// Note that the public fields should only be accessed from custom StateFn
// functions. Parsers should only call Lex().
//
type Lexer struct {
	f     *token.File
	L     *Lang      // current language
	T     token.Type // current token type
	b     []rune     // token string buffer
	s     token.Pos  // token start position
	n     token.Pos  // position of next rune read
	q     *queue     // Item queue
	state StateFn
}

// A StateFn is a state function. When a StateFn is called, the input that lead
// to that state has already been scanned and can be retrieved with Lexer.Token().
//
type StateFn func(l *Lexer) StateFn

// New creates a new lexer associated with the given source file.
//
func New(f *token.File, l *Lang) *Lexer {
	return &Lexer{
		f: f,
		L: l,
		// initial q size must be an exponent of 2
		q: &queue{items: make([]Item, 1)},
	}
}

// Lex reads source text and returns the next item until EOF.
//
func (l *Lexer) Lex() Item {
	for l.q.count == 0 {
		if l.state == nil {
			l.state = l.L.doMatch(l)
		} else {
			l.state = l.state(l)
		}
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
func (l *Lexer) Emit(t token.Type, value interface{}) {
	l.q.push(Item{
		Type:  t,
		Pos:   l.s,
		Value: value,
	})
	l.Discard()
}

// Errorf emits an error token. The Item value is set to a string representation
// of the error and the position set to the position of the last rune read by Next().
//
func (l *Lexer) Errorf(format string, args ...interface{}) {
	l.q.push(Item{
		Type:  token.Error,
		Pos:   l.n - 1, // that's where the error actually occurred
		Value: fmt.Sprintf(format, args...),
	})
	l.Discard()
}

// Next returns the next rune in the input stream.
//
func (l *Lexer) Next() rune {
	if sz := l.TokenLen(); sz < len(l.b) {
		r := l.b[sz]
		l.n++
		return r
	}
	r, s, err := l.f.ReadRune()
	switch {
	case s == 0:
		r = EOF
	case err != nil && err != io.EOF:
		r = EOF
		l.Errorf(err.Error())
	}
	l.n++
	if r == '\n' {
		l.f.AddLine(l.n)
	}
	l.b = append(l.b, r)
	return r
}

// Backup reverts the last call to next().
//
func (l *Lexer) Backup() {
	if l.n <= l.s {
		panic("invalid use of Backup")
	}
	l.n--
}

// BackupN reverts multiple calls to Next().
//
func (l *Lexer) BackupN(n int) {
	if l.TokenLen() < n {
		panic("invalid use of BackupN")
	}
	l.n -= token.Pos(n)
}

// Discard discards the current token.
//
func (l *Lexer) Discard() {
	l.b = l.b[:copy(l.b, l.b[l.TokenLen():])]
	l.T = token.Invalid
	l.s = l.n
}

// Token returns the current token as a rune slice.
//
func (l *Lexer) Token() []rune {
	return l.b[:l.TokenLen()]
}

// TokenString returns a string representation of the current token.
//
func (l *Lexer) TokenString() string {
	return string(l.b[:l.TokenLen()])
}

// TokenLen returns the length of the current token.
//
func (l *Lexer) TokenLen() int {
	return int(l.n - l.s)
}

// Last returns the last rune read. May panic if called without a previous call
// to Next() since the last Discard(), Emit() or Errorf().
//
func (l *Lexer) Last() rune {
	return l.b[l.TokenLen()-1]
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

// AcceptUpTo accepts input until it finds an occurence of s.
//
func (l *Lexer) AcceptUpTo(s []rune) bool {
	if len(s) == 0 {
		return true
	}
	here := l.TokenLen()
	for {
		r := l.Next()
		if r == EOF {
			l.Backup()
			return false
		}
		var i, j int
		for i, j = l.TokenLen()-1, len(s)-1; i >= here && j >= 0 && l.b[i] == s[j]; i, j = i-1, j-1 {
		}
		if j < 0 {
			return true
		}
	}
}

// Accept accepts input if it matches the contents of s.
// Return true if a match was found.
//
func (l *Lexer) Accept(s []rune) bool {
	var i int
	if len(s) == 0 {
		return true
	}
	for i = 0; i < len(s); i++ {
		r := l.Next()
		if r != s[i] {
			break
		}
	}
	if i == len(s) {
		return true
	}
	l.BackupN(i + 1)
	return false
}

// Expect checks that input matches the contents of s but does not
// consume it.
//
func (l *Lexer) Expect(s []rune) bool {
	var i int
	if len(s) == 0 {
		return false
	}
	for i = 0; i < len(s); i++ {
		r := l.Next()
		if r != s[i] {
			break
		}
	}
	if i == len(s) {
		l.BackupN(i)
		return true
	}
	l.BackupN(i + 1)
	return false
}
