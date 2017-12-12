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

// Package state provides state functions for lexing quoted strings,
// quoted characters and numbers (integers in any base as well as floats) and
// graceful handling of EOF.
//
// According to the convention on Lexer.StateFn, all state functions expect that
// the first character that is part of the lexed entity has already been read by
// Lexer.Next and will be retrieved by the state function via Lexer.Current.
//
// All functions (with the exception of EOF) are in fact constructors that
// take a at least a token type as argument and return closures. Note that
// because some of these constructors pre-allocate buffers, using the returned
// state functions concurently is not safe. See the examples for correct usage.
//
package state

//go:generate bash -c "godoc2md -ex -template ../../template/README.md.tpl github.com/db47h/parsekit/lexer/state >README.md"

import (
	"math/big"
	"unicode"
	"unicode/utf8"

	"github.com/db47h/parsekit/lexer"
	"github.com/db47h/parsekit/token"
)

// Number returns a StateFn that lexes an integer or float literal then emits it
// with the given type and either *big.Int or *big.Float value. This function
// expects that the first digit or leading decimal separator has already been
// read.
//
// The tokInt and tokFloat arguments specify the token type to emit for integer
// and floating point literals respectively. decimalSep is the character used
// as a decimal separator and the octal parameter indicates if integer literals
// starting with a leading '0' should be treated as octal numbers.
//
// The StateFn returned by this function is not reentrant. This is because Number
// pre-allocates a buffer for use by the StateFn (that will be reset on every
// call to the StateFn). As a rule of thumb, do not reuse the return value from
// Number.
//
func Number(tokInt, tokFloat token.Type, decimalSep rune, octal bool) lexer.StateFn {
	b := make([]byte, 0, 64)
	return func(l *lexer.Lexer) lexer.StateFn {
		b = b[:0]
		r := l.Current()
		pos := l.Pos()
		base := 10
		switch r {
		case decimalSep:
			return numFP(l, tokFloat, b)
		case '0':
			if octal {
				base = 8
			}
			fallthrough
		case '1', '2', '3', '4', '5', '6', '7', '8', '9':
			b = append(b, byte(r))
			for r = l.Next(); isDigit(r); r = l.Next() {
				b = append(b, byte(r))
			}
			if r == decimalSep || r == 'e' {
				return numFP(l, tokFloat, b)
			}
			l.Backup()
		default:
			panic("Not a number")
		}

		// integer
		if octal && b[0] == '0' {
			// check digits
			for i, r := range b[1:] {
				if r >= '0'+byte(base) {
					l.Errorf(pos+token.Pos(i)+1, "invalid character %#U in base %d immediate value", r, base)
					return nil
				}
			}
		}
		z, _ := new(big.Int).SetString(string(b), base)
		if z == nil {
			panic("int conversion failed")
		}
		l.Emit(tokInt, z)
		return nil
	}
}

// numFP lexes the fractional part of a number. The decimal separator
// or exponent 'e' rune has already been consumed (and no other rune than these).
//
func numFP(l *lexer.Lexer, t token.Type, b []byte) lexer.StateFn {
	r := l.Current()
	if r != 'e' {
		// decimal separator
		b = append(b, '.')
		for r = l.Next(); isDigit(r); r = l.Next() {
			b = append(b, byte(r))
		}
	}
	if r == 'e' {
		b = append(b, byte(r))
		r = l.Next()
		switch r {
		case '+', '-':
			b = append(b, byte(r))
			r = l.Next()
		}
		if !isDigit(r) {
			// Implementations that wish to implement things like implicit
			// multiplication, like 24epsilon, cannot use this function as-is.
			l.Errorf(l.Pos(), "malformed floating-point constant exponent")
			l.Backup()
			return nil
		}
		for {
			b = append(b, byte(r))
			r = l.Next()
			if !isDigit(r) {
				break
			}
		}
	}
	l.Backup()

	z, _, err := big.ParseFloat(string(b), 10, 512, big.ToNearestEven)
	if err != nil {
		panic(err)
	}
	l.Emit(t, z)
	return nil
}

// Int returns a state function that lexes the digits of an int in the given
// base amd emits it as a big.Int. This function expects that the first digit
// has been read.
//
// Supported bases are 2 to 36.
//
// Number lexing stops at the first non-digit character.
//
// For bases < 10 any ASCII digit greater than base will cause an error. Lexing
// will resume at the first non-digit character.
//
// The StateFn returned by this function is not reentrant. This is because Int
// pre-allocates a buffer for use by the StateFn (that will be reset on every
// call to the StateFn). As a rule of thumb, do not reuse the return value from
// Int.
//
// When entering the StateFn, if the last character read by Lexer.Next() is
// not a valid digit for the given base (i.e. empty number), this will cause an
// error and lexing will resume at that character. This may cause an infinite
// loop if not used properly.
//
func Int(t token.Type, base int) lexer.StateFn {
	if base < 2 || base > 36 {
		panic("unsupported number base")
	}
	b := make([]byte, 0, 64)
	return func(l *lexer.Lexer) lexer.StateFn {
		b = b[:0]
		r := l.Current()
		for {
			rl := unicode.ToLower(r)
			if rl >= 'a' {
				rl -= 'a' - '0' - 10
			}
			rl -= '0'
			if rl >= 0 && rl < int32(base) {
				b = append(b, byte(r))
				r = l.Next()
				continue
			}
			// for bases 2 and 8, consider that an invalid digit is an error instead
			// of an end of token: error then skip remaining digits.
			if rl >= int32(base) && rl <= 9 {
				l.Errorf(l.Pos(), "invalid character %#U in base %d immediate value", r, base)
				l.AcceptWhile(isDigit) // eat remaining digits
				return nil
			}
			if len(b) == 0 {
				// no digits!
				l.Errorf(l.Pos(), "malformed base %d immediate value", base)
				l.Backup()
				return nil
			}
			l.Backup()

			z := new(big.Int)
			z, _ = z.SetString(string(b), base)
			if z == nil {
				panic("int conversion failed")
			}
			l.Emit(t, z)
			return nil
		}
	}
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// EOF places the lexer in End-Of-File state.
// Once in this state, the lexer will only emit EOF.
//
func EOF(l *lexer.Lexer) lexer.StateFn {
	l.Emit(token.EOF, nil)
	return EOF
}

const (
	errEnd     = -2
	errRawByte = -1
	errNone    = iota
	errEOL
	errInvalidEscape
	errInvalidRune
	errInvalidHex
	errInvalidOctal
	errSize
	errEmpty
)

var msg = [...]string{
	errNone:          "",
	errEOL:           "unterminated %s",
	errInvalidEscape: "unknown escape sequence",
	errInvalidRune:   "escape sequence is invalid Unicode code point",
	errInvalidHex:    "non-hex character in escape sequence: %#U",
	errInvalidOctal:  "non-octal character in escape sequence: %#U",
	errSize:          "invalid character literal (more than 1 character)",
	errEmpty:         "empty character literal or unescaped %c in character literal",
}

// QuotedString returns a StateFn that lexes a Go string literal. It supports
// the same escape sequences as double-quoted Go string literals. Go raw string
// literals are not supported.
//
// When entering the StateFn, the starting delimiter has already been read and
// will be reused as end-delimiter.
//
func QuotedString(t token.Type) lexer.StateFn {
	s := make([]byte, 0, 64)
	var rb [utf8.UTFMax]byte
	return func(l *lexer.Lexer) lexer.StateFn {
		s = s[:0]
		quote := l.Current()
		pos := l.Pos()
		for {
			r, err := readChar(l, quote)
			switch err {
			case errNone:
				if r <= utf8.RuneSelf {
					s = append(s, byte(r))
				} else {
					s = append(s, rb[:utf8.EncodeRune(rb[:], r)]...)
				}
			case errRawByte:
				s = append(s, byte(r))
			case errEnd:
				l.Emit(t, string(s))
				return nil
			case errEOL:
				l.Backup()
				l.Errorf(pos, msg[errEOL], "string")
				return nil // keep going
			case errInvalidEscape, errInvalidRune:
				l.Errorf(l.Pos(), msg[err])
				return terminateString(l, quote)
			case errInvalidHex, errInvalidOctal:
				l.Errorf(l.Pos(), msg[err], l.Current())
				return terminateString(l, quote)
			}
		}
	}
}

// QuotedChar returns a StateFn that lexes a Go character literal.
//
// When entering the StateFn, the starting delimiter has already been read and
// will be reused as end-delimiter.
//
func QuotedChar(t token.Type) lexer.StateFn {
	return func(l *lexer.Lexer) lexer.StateFn {
		quote := l.Current()
		pos := l.Pos()
		r, err := readChar(l, quote)
		switch err {
		case errNone, errRawByte:
			n := l.Next()
			if n == quote {
				l.Emit(t, r)
				return nil
			}
			pos = l.Pos()
			l.Backup() // undo a potential EOF/EOL
			l.Errorf(pos, msg[errSize])
			return terminateString(l, quote)
		case errEnd:
			l.Errorf(l.Pos(), msg[errEmpty], quote)
			return nil
		case errEOL:
			l.Backup()
			l.Errorf(pos, msg[errEOL], "character literal")
			return nil // keep going
		case errInvalidEscape, errInvalidRune:
			l.Errorf(l.Pos(), msg[err])
			return terminateString(l, quote)
		case errInvalidHex, errInvalidOctal:
			l.Errorf(l.Pos(), msg[err], l.Current())
			return terminateString(l, quote)
		default:
			panic("unexpected return value from readChar")
		}
	}
}

// just eat up string and look for end quote not preceded by '\'
// TODO: if the rune that caused the error is a \, then our \ handling is off.
func terminateString(l *lexer.Lexer, quote rune) lexer.StateFn {
	return func(l *lexer.Lexer) lexer.StateFn {
		for {
			r := l.Next()
			switch r {
			case quote:
				return nil
			case '\\':
				r = l.Next()
				if r != '\n' && r != lexer.EOF {
					continue
				}
				fallthrough
			case '\n', lexer.EOF:
				// unterminated string. Just ignore the error since
				// this function is already called on error.
				l.Backup()
				return nil
			}
		}
	}
}

func readChar(l *lexer.Lexer, quote rune) (r rune, err int) {
	r = l.Next()
	switch r {
	case quote:
		return r, errEnd
	case '\\':
		r = l.Next()
		switch r {
		case 'a':
			return '\a', errNone
		case 'b':
			return '\b', errNone
		case 'f':
			return '\f', errNone
		case 'n':
			return '\n', errNone
		case 'r':
			return '\r', errNone
		case 't':
			return '\t', errNone
		case 'v':
			return '\v', errNone
		case '\\':
			return '\\', errNone
		case quote:
			return r, errNone
		case 'U':
			r, err := readDigits(l, 8, 16)
			if err == errNone && !utf8.ValidRune(r) {
				return utf8.RuneError, errInvalidRune
			}
			return r, err
		case 'u':
			r, err := readDigits(l, 4, 16)
			if err == errNone && !utf8.ValidRune(r) {
				return utf8.RuneError, errInvalidRune
			}
			return r, err
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'x':
			if r == 'x' {
				r, err = readDigits(l, 2, 16)
			} else {
				l.Backup()
				r, err = readDigits(l, 3, 8)
			}
			if err == errNone {
				err = errRawByte
			}
			return r, err
		case '\n', lexer.EOF:
			return r, errEOL
		default:
			return r, errInvalidEscape
		}
	case '\n', lexer.EOF:
		return r, errEOL
	}
	return r, errNone
}

func readDigits(l *lexer.Lexer, n, b int32) (v rune, err int) {
	for i := int32(0); i < n; i++ {
		r := l.Next()
		if r == '\n' || r == lexer.EOF {
			return v, errEOL
		}
		rl := unicode.ToLower(r)
		if rl >= 'a' {
			rl -= 'a' - '0' - 10
		}
		rl -= '0'
		if rl < 0 || rl >= b {
			if b == 8 {
				return v, errInvalidOctal
			}
			return v, errInvalidHex
		}
		v = v*b + rl
	}
	return v, errNone
}
