// Package lexer implements a lexer for assembler source text.
//
// This is a concurrent similar to https://golang.org/src/text/template/parse/lex.go.
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

	"github.com/db47h/asm/token"
)

const eof = -1

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
//
type Lexer struct {
	f    *token.File
	err  func(error)
	t    []rune    // token string buffer
	s    token.Pos // current token start pos
	n    token.Pos // position of next rune to read
	r    rune      // last rune read
	u    bool      // undo
	c    chan *Item
	done chan struct{}
}

type stateFn func(l *Lexer) stateFn

// New creates a new lexer associated with the given source file.
// The provided error handler is called whenever an io error (other than
// io.EOF) occurs during lexing. If errorHandler is nil, the lexer will panic
// on io errors.
//
func New(f *token.File, errorHandler func(err error)) *Lexer {
	l := &Lexer{
		f:    f,
		c:    make(chan *Item),
		done: make(chan struct{}),
		err:  errorHandler,
	}
	if errorHandler == nil {
		l.err = l.defaultErrorHandler
	}
	go l.run()
	return l
}

func (l *Lexer) defaultErrorHandler(err error) {
	line, col := l.f.Position(l.nextPos() - 1)
	panic(fmt.Errorf("%s:%d:%d io error \"%s\"", l.f.Name(), line, col, err))
}

func (l *Lexer) run() {
	for state := lexAny; state != nil; state = state(l) {
	}
	close(l.c)
}

// Lex reads source text and returns the next item until EOF. Once this
// function has returned an EOF token, any further calls to it will panic.
//
func (l *Lexer) Lex() *Item {
	i := <-l.c
	if i == nil {
		// l.c has been closed
		return &Item{
			Token: token.EOF,
			Pos:   l.s,
			Value: nil,
		}
	}
	return i
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

// emit emits a single token. Returns the next state depending
// on the success of the operation.
//
func (l *Lexer) emit(t token.Token, value interface{}) stateFn {
	if l.emitItem(&Item{
		Token: t,
		Pos:   l.s,
		Value: value,
	}) {
		return lexAny
	}
	return nil
}

// emitItem emits the given item. Returns false if the lexer has been aborted
// or the last token is EOF. If false is returned, the caller (usually a stateFn)
// should return nil to abort the lexer's loop.
//
func (l *Lexer) emitItem(i *Item) bool {
	for {
		select {
		case l.c <- i:
			if l.u {
				l.t = l.t[len(l.t)-1:]
			} else {
				l.t = l.t[:0]
			}
			if i.Token == token.EOF {
				return false
			}
			l.s = l.nextPos()
			return true
		case <-l.done:
			return false
		}
	}
}

// errorf emits an error token. The Item value is set to a string
// representation of the error. Places the lexer in skipToEOL state (i.e. all
// input until the next EOL is ignored).
//
func (l *Lexer) errorf(format string, args ...interface{}) stateFn {
	i := &Item{
		Token: token.Error,
		Pos:   l.nextPos() - 1, // that's where the error actually occurred
		Value: fmt.Sprintf(format, args...),
	}
	if l.emitItem(i) {
		return skipToEOL
	}
	return nil
}

func (l *Lexer) next() rune {
	if l.u {
		l.u = false
		return l.r
	}
	r, s, err := l.f.ReadRune()
	switch {
	case s == 0 || err == io.EOF:
		r = eof
	case err != nil:
		r = eof
		defer l.err(err)
	}
	if l.r == '\n' {
		l.f.AddLine(l.n)
	}
	l.n++
	l.r = r
	l.t = append(l.t, r)
	return r
}

func (l *Lexer) undo() {
	if l.u {
		panic("impossible to undo")
	}
	l.u = true
}

func (l *Lexer) nextPos() token.Pos {
	if l.u {
		return l.n - 1
	}
	return l.n
}

func (l *Lexer) tokenString() string {
	return string(l.t[:l.nextPos()-l.s])
}

func lexAny(l *Lexer) stateFn {
	r := l.next()
	switch r {
	case eof:
		return l.emit(token.EOF, nil)
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
		return lexComment
	case '.': // not necessarily a word separator
		return l.emit(token.Dot, nil)
	}

	switch {
	case unicode.IsSpace(r):
		return lexSpace
	case r >= '0' && r <= '9':
		if r != '0' {
			l.undo() // let scanInt read the whole number
			return lexIntDigits(10)
		}
		return lexIntBase
	case unicode.IsLetter(r) || r == '_':
		return lexIdentifier
	}
	return l.errorf("illegal symbol %s", strconv.QuoteRune(r))
}

func isWordSeparator(r rune) bool {
	// This needs updating if we add symbols to the syntax
	// these are valid characters immediately following (and marking the end of)
	// any token.
	switch r {
	case '(', ')', '[', ']', '\\', ',', ';', '+', '-', '*', '/', '%', '&', '|', '^':
		return true
	case ':': // TODO: allowed inside symbols on some platforms
		return true
	case eof:
		return true
	default:
		return unicode.IsSpace(r)
	}
}

func lexSpace(l *Lexer) stateFn {
	for {
		r := l.next()
		if r != eof && unicode.IsSpace(r) && r != '\n' {
			continue
		}
		// revert last rune read
		l.undo()
		return l.emit(token.Space, nil)
	}
}

func lexComment(l *Lexer) stateFn {
	for {
		r := l.next()
		if r == eof || r == '\n' {
			l.undo()
			return l.emit(token.Comment, l.tokenString())
		}
	}
}

// lexIntBase reads the character foillowing a leading 0 in order to determine
// the number base or directly emit a 0 literal or local label.
//
// Supported number bases are 2, 8, 10 and 16.
//
// Special case: a leading 0 followed by 'b' or 'f' is a local label.
//
func lexIntBase(l *Lexer) stateFn {
	r := l.next()
	switch {
	case r >= '0' && r <= '9':
		// undo in order to let scanIntDigits read the whole number
		// (except the leading 0) or error appropriately if r is >= 8
		l.undo()
		return lexIntDigits(8)
	case r == 'x' || r == 'X':
		return lexIntDigits(16)
	case r == 'b': // possible LocalLabel caught in scanIntDigits
		return lexIntDigits(2)
	case r == 'f':
		return lexLocalLabel
	case isWordSeparator(r):
		l.undo()
		return l.emit(token.Immediate, &big.Int{})
	default:
		return l.errorf("illegal symbol %s in immediate value", strconv.QuoteRune(r))
	}
}

// lexIntDigits scans the 2nd to n digit of an int in the given base.
//
// Supported bases are 2, 8, 10 and 16.
//
func lexIntDigits(base int32) stateFn {
	return func(l *Lexer) stateFn {
		v := &big.Int{}
		var t big.Int
		for {
			r := l.next()
			if isWordSeparator(r) {
				l.undo()
				return emitInt(l, base, v)
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
				return lexLocalLabel
			}
			return l.errorf("illegal symbol %s in base %d immediate value", strconv.QuoteRune(r), base)
		}
	}
}

// emitInt is the final stage of int lexing for ints with len > 1. It checks if the
// immediate value is well-formed. (i.e the minimum amount of digits)
// then emits the appropriate value.
//
// There's a special case with "0b" that will be sent as LocalLabel "0b".
//
func emitInt(l *Lexer, base int32, value *big.Int) stateFn {
	// len is at least 2 for bases 2 and 16. i.e. we've read at least
	// "0b" or "0x").
	sz := l.nextPos() - l.s
	if base == 16 && sz < 3 {
		return l.errorf("malformed immediate value \"%s\"", l.tokenString())
	} else if base == 2 && sz == 2 {
		// "0b" exactly; the "0f" case has been filtered out in scanIntBase.
		return l.emit(token.LocalLabel, l.tokenString())
	}
	return l.emit(token.Immediate, value)
}

// lexIdentifier scans an identifier starting with _ or a unicode letter
// followed by any combination of printable unicode characters that are not
// word separators (operators, brackets, backslash, ',', ';', ':'). This
// includes letters, marks, numbers, punctuation, symbols, from categories L, M,
// N, P, S.
//
func lexIdentifier(l *Lexer) stateFn {
	for {
		r := l.next()
		if isWordSeparator(r) {
			l.undo()
			return l.emit(token.Identifier, l.tokenString())
		}
		if !unicode.IsPrint(r) {
			return l.errorf("illegal symbol in identifier %s", strconv.QuoteRune(r))
		}
	}
}

// skipToEOL silently eats everything until next EOL
// and keep that EOL for the next next()
//
func skipToEOL(l *Lexer) stateFn {
	for {
		r := l.next()
		if r == eof || r == '\n' {
			l.undo()
			// reset start for '\n' or EOF
			l.s = l.nextPos()
			return lexAny
		}
	}
}

// lexLocalLabel scans the character following the final 'b' or 'f'
// and makes sure it's a word separator.
//
func lexLocalLabel(l *Lexer) stateFn {
	r := l.next()
	if isWordSeparator(r) {
		l.undo()
		return l.emit(token.LocalLabel, l.tokenString())
	}
	return l.errorf("malformed local label or immediate value: illegal symbol %s", strconv.QuoteRune(r))
}
