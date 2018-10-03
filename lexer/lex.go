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
	"unicode/utf8"

	"github.com/db47h/parsekit/token"
)

const (
	undoSZ    = 8
	undoMask  = undoSZ - 1
	keepBytes = undoSZ * utf8.UTFMax
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
	buf   [4 << 10]byte // byte buffer
	undo  [undoSZ]int   // undo indices
	queue               // Item queue
	f     *token.File
	line  int       // line count
	state StateFn   // current state
	init  StateFn   // current initial-state function.
	offs  int       // offset of first byte in buffer
	r, w  int       // read/write indices
	u     int       // undo index
	ts    token.Pos // token start position
	ioErr error     // if not nil, IO error @w
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
	s := &state{
		// initial q size must be an exponent of 2
		queue: queue{items: make([]item, 2)},
		f:     f,
		line:  1,
		init:  init,
	}

	f.AddLine(0, 1)          // add line 1 to file
	s.buf[0] = utf8.RuneSelf // sentinel value
	return (*Lexer)(s)
}

// Init (re-)sets the initial state function for the lexer. It can be used by
// state functions to implement context switches (e.g. switch from accepting
// plain text to expressions in a template like language). This function returns
// its argument.
//
func (s *State) Init(initState StateFn) StateFn {
	s.init = initState
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
			l.state = l.init(st)
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
//
func (s *State) Emit(t token.Type, value interface{}) {
	s.push(t, s.ts, value)
}

// Errorf emits an error token. The Item value is set to a string representation
// of the error and the position set to pos.
//
func (s *State) Errorf(pos token.Pos, format string, args ...interface{}) {
	s.push(token.Error, pos, fmt.Sprintf(format, args...))
}

// Next returns the next rune in the input stream. IF the end of the stream
// has ben reached or an IO error occurred, it will return EOF.
//
func (s *State) Next() rune {
again:
	for s.r+utf8.UTFMax > s.w && !utf8.FullRune(s.buf[s.r:s.w]) && s.ioErr == nil {
		s.fill()
	}

	// Common case: ASCII
	// Invariant: s.buf[s.w] == utf8.RuneSelf
	if b := s.buf[s.r]; b < utf8.RuneSelf {
		s.r++
		s.u = (s.u + 1) & undoMask
		s.undo[s.u] = 1
		if b == 0 {
			s.Errorf(s.Pos(), "invalid NUL character")
			goto again
		}
		if b == '\n' {
			s.line++
			s.f.AddLine(s.Pos()+1, s.line)
		}
		return rune(b)
	}

	// EOF
	if s.r == s.w {
		// EOF has 0 length.
		// Add EOF to undo buffer only if not already 0
		if s.undo[s.u] != 0 {
			s.u = (s.u + 1) & undoMask
			s.undo[s.u] = 0
		}
		if s.ioErr != io.EOF {
			s.Errorf(s.Pos(), "I/O error: "+s.ioErr.Error())
		}
		return EOF
	}

	// UTF8
	r, w := utf8.DecodeRune(s.buf[s.r:s.w])
	s.r += w
	s.u = (s.u + 1) & undoMask
	s.undo[s.u] = w
	if r == utf8.RuneError && w == 1 {
		s.Errorf(s.Pos(), "invalid UTF-8 encoding")
		goto again
	}

	// BOM only allowed as first rune in the file
	const BOM = 0xfeff
	if r == BOM {
		if s.Pos() > 0 {
			s.Errorf(s.Pos(), "invalid BOM in the middle of the file")
		}
		goto again
	}

	return r
}

func (s *State) fill() {
	// slide buffer contents
	if s.r > keepBytes {
		// keep keepBytes in buffer for Backups()
		n := s.r - keepBytes
		copy(s.buf[:], s.buf[n:s.w])
		s.offs += n
		s.r = keepBytes
		s.w -= n
	}

	for i := 0; i < 100; i++ {
		n, err := s.f.Read(s.buf[s.w : len(s.buf)-1]) // -1 to leave space for sentinel
		s.w += n
		if n > 0 || err != nil {
			s.buf[s.w] = utf8.RuneSelf // sentinel
			if err != nil {
				s.ioErr = err
			}
			return
		}
	}

	s.ioErr = io.ErrNoProgress
}

func (s *State) updateStart() {
	s.ts = token.Pos(s.offs + s.r)
}

// Peek returns the next rune in the input stream without consuming it. This
// is equivalent to calling Next followed by Backup.
//
func (s *State) Peek() rune {
	r := s.Next()
	s.Backup()
	return r
}

// Backup reverts the last call to Next. Backup does not check if the number of
// backup levels has been exceeded. Use with caution.
// TODO: implement check.
// It will be a silent no-op if trying to backup past the start of the file.
//
func (s *State) Backup() {
	// if s.r == 0 && EOF in undo buffer (s.undo[s.u] == 0) => valid undo
	// we should decrement s.u. However, no further backup is possible and
	// subsequent Next() will still return EOF so we don't check this condition.
	if s.r > 0 {
		s.r -= s.undo[s.u]
		s.u = (s.u - 1) & undoMask
	}
}

// Pos returns the position (rune offset) of the current rune. Returns -1
// if no input has been read yet.
//
func (s *State) Pos() token.Pos {
	return token.Pos(s.offs + s.r - s.undo[s.u])
}

// AcceptWhile accepts input while the f function returns true. The return value
// is the number of runes accepted.
//
func (s *State) AcceptWhile(f func(r rune) bool) int {
	i := 0
	for r := s.Next(); f(r); r = s.Next() {
		i++
	}
	s.Backup()
	return i
}
