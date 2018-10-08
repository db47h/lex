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

// Package state provides state functions for lexing numbers, quoted strings and
// quoted characters.
//
// State functions in this package expect that the first character that is
// part of the lexed entity has already been read by lex.Next. For example:
//
//	r := s.Next()
//	switch r {
//	case '"':
//		// do not call s.Backup() here
//		return state.QuotedString(tokString)
//	}
//
// All functions (with the exception of EOF) are in fact constructors that
// take a at least a token type as argument and return closures. Note that
// because some of these constructors pre-allocate buffers, using the returned
// state functions concurrently is not safe. See the examples for correct usage.
//
package state

//go:generate bash -c "godoc2md -ex -template ../template/README.md.tpl github.com/db47h/lex/state >README.md"

import (
	"unicode/utf8"

	"github.com/db47h/lex"
)

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
func QuotedString(t lex.Token) lex.StateFn {
	s := make([]byte, 0, 64)
	var rb [utf8.UTFMax]byte
	return func(l *lex.State) lex.StateFn {
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
				l.Emit(pos, t, string(s))
				return nil
			case errEOL:
				l.Backup()
				l.Errorf(pos, msg[errEOL], "string")
				return nil // keep going
			case errInvalidEscape, errInvalidRune:
				l.Errorf(l.Pos(), msg[err])
				return terminateString(quote)
			case errInvalidHex, errInvalidOctal:
				l.Errorf(l.Pos(), msg[err], l.Current())
				return terminateString(quote)
			}
		}
	}
}

// QuotedChar returns a StateFn that lexes a Go character literal.
//
// When entering the StateFn, the starting delimiter has already been read and
// will be reused as end-delimiter.
//
func QuotedChar(t lex.Token) lex.StateFn {
	return func(l *lex.State) lex.StateFn {
		quote := l.Current()
		pos := l.Pos()
		r, err := readChar(l, quote)
		switch err {
		case errNone, errRawByte:
			n := l.Next()
			if n == quote {
				l.Emit(pos, t, r)
				return nil
			}
			pos = l.Pos()
			l.Backup() // undo a potential EOF/EOL
			l.Errorf(pos, msg[errSize])
			return terminateString(quote)
		case errEnd:
			l.Errorf(l.Pos(), msg[errEmpty], quote)
			return nil
		case errEOL:
			l.Backup()
			l.Errorf(pos, msg[errEOL], "character literal")
			return nil // keep going
		case errInvalidEscape, errInvalidRune:
			l.Errorf(l.Pos(), msg[err])
			return terminateString(quote)
		case errInvalidHex, errInvalidOctal:
			l.Errorf(l.Pos(), msg[err], l.Current())
			return terminateString(quote)
		default:
			panic("unexpected return value from readChar")
		}
	}
}

// just eat up string and look for end quote not preceded by '\'
// TODO: if the rune that caused the error is a \, then our \ handling is off.
func terminateString(quote rune) lex.StateFn {
	return func(l *lex.State) lex.StateFn {
		for {
			r := l.Next()
			switch r {
			case quote:
				return nil
			case '\\':
				r = l.Next()
				if r != '\n' && r != lex.EOF {
					continue
				}
				fallthrough
			case '\n', lex.EOF:
				// unterminated string. Just ignore the error since
				// this function is already called on error.
				l.Backup()
				return nil
			}
		}
	}
}

func readChar(l *lex.State, quote rune) (r rune, err int) {
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
		case '\n', lex.EOF:
			return r, errEOL
		default:
			return r, errInvalidEscape
		}
	case '\n', lex.EOF:
		return r, errEOL
	}
	return r, errNone
}

func readDigits(l *lex.State, n, b int32) (v rune, err int) {
	for i := int32(0); i < n; i++ {
		var rl rune
		r := l.Next()
		if r == '\n' || r == lex.EOF {
			return v, errEOL
		}
		switch {
		case r >= 'a':
			rl = r - 'a' + 10
		case r >= 'A':
			rl = r - 'A' + 10
		default:
			rl = r - '0'
		}
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
