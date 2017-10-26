// Package lexer implements a lexer for assembler source text.
//
// This is a concurrent lexer loosely based on Ivy (https://github.com/robpike/ivy).
//
// Despite its concurrent architecture, it essentially behaves as any other
// lexer from an API standpoint.
//
package lexer

import (
	"fmt"
	"io"
	"math/big"
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/db47h/asm/token"
)

// Pos represents a token's position.
//
type Pos struct {
	Offset int // starts at 0
	Line   int // starts at 1
	Column int // starts at 1
}

func (p *Pos) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// Lexeme represents a lexeme.
//
type Lexeme struct {
	token.Token
	Pos   // Start position
	Value interface{}
}

// String returns a string representation of the lexeme. This should be used only for debugging purposes as
// the output format is not guaranteed to be stable.
//
func (l *Lexeme) String() string {
	switch l.Token {
	case token.Error:
		return fmt.Sprintf("%d:%d: %s \"%s\"", l.Pos.Line, l.Pos.Column, l.Token, l.Value)
	default:
		switch v := l.Value.(type) {
		case string:
			return fmt.Sprintf("%d:%d: %s %q", l.Pos.Line, l.Pos.Column, l.Token, v)
		case interface {
			String() string
		}:
			return fmt.Sprintf("%d:%d: %s %q", l.Pos.Line, l.Pos.Column, l.Token, v.String())
		default:
			return fmt.Sprintf("%d:%d: %s", l.Pos.Line, l.Pos.Column, l.Token)
		}
	}
}

// A Lexer holds the internal state of the lexer while processing a given text.
//
type Lexer struct {
	b    []byte
	s    Pos // token start pos
	n    Pos // next rune to read by next()
	u    Pos // saved position to undo last call to next()
	c    chan *Lexeme
	done chan struct{}
}

type scanState func(l *Lexer) scanState

// New creates a new lexer associated with the given source byte slice.
//
func New(b []byte) *Lexer {
	l := &Lexer{
		b: b,
		s: Pos{
			Line:   1,
			Column: 1,
		},
		n: Pos{
			Line:   1,
			Column: 1,
		},
		u:    Pos{Line: 0, Column: 0},
		c:    make(chan *Lexeme),
		done: make(chan struct{}),
	}
	go func() {
		state := scanAny
		for state != nil {
			state = state(l)
		}
	}()
	return l
}

// Lex reads source text and returns the next lexeme until EOF. Once this
// function has returned an EOF token, any further calls to it will panic.
//
func (l *Lexer) Lex() *Lexeme {
	lx := <-l.c
	if lx == nil {
		// l.c has been closed
		return &Lexeme{
			Token: token.EOF,
			Pos:   l.s,
			Value: nil,
		}
	}
	return lx
}

// Close stops the lexer. This function should always be called once the lexer
// is no longer needed. After Close() has been called, Calling Lex() again wiil
// result in an undefined behavior.
//
func (l *Lexer) Close() {
	if l.done != nil {
		close(l.done)
	}
}

// emit emits a single token. Returns the next state depending
// on the success of the operation.
//
func (l *Lexer) emit(t token.Token, value interface{}) scanState {
	if l.emitLexeme(&Lexeme{
		Token: t,
		Pos:   l.s,
		Value: value,
	}) {
		return scanAny
	}
	return nil
}

// emitLexeme emits the given lexeme. Returns false if the lexer has been aborted
// or the last token is EOF. If false is returned, the caller (usually a scanState)
// should return nil to abort the lexer's loop.
//
func (l *Lexer) emitLexeme(lm *Lexeme) bool {
	for {
		select {
		case l.c <- lm:
			l.s = l.n
			if lm.Token != token.EOF {
				return true
			}
			close(l.c)
			return false
		case <-l.done:
			close(l.c) // undefined behavior if calling Lex after this
			return false
		}
	}
}

// emitError emits an error assuming the general case that the
// error occurred at s.u. See emitErrorAtPos.
//
func (l *Lexer) emitError(err error) scanState {
	return l.emitErrorAtPos(err, l.u)
}

// emitErrorOrToken emits an error or the given token if the error is nil or EOF.
//
func (l *Lexer) emitErrorOrToken(err error, t token.Token, value interface{}) scanState {
	if err == nil || err == io.EOF {
		return l.emit(t, value)
	}
	return l.emitError(err)
}

// emitErrorAtPos emits an error Token at the given pos. The value of the
// Lexeme is set to the error. Places the lexer in skipToEOL state (i.e. all
// input until the next EOL is ignored).
//
func (l *Lexer) emitErrorAtPos(err error, pos Pos) scanState {
	lm := &Lexeme{
		Token: token.Error,
		Pos:   pos, // that's where the error actually occurred
		Value: err,
	}
	if l.emitLexeme(lm) {
		return skipToEOL
	}
	return nil
}

func (l *Lexer) next() (rune, error) {
	l.u = l.n
	r, sz := utf8.DecodeRune(l.b[l.n.Offset:])
	if r == utf8.RuneError {
		if sz == 0 {
			return r, io.EOF
		}
		l.n.Offset += sz
		return r, fmt.Errorf("invalid rune \\x%02X", l.b[l.n.Offset-sz:l.n.Offset])
	}
	l.n.Offset += sz
	if r == '\n' {
		l.n.Line++
		l.n.Column = 1
	} else {
		l.n.Column++
	}
	return r, nil
}

func (l *Lexer) undo() {
	l.n = l.u
}

func (l *Lexer) tokenString() string {
	if l.s.Offset < l.n.Offset {
		return string(l.b[l.s.Offset:l.n.Offset])
	}
	return ""
}

func scanAny(l *Lexer) scanState {
	r, err := l.next()
	if err != nil {
		return l.emitErrorOrToken(err, token.EOF, nil)
	}
	switch r {
	case '\n':
		return l.emit(token.EOL, nil)
	case '(':
		return l.emit(token.LeftParen, nil)
	case ')':
		return l.emit(token.RightParen, nil)
	case '[':
		return l.emit(token.LeftBracket, nil)
	case ']':
		return l.emit(token.RightBracket, nil)
	case ':': // TODO: allowed inside symbols on some platforms
		return l.emit(token.Colon, nil)
	case '\\': // prefix for macro arguments
		return l.emit(token.Backslash, nil)
	case ',':
		return l.emit(token.Comma, nil)
	case '+':
		return l.emit(token.OpPlus, nil)
	case '-':
		return l.emit(token.OpMinus, nil)
	case '*':
		return l.emit(token.OpFactor, nil)
	case '/':
		return l.emit(token.OpDiv, nil)
	case '%':
		return l.emit(token.OpMod, nil)
	case '&':
		return l.emit(token.OpAnd, nil)
	case '|':
		return l.emit(token.OpOr, nil)
	case '^':
		return l.emit(token.OpXor, nil)
	case ';':
		return scanComment
	case '.': // not necessarily a word separator
		return l.emit(token.Dot, nil)
	}

	switch {
	case unicode.IsSpace(r):
		return scanSpace
	case r >= '0' && r <= '9':
		if r != '0' {
			l.undo() // let scanInt read the whole number
			return scanIntDigits(10)
		}
		return scanIntBase
	case unicode.IsLetter(r) || r == '_':
		return scanIdentifier
	}
	return l.emitError(fmt.Errorf("illegal symbol %s", strconv.QuoteRune(r)))
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

func scanSpace(l *Lexer) scanState {
	for {
		r, err := l.next()
		if err != nil {
			return l.emitErrorOrToken(err, token.Space, nil)
		}
		if unicode.IsSpace(r) && r != '\n' {
			continue
		}
		// revert last rune read
		l.undo()
		return l.emit(token.Space, nil)
	}
}

func scanComment(l *Lexer) scanState {
	for {
		r, err := l.next()
		if err != nil {
			return l.emitErrorOrToken(err, token.Comment, l.tokenString())
		}
		if r == '\n' {
			l.undo()
			return l.emit(token.Comment, l.tokenString())
		}
	}
}

// scanIntBase reads the character foillowing a leading 0 in order to determine
// the number base or directly emit a 0 literal or local label.
//
// Supported number bases are 2, 8, 10 and 16.
//
// Special case: a leading 0 followed by 'b' or 'f' is a local label.
//
func scanIntBase(l *Lexer) scanState {
	r, err := l.next()
	if err != nil {
		l.emitErrorOrToken(err, token.Immediate, &big.Int{})
	}
	switch {
	case r >= '0' && r <= '9':
		// undo in order to let scanIntDigits read the whole number
		// (except the leading 0) or error appropriately if r is >= 8
		l.undo()
		return scanIntDigits(8)
	case r == 'x' || r == 'X':
		return scanIntDigits(16)
	case r == 'b': // possible LocalLabel caught in scanIntDigits
		return scanIntDigits(2)
	case r == 'f':
		return scanLocalLabel
	case isWordSeparator(r):
		l.undo()
		return l.emit(token.Immediate, &big.Int{})
	default:
		return l.emitError(fmt.Errorf("illegal symbol %s in immediate value", strconv.QuoteRune(r)))
	}
}

// scanIntDigits scans the 2nd to n digit of an int in the given base.
//
// Supported bases are 2, 8, 10 and 16.
//
func scanIntDigits(base int32) scanState {
	return func(l *Lexer) scanState {
		v := &big.Int{}
		var t big.Int
		for {
			r, err := l.next()
			if err != nil {
				if err == io.EOF {
					return emitInt(l, base, v)
				}
				return l.emitError(err)
			}
			rl := unicode.ToLower(r)
			if rl >= 'a' {
				rl -= 'a' - '0' + 10
			}
			rl -= '0'
			if rl >= 0 && rl < base {
				t.SetInt64(int64(base))
				v = v.Mul(v, &t)
				t.SetInt64(int64(rl))
				v = v.Add(v, &t)
				continue
			}
			if base == 10 && r == 'b' || r == 'f' {
				return scanLocalLabel
			}
			if isWordSeparator(r) {
				l.undo()
				return emitInt(l, base, v)
			}
			return l.emitError(fmt.Errorf("illegal symbol %s in base %d immediate value", strconv.QuoteRune(r), base))
		}
	}
}

// emitInt is the final stage of int lexing for ints with len > 1. It checks if the
// immediate value is well-formed. (i.e the minimum amount of digits)
// then emits the appropriate value.
//
// There's a special case with "0b" that will be sent as LocalLabel "0b".
//
func emitInt(l *Lexer, base int32, value *big.Int) scanState {
	// len is at least one. Base 16 needs at least 3 bytes.
	if base == 16 && l.n.Offset-l.s.Offset < 3 {
		return l.emitErrorAtPos(fmt.Errorf("malformed immediate value %q", l.b[l.s.Offset:l.n.Offset]), l.s)
	} else if base == 2 && l.n.Offset-l.s.Offset == 2 && l.b[l.s.Offset+1] == 'b' {
		// the "0f" case has been filtered out in scanIntBase
		return l.emit(token.LocalLabel, l.tokenString())
	}
	return l.emit(token.Immediate, value)
}

// scanIdentifier scans an identifier starting with _ or a unicode letter
// followed by any combination of printable unicode characters that are not
// word separators (operators, brackets, backslash, ',', ';', ':'). This
// includes letters, marks, numbers, punctuation, symbols, from categories L, M,
// N, P, S.
//
func scanIdentifier(l *Lexer) scanState {
	for {
		r, err := l.next()
		if err != nil {
			return l.emitErrorOrToken(err, token.Identifier, l.tokenString())
		}
		if isWordSeparator(r) {
			l.undo()
			return l.emit(token.Identifier, l.tokenString())
		}
		if !unicode.IsPrint(r) {
			return l.emitError(fmt.Errorf("illegal symbol in identifier %s", strconv.QuoteRune(r)))
		}
	}
}

// skipToEOL silently eats everything until next EOL
// and keep that EOL for the next next()
//
func skipToEOL(l *Lexer) scanState {
	for {
		r, err := l.next()
		// ignore all errors but EOF
		if err == io.EOF {
			// place EOF in the correct position
			l.s = l.n
			return l.emit(token.EOF, nil)
		}
		if r == '\n' { // err == nil implied
			l.undo()
			// reset start for '\n'
			l.s = l.n
			return scanAny
		}
	}
}

// scanLocalLabel scans the character following the final 'b' or 'f'
// and makes sure it's a word separator
//
func scanLocalLabel(l *Lexer) scanState {
	r, err := l.next()
	if err != nil {
		return l.emitErrorOrToken(err, token.LocalLabel, l.tokenString())
	}
	if isWordSeparator(r) {
		l.undo()
		return l.emit(token.LocalLabel, l.tokenString())
	}
	return l.emitError(fmt.Errorf("malformed local label or immediate value: illegal symbol %s", strconv.QuoteRune(r)))
}
