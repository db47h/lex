// Package lexer implements a lexer for assembler source text.
//
// This is a concurrent lexer, similar to https://golang.org/src/text/template/parse/lex.go.
//
// Despite its concurrent architecture, it essentially behaves as any other
// lexer from an API standpoint.
//
package lexer

import (
	"fmt"
	"io"
	"math/big"
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
	r            *reader
	f            *token.File
	isSeparator  func(token.Token, rune) bool
	isIdentifier func(rune, bool) bool
	err          func(*Lexer, error)
	t            []rune    // token string buffer
	n            token.Pos // position of next rune read
	c            chan *Item
	done         chan struct{}
}

type stateFn func(l *Lexer) stateFn

// New creates a new lexer associated with the given source file.
//
func New(f *token.File, opts ...Option) *Lexer {
	o := options{
		isSeparator:  defIsSeparator,
		isIdentifier: defIsIdentifier,
		errorHandler: defErrorHandler,
	}
	for _, f := range opts {
		f(&o)
	}
	l := &Lexer{
		r:            &reader{r: f},
		f:            f,
		c:            make(chan *Item),
		done:         make(chan struct{}),
		isSeparator:  o.isSeparator,
		isIdentifier: o.isIdentifier,
		err:          o.errorHandler,
	}
	go l.run()
	return l
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
			Pos:   l.n - 1,
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
		Pos:   l.n - token.Pos(len(l.t)),
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
			// Reuse the l.t slice.
			// We could end up with a large-ish slice hanging around (i.e. as
			// big as the largest lexed token), but this limits garbage collection.
			l.t = l.t[:0]
			if i.Token != token.EOF {
				return true
			}
			return false
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
		Pos:   l.n - 1, // that's where the error actually occurred
		Value: fmt.Sprintf(format, args...),
	}
	if l.emitItem(i) {
		return skipToEOL
	}
	return nil
}

func (l *Lexer) next() rune {
	r, s, err := l.r.ReadRune()
	switch {
	case s == 0 || err == io.EOF:
		r = eof
	case err != nil:
		r = eof
		defer l.err(l, err)
	}
	l.n++
	if r == '\n' {
		l.f.AddLine(l.n)
	}
	l.t = append(l.t, r)
	return r
}

// undo undoes the last call to next().
//
func (l *Lexer) undo() {
	ln := len(l.t) - 1
	if ln < 0 {
		panic("invalid use of undo")
	}
	l.t = l.t[:ln]
	l.n--
	if err := l.r.UnreadRune(); err != nil {
		panic(err)
	}
}

func (l *Lexer) tokenString() string {
	return string(l.t)
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
	case ':':
		return l.emit(token.Colon, nil)
	case '\\':
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
	case l.isIdentifier(r, true):
		return lexIdentifier
	}
	return l.errorf("invalid character %#U", r)
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
	default:
		l.undo()
		return l.emit(token.Immediate, &big.Int{})
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
			if rl >= base && rl <= 9 {
				return l.errorf("invalid character %#U in base %d immediate value", r, base)
			}
			l.undo()
			return emitInt(l, base, v)
		}
	}
}

// emitInt is the final stage of int lexing for ints with len > 1. It checks if the
// immediate value is well-formed. (i.e the minimum amount of digits)
// then emits the appropriate value(s).
//
func emitInt(l *Lexer, base int32, value *big.Int) stateFn {
	// len is at least 2 for bases 2 and 16. i.e. we've read at least
	// "0b" or "0x").
	sz := len(l.t)
	if (base == 2 || base == 16) && sz < 3 {
		// undo the trailing 'x' or 'b'
		l.undo()
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
		if l.isSeparator(token.Identifier, r) {
			l.undo()
			return l.emit(token.Identifier, l.tokenString())
		}
		if !l.isIdentifier(r, false) {
			return l.errorf("invalid identifier character %#U", r)
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
			l.t = l.t[:0]
			return lexAny
		}
	}
}
