// Package scanner implements a scanner for assembler source text.
//
// This is a concurrent scanner loosely based on Ivy (https://github.com/robpike/ivy).
//
// While it essentially behaves as any other scanner as seen from the API,
// there are a few things to be aware of:
//
// Init() and Close() should be called only once. There are no checks for this.
// As a result, a scanner cannot be re-used to process a different stream of data.
//
// If the last call to scan returns a token.EOF, no further calls to Scan should be made.
// In the current version, the caller would block forever.
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

func (p *Pos) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// Token represents a scanned token.
//
type Token struct {
	token.Token
	Pos        // Start position
	Raw []byte // Raw bytes for the token
}

// String returns a string representation of the token. This should be used only for debugging purposes as
// the output format is not guaranteed to be stable.
//
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
	s    Pos // token start pos
	n    Pos // next rune to read by next()
	u    Pos // saved position to undo last call to next()
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

// emit emits a single token. Returns the next state depending
// on the success of the operation.
//
func (s *Scanner) emit(t token.Token) scanState {
	if s.emitToken(&Token{
		Token: t,
		Pos:   s.s,
		Raw:   s.b[s.s.Offset:s.n.Offset],
	}) {
		return scanAny
	}
	return nil
}

// emitToken emits the given token. Returns false if the scanner has been aborted
// or the last token is EOF. If false is returned, the caller (usually a scanState)
// should return nil to abort the scanner's loop.
//
func (s *Scanner) emitToken(t *Token) bool {
	for {
		select {
		case s.c <- *t:
			s.s = s.n
			return t.Token != token.EOF
		case <-s.done:
			return false
		}
	}
}

// emitError emits an error assuming the general case that the
// error occurred at s.u. See emitErrorAtPos.
//
func (s *Scanner) emitError(err error) scanState {
	return s.emitErrorAtPos(err, s.u)
}

// emitErrorOrToken emits an error or the given token if the error is nil or EOF.
//
func (s *Scanner) emitErrorOrToken(err error, t token.Token) scanState {
	if err == nil || err == io.EOF {
		return s.emit(t)
	}
	return s.emitError(err)
}

// emitErrorAtPos emits an error Token at the given pos. The Raw value of the
// Token is set to the error's string representation. Places the scanner in
// skipToEOL state (i.e. all input until the next EOL is ignored).
//
func (s *Scanner) emitErrorAtPos(err error, pos Pos) scanState {
	tok := &Token{
		Token: token.Error,
		Pos:   pos, // that's where the error actually occurred
		Raw:   []byte(err.Error()),
	}
	if s.emitToken(tok) {
		return skipToEOL
	}
	return nil
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
		return s.emitErrorOrToken(err, token.EOF)
	}
	switch r {
	case '\n':
		return s.emit(token.EOL)
	case '(':
		return s.emit(token.LeftParen)
	case ')':
		return s.emit(token.RightParen)
	case '[':
		return s.emit(token.LeftBracket)
	case ']':
		return s.emit(token.RightBracket)
	case ':': // TODO: allowed inside symbols on some platforms
		return s.emit(token.Colon)
	case '\\': // prefix for macro arguments
		return s.emit(token.Backslash)
	case ',':
		return s.emit(token.Comma)
	case '+':
		return s.emit(token.OpPlus)
	case '-':
		return s.emit(token.OpMinus)
	case '*':
		return s.emit(token.OpFactor)
	case '/':
		return s.emit(token.OpDiv)
	case '%':
		return s.emit(token.OpMod)
	case '&':
		return s.emit(token.OpAnd)
	case '|':
		return s.emit(token.OpOr)
	case '^':
		return s.emit(token.OpXor)
	case ';':
		return scanComment
	case '.': // not necessarily a word separator
		return s.emit(token.Dot)
	}

	switch {
	case unicode.IsSpace(r):
		return scanSpace
	case r >= '0' && r <= '9':
		if r != '0' {
			return scanInt(10)
		}
		return scanIntBase
	case unicode.IsLetter(r) || r == '_':
		return scanIdentifier
	}
	return s.emitError(fmt.Errorf("illegal symbol %s", strconv.QuoteRune(r)))
}

func isWordSeparator(r rune) bool {
	// This needs updating if we add symbols to the syntax
	// these are valid characters immediately following (and marking the end of) a number
	switch r {
	case '(', ')', '[', ']', '\\', ',', ';', '+', '-', '*', '/', '%', '&', '|', '^':
		return true
	case ':': // TODO: allowed inside symbols on some platforms
		return true
	default:
		return unicode.IsSpace(r)
	}
}

func scanSpace(s *Scanner) scanState {
	for {
		r, err := s.next()
		if err != nil {
			return s.emitErrorOrToken(err, token.Space)
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
		if err != nil {
			return s.emitErrorOrToken(err, token.Comment)
		}
		if r == '\n' {
			s.undo()
			return s.emit(token.Comment)
		}
	}
}

// scanIntBase reads the character foillowing a leading 0
// to determine the number base or directly emit a 0 literal.
// Supported number bases are 8, 10 and 16.
//
// Special case: a leading 0 followed by 'b' or 'f' is a local label.
//
func scanIntBase(s *Scanner) scanState {
	r, err := s.next()
	if err != nil {
		return s.emitErrorOrToken(err, token.Immediate)
	}
	switch {
	case r >= '0' && r <= '9':
		return scanInt(8)
	case r == 'x' || r == 'X':
		return scanInt(16)
	case r == 'b' || r == 'f':
		return scanLocalLabel
	case isWordSeparator(r):
		s.undo()
		return s.emit(token.Immediate)
	default:
		return s.emitError(fmt.Errorf("illegal symbol %s in immediate value", strconv.QuoteRune(r)))
	}
}

// emitInt is the final stage of int scanning for ints with len > 1. It checks if the
// immediate value is well-formed. (i.e the minimum amount of digits)
// then emits the appropriate value.
//
func emitInt(s *Scanner, base int) scanState {
	// len is at least one. Base 16 needs at least 3 bytes.
	if base != 16 || s.n.Offset-s.s.Offset > 2 {
		return s.emit(token.Immediate)
	}
	return s.emitErrorAtPos(fmt.Errorf("malformed immediate value %q", s.b[s.s.Offset:s.n.Offset]), s.s)
}

// scanInt scans the 2nd to n digit of an int in the given base.
// Supported bases are 8, 10 and 16.
//
func scanInt(base int) scanState {
	max := '0'
	if base < 10 {
		max = '0' + rune(base-1)
	}
	return func(s *Scanner) scanState {
		for {
			r, err := s.next()
			if err != nil {
				if err == io.EOF {
					return emitInt(s, base)
				}
				return s.emitError(err)
			}
			rl := unicode.ToLower(r)
			if rl >= '0' && rl <= max || base == 16 && rl >= 'a' && rl <= 'f' {
				continue
			}
			if r == 'b' || r == 'f' {
				return scanLocalLabel
			}
			if isWordSeparator(r) {
				s.undo()
				return emitInt(s, base)
			}
			return s.emitError(fmt.Errorf("illegal symbol %s in base %d immediate value", strconv.QuoteRune(r), base))
		}
	}
}

// scanIdentifier scans an identifier starting with _ or a unicode letter
// followed by any combination of printable unicode characters that are not
// word separators (operators, brackets, backslash, ',', ';', ':'). This
// includes letters, marks, numbers, punctuation, symbols, from categories L, M,
// N, P, S.
//
func scanIdentifier(s *Scanner) scanState {
	for {
		r, err := s.next()
		if err != nil {
			return s.emitErrorOrToken(err, token.Identifier)
		}
		if isWordSeparator(r) {
			s.undo()
			return s.emit(token.Identifier)
		}
		if !unicode.IsPrint(r) {
			return s.emitError(fmt.Errorf("illegal symbol in identifier %s", strconv.QuoteRune(r)))
		}
	}
}

// skipToEOL silently eats everything until next EOL
// and keep that EOL for the next next()
//
func skipToEOL(s *Scanner) scanState {
	for {
		r, err := s.next()
		// ignore all errors but EOF
		if err == io.EOF {
			// place EOF in the correct position
			s.s = s.n
			return s.emit(token.EOF)
		}
		if r == '\n' { // err == nil implied
			s.undo()
			// reset start for '\n'
			s.s = s.n
			return scanAny
		}
	}
}

// scanLocalLabel scans the character following the final 'b' or 'f'
// and makes sure it's a word separator
//
func scanLocalLabel(s *Scanner) scanState {
	r, err := s.next()
	if err != nil {
		return s.emitErrorOrToken(err, token.LocalLabel)
	}
	if isWordSeparator(r) {
		s.undo()
		return s.emit(token.LocalLabel)
	}
	return s.emitError(fmt.Errorf("malformed local label or immediate value: illegal symbol %s", strconv.QuoteRune(r)))
}
