// Copyright 2017 Denis Bernard <db047h@gmail.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package lexer

import (
	"fmt"
	"io"

	"github.com/db47h/parsekit/token"
)

// queue is a FIFO queue.
//
type queue struct {
	items []item
	head  int
	tail  int
	count int
}

func (q *queue) push(t token.Type, p token.Pos, v interface{}) {
	if q.head == q.tail && q.count > 0 {
		items := make([]item, len(q.items)*2)
		copy(items, q.items[q.head:])
		copy(items[len(q.items)-q.head:], q.items[:q.head])
		q.head = 0
		q.tail = len(q.items)
		q.items = items
	}
	q.items[q.tail] = item{t, p, v}
	q.tail = (q.tail + 1) % len(q.items)
	q.count++
}

// pop pops the first item from the queue. Callers must check that q.count > 0 beforehand.
//
func (q *queue) pop() (token.Type, token.Pos, interface{}) {
	i := q.head
	q.head = (q.head + 1) % len(q.items)
	q.count--
	it := &q.items[i]
	return it.t, it.p, it.v
}

// EOF is the return value from Next() when EOF is reached.
//
const EOF = -1

// Item represents a token returned from the lexer.
//
type item struct {
	t token.Type
	p token.Pos
	v interface{}
}

// Lexer wraps the public methods of a lexer. This interface is intended for
// parsers that call New(), then Lex() until EOF.
//
type Lexer state

// State holds the internal state of the lexer while processing a given input.
// Note that the public fields should only be accessed from custom StateFn
// functions.
//
type State state

type state struct {
	queue // Item queue
	// Start position of current token. Used as token position by Emit.
	// calling Emit or returning nil from a StateFn resets its value to Pos() + 1.
	// State functions should normally not need to read or change this value.
	S token.Pos

	f     *token.File
	n     token.Pos // position of next rune to read
	l     int       // line count
	state StateFn   // current state
	i     StateFn   // current initial-state function.
	r     rune      // last rune read by Next
	p     rune      // previous rune
	b     bool      // true if backed-up
}

// A StateFn is a state function.
//
// If a StateFn returns nil, the lexer resets the current token start position
// then transitions back to its initial state function.
//
type StateFn func(l *State) StateFn

// New creates a new lexer associated with the given source file. A new lexer
// must be created for every source file to be lexed.
//
func New(f *token.File, init StateFn) *Lexer {
	// add line 1 to file
	f.AddLine(0, 1)
	return (*Lexer)(&state{
		// initial q size must be an exponent of 2
		queue: queue{items: make([]item, 2)},
		f:     f,
		i:     init,
	})
}

// Init (re-)sets the initial state function for the lexer. It can be used by
// state functions to implement context switches (e.g. switch from accepting
// plain text to expressions in a template like language). This function returns
// its argument.
//
func (l *State) Init(initState StateFn) StateFn {
	l.i = initState
	return initState
}

// Lex reads source text and returns the next item until EOF.
//
// As a convention, once the end of file has been reached (or some
// non-recoverable error condition), Lex() must return an item of type
// token.EOF. Implementors of custom lexers must take care of this.
//
func (l *Lexer) Lex() (token.Type, token.Pos, interface{}) {
	for l.count == 0 {
		st := (*State)(l)
		if l.state == nil {
			st.updateStart()
			l.state = l.i(st)
		} else {
			l.state = l.state(st)
		}
	}
	return l.pop()
}

// File returns the token.File used as input for the lexer.
//
func (l *Lexer) File() *token.File {
	return l.f
}

// Emit emits a single token of the given type and value positioned at l.S.
// Emit sets l.S to the start of the next token (i.e. l.Pos() + 1).
//
func (l *State) Emit(t token.Type, value interface{}) {
	l.push(t, l.S, value)
	l.updateStart()
}

// Errorf emits an error token. The Item value is set to a string representation
// of the error and the position set to pos.
//
func (l *State) Errorf(pos token.Pos, format string, args ...interface{}) {
	l.push(token.Error, pos, fmt.Sprintf(format, args...))
}

func (l *State) next() rune {
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

// Next returns the next rune in the input stream. IF the end of the stream
// has ben reached or an IO error occurred, it will return EOF.
//
func (l *State) Next() rune {
	if l.b {
		l.b = false
		return l.r
	}
	if l.r == EOF {
		return EOF
	}
	return l.next()
}

// Peek returns the next rune in the input stream without consuming it. This
// is equivalent to calling Next followed by Backup.
//
func (l *State) Peek() rune {
	if l.b {
		return l.r
	}
	if l.r == EOF {
		return EOF
	}
	l.b = true
	return l.next()
}

// Backup reverts the last call to Next.
//
func (l *State) Backup() {
	if l.b || l.n == 0 {
		panic("cannot backup twice in a row")
	}
	l.b = true
}

// updateStart sets l.S to l.Pos() + 1.
//
func (l *State) updateStart() {
	if l.b {
		l.S = l.n - 1
	} else {
		l.S = l.n
	}
}

// Pos returns the position (rune offset) of the current rune. Returns -1
// if no input has been read yet.
//
func (l *State) Pos() token.Pos {
	if l.b {
		return l.n - 2
	}
	return l.n - 1
}

// Current returns the current rune. This is the last rune read by Next
// or the previous one if Backup has been called after Next.
//
func (l *State) Current() rune {
	if l.b {
		return l.p
	}
	return l.r
}

// AcceptWhile accepts input while the f function returns true. The return value
// is the number of runes accepted.
//
func (l *State) AcceptWhile(f func(r rune) bool) int {
	i := 0
	for r := l.Next(); f(r); r = l.Next() {
		i++
	}
	l.Backup()
	return i
}

// Accept accepts input if it matches r. Returns true if successful.
//
func (l *State) Accept(r rune) bool {
	if l.Peek() != r {
		return false
	}
	l.Next()
	return true
}
