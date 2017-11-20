// Package lexer provides boiler plate code to quickly build lexers using
// state functions.
//
// The implementation is similar to https://golang.org/src/text/template/parse/lex.go.
// Se also Rob Pike's talk about it: https://talks.golang.org/2011/lex.slide.
//
// The package provides facilities to read input from a RuneReader (this will be
// chaged in the future to a simple io.Reader) with unlimited look-ahead, as
// well as utility functions commonly used in lexers.
//
// While the package could be used as-is with hand-crafted state functions,
// the types and state functions provided in the subpackages lang and state make
// building a new lexer even faster (and painless).
//
// Implementation details:
//
// Asynchronous token emission is implemented with a FIFO queue instead of using
// Go channels (like in the Go text template package). Benchmarks with an
// earlier implementation that used a channel showed that using a FIFO is about
// 5 times faster.
//
// The drawback of using a FIFO is that once Emit() has been called from a state
// function, the sent item will be received by the caller (parser) only when the
// state function returns, so it must return as soon as possible.
//
// Combined with the lexer/lang package, it performs at about a third of the
// speed of the Go lexer (for Go source code) and it's on-par with the Go text
// template lexer where the performance gain from using a FIFO is
// counter-balanced by overhead from the lang package as well as the undo buffer
// not needed in this specific case.
//
package lexer

import (
	"fmt"
	"io"

	"github.com/db47h/parsekit/token"
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
//
const EOF = -1

// Item represents a token returned from the lexer.
//
type Item struct {
	// Token type. Can be any user-defined value, token.EOF or token.Error.
	token.Type
	// Token start position within the file (in runes).
	token.Pos
	// The value type us user-defined for token types > 0.
	// For built-in token types, this is nil except for errors where Value
	// is a string describing the error.
	Value interface{}
}

// String returns a string representation of the item. This should be used only
// for debugging purposes as the output format is not guaranteed to be stable.
//
func (i *Item) String() string {
	switch v := i.Value.(type) {
	case string:
		return fmt.Sprintf("%v %s", i.Type, v)
	case fmt.Stringer:
		return fmt.Sprintf("%v %s", i.Type, v.String())
	default:
		return fmt.Sprintf("%v", i.Type) // i.Token.String()
	}
}

// Interface wraps the public methods of a lexer.
//
type Interface interface {
	Lex() Item         // Lex reads source text and returns the next item until EOF.
	File() *token.File // File returns the token.File used as input for the lexer.
}

// A Lexer holds the internal state of the lexer while processing a given input.
// Note that the public fields should only be accessed from custom StateFn
// functions. Parsers should only call Lex().
//
type Lexer struct {
	// Current initial-state function. This can be used by state functions to
	// implement context switches (e.g. switch to a JS lexer while parsing HTML, etc.)
	I StateFn
	// Start position of current token. Used as token position by Emit.
	// Emit, Discard or returning nil from a StateFn resets its value to Pos() + 1.
	// State functions should normally not need to adjust this value.
	S token.Pos

	f     *token.File
	n     token.Pos // position of next rune read
	l     int       // line count
	q     *queue    // Item queue
	state StateFn
	r     rune // last rune read
	p     rune // previous rune
	b     bool // true if backed-up
}

// A StateFn is a state function. When a StateFn is called, the input that lead
// to that state has already been scanned and can be retrieved with Lexer.Token().
// If a StateFn returns nil the lexer transitions back to its initial state
// function.
//
type StateFn func(l *Lexer) StateFn

// New creates a new lexer associated with the given source file.
//
func New(f *token.File, init StateFn) Interface {
	// add line 1 to file
	f.AddLine(0, 1)
	return &Lexer{
		f: f,
		I: init,
		// initial q size must be an exponent of 2
		q: &queue{items: make([]Item, 2)},
	}
}

// Lex reads source text and returns the next item until EOF.
//
func (l *Lexer) Lex() Item {
	for l.q.count == 0 {
		if l.state == nil {
			l.Discard()
			l.state = l.I(l)
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
		Pos:   l.S,
		Value: value,
	})
	l.Discard()
}

// Errorf emits an error token. The Item value is set to a string representation
// of the error and the position set to pos.
//
func (l *Lexer) Errorf(pos token.Pos, format string, args ...interface{}) {
	l.q.push(Item{
		Type:  token.Error,
		Pos:   pos, // that's where the error actually occurred
		Value: fmt.Sprintf(format, args...),
	})
}

func (l *Lexer) next() rune {
	r, s, err := l.f.ReadRune()
	l.n++
	switch {
	case s == 0:
		r = EOF
	case err != nil && err != io.EOF:
		r = EOF
		l.Errorf(l.n, err.Error())
	}
	if r == '\n' {
		l.l++
		l.f.AddLine(l.n, l.l+1)
	}
	l.r, l.p = r, l.r
	return r
}

// Next returns the next rune in the input stream.
//
func (l *Lexer) Next() rune {
	if l.b {
		l.b = false
		return l.r
	}
	if l.r == EOF {
		return l.r
	}
	return l.next()
}

// Peek returns the next rune in the input stream without consuming it.
//
func (l *Lexer) Peek() rune {
	if l.b {
		return l.r
	}
	if l.r == EOF {
		return EOF
	}
	l.b = true
	return l.next()
}

// Backup reverts the last call to next().
//
func (l *Lexer) Backup() {
	if l.b || l.n == 0 {
		panic("cannot backup twice in a row")
	}
	l.b = true
}

// Discard discards the current token.
//
func (l *Lexer) Discard() {
	if l.b {
		l.S = l.n - 1
	} else {
		l.S = l.n
	}
}

// Pos returns the position (rune offset) of the last rune read.
//
func (l *Lexer) Pos() token.Pos {
	if l.b {
		return l.n - 2
	}
	return l.n - 1
}

// Last returns the last rune read. May panic if called without a previous call
// to Next() since the last Discard(), Emit() or Errorf().
//
func (l *Lexer) Last() rune {
	if l.b {
		return l.p
	}
	return l.r
}

// AcceptWhile accepts input while the f function returns true. The return value
// is the number of runes accepted.
//
func (l *Lexer) AcceptWhile(f func(r rune) bool) int {
	i := 0
	for r := l.Next(); f(r); r = l.Next() {
		i++
	}
	l.Backup()
	return i
}

// Accept accepts input if it matches r. Returns true if successful.
//
func (l *Lexer) Accept(r rune) bool {
	if l.Peek() != r {
		return false
	}
	l.Next()
	return true
}

// Expect checks that the next rune matches r but does not consume it.
//
func (l *Lexer) Expect(r rune) bool {
	return l.Peek() == r
}
