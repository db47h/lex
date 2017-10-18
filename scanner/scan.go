// Package scanner implements a scanner for assembler source text.
//
package scanner

import (
	"fmt"
	"io"
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/db47h/asm/token"
)

// Pos represents a token's position
//
type Pos struct {
	Offset int // starts at 0
	Line   int // starts at 1
	Column int // starts at 1
}

// Token represents a scanned token
//
type Token struct {
	token.Token
	Pos
	Raw []byte
}

func (t *Token) String() string {
	if t.Token == token.Error {
		return fmt.Sprintf("%d:%d: %s \"%s\"", t.Pos.Line, t.Pos.Column, t.Token, t.Raw)
	}
	return fmt.Sprintf("%d:%d: %s %q", t.Pos.Line, t.Pos.Column, t.Token, t.Raw)
}

type scanState func(s *Scanner) scanState

// A Scanner holds the scanner internal state while processing a given text.
//
type Scanner struct {
	b    []byte
	t    token.Token // hint for scanIdentifier
	s    Pos         // token start pos
	n    Pos         // next rune to read by next()
	u    Pos         // saved position to undo last call to next()
	c    chan Token
	done chan struct{}
}

// Init readies the scanner to scan a given source.
//
func (s *Scanner) Init(b []byte) {
	s.b = b
	s.s = Pos{
		Line:   1,
		Column: 1,
	}
	s.n = s.s
	s.c = make(chan Token)
	s.done = make(chan struct{})
	go func() {
		state := scanAny
		for state != nil {
			state = state(s)
		}
	}()
}

// Scan scans source text and returns the next token until EOF
//
func (s *Scanner) Scan() Token {
	t := <-s.c
	return t
}

// Close stops the scanner.
//
func (s *Scanner) Close() {
	if s.done != nil {
		close(s.done)
	}
}

// emit emits a single token. The i argument must be either a token.Token or an error.
//
func (s *Scanner) emit(i interface{}) scanState {
	var rv scanState
	var tok Token
	switch t := i.(type) {
	case token.Token:
		tok = Token{
			Token: t,
			Pos:   s.s,
			Raw:   s.b[s.s.Offset:s.n.Offset],
		}
		rv = scanAny
	case error:
		if t == io.EOF {
			tok = Token{
				Token: token.EOF,
				Pos:   s.s,
				Raw:   nil,
			}
			rv = nil
		} else {
			tok = Token{
				Token: token.Error,
				Pos:   s.u, // that's where the error actually occurred
				Raw:   []byte(t.Error()),
			}
			rv = skipToEOL
		}
	default:
		panic("Invalid argument to emit()")
	}
	for {
		select {
		case s.c <- tok:
			s.s = s.n
			return rv
		case <-s.done:
			return nil
		}
	}
}

func (s *Scanner) next() (rune, error) {
	s.u = s.n
	r, sz := utf8.DecodeRune(s.b[s.n.Offset:])
	if r == utf8.RuneError {
		if sz == 0 {
			return r, io.EOF
		}
		s.n.Offset += sz
		return r, fmt.Errorf("invalid rune \\x%02X", s.b[s.n.Offset:])
	}
	s.n.Offset += sz
	if r == '\n' {
		s.n.Line++
		s.n.Column = 1
	} else {
		s.n.Column++
	}
	return r, nil
}

func (s *Scanner) undo() {
	s.n = s.u
}

func scanAny(s *Scanner) scanState {
	r, err := s.next()
	if err != nil {
		return s.emit(err)
	}
	switch r {
	case '\n':
		return s.emit(token.EOL)
	case '(':
		return s.emit(token.LeftParen)
	case ')':
		return s.emit(token.RightParen)
	case ':':
		return s.emit(token.Colon)
	case ',':
		return s.emit(token.Comma)
	case ';':
		return scanComment
	case '.':
		s.t = token.Directive
		return scanIdentifier
	case '%':
		s.t = token.BuiltIn
		return scanIdentifier
	default:
		switch {
		case unicode.IsSpace(r):
			return scanSpace
		case r >= '0' && r <= '9':
			return scanImmediate(r)
		case unicode.IsLetter(r) || r == '_':
			s.t = token.Identifier
			return scanIdentifier
		}
		return s.emit(fmt.Errorf("illegal symbol %s", strconv.QuoteRune(r)))
	}
}

func isWordSeparator(r rune) bool {
	// TODO: this may need updating if we add symbols to the syntax
	// these are valid characters immediately following (and marking the end of) a number
	return r == '(' || r == ')' || r == ':' || r == ',' || unicode.IsSpace(r) || r == ';'
}

func scanSpace(s *Scanner) scanState {
	for {
		r, err := s.next()
		if err == io.EOF {
			// catch EOF next time
			return s.emit(token.Space)
		}
		if unicode.IsSpace(r) && r != '\n' {
			continue
		}
		// revert last rune read
		s.undo()
		return s.emit(token.Space)
	}
}

func scanComment(s *Scanner) scanState {
	for {
		r, err := s.next()
		if err == io.EOF {
			return s.emit(token.Comment)
		}
		if r == '\n' {
			s.undo()
			return s.emit(token.Comment)
		}
	}
}

func scanImmediate(r rune) scanState {
	if r != '0' {
		return scanInt(10)
	}
	return scanBaseX
}

func scanBaseX(s *Scanner) scanState {
	r, err := s.next()
	if err != nil {
		if err == io.EOF {
			return s.emit(token.Immediate)
		}
		return s.emit(err)
	}
	switch {
	case r == 'b' || r == 'B':
		return scanInt(2)
	case r == 'x' || r == 'X':
		return scanInt(16)
	case isWordSeparator(r):
		s.undo()
		return s.emit(token.Immediate)
	case r >= '0' && r <= '9':
		return scanInt(8)
	default:
		return s.emit(fmt.Errorf("illegal symbol %s in immediate value", strconv.QuoteRune(r)))
	}
}

func wellFormedInt(n []byte, base int) interface{} {
	if (base == 2 || base == 10) && len(n) > 0 || len(n) > 2 {
		return token.Immediate
	}
	return fmt.Errorf("malformed immediate value %q", n)
}

func scanInt(base int) scanState {
	return func(s *Scanner) scanState {
		for {
			r, err := s.next()
			if err != nil {
				if err == io.EOF {
					return s.emit(wellFormedInt(s.b[s.s.Offset:s.n.Offset], base))
				}
				return s.emit(err)
			}
			rl := unicode.ToLower(r)
			if rl >= '0' && (base <= 10 && rl <= '0'+rune(base-1) || base > 10 && (rl <= '9' || rl >= 'a' && rl <= 'f')) {
				continue
			}
			if isWordSeparator(r) {
				s.undo()
				return s.emit(wellFormedInt(s.b[s.s.Offset:s.n.Offset], base))
			}
			return s.emit(fmt.Errorf("illegal symbol %s in base %d immediate value", strconv.QuoteRune(r), base))
		}
	}
}

func scanIdentifier(s *Scanner) scanState {
	for {
		r, err := s.next()
		if err != nil {
			if err == io.EOF {
				// catch EOF next time
				return s.emit(s.t)
			}
			return s.emit(err)
		}
		if unicode.In(r, unicode.Letter, unicode.Digit) || r == '_' {
			continue
		}
		s.undo()
		return s.emit(s.t)
	}
}

// skipToEOL silently eats everything until next EOL
// and keep that EOL for the next next()
//
func skipToEOL(s *Scanner) scanState {
	for {
		r, err := s.next()
		if err == io.EOF {
			// place EOF in the correct position
			s.s = s.n
			return s.emit(token.EOF)
		}
		if r == '\n' {
			s.undo()
			// reset start for '\n'
			s.s = s.n
			return scanAny
		}
	}
}
