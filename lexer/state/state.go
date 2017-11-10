package state

import (
	"math/big"
	"unicode"
	"unicode/utf8"

	"github.com/db47h/asm/lexer"
	"github.com/db47h/asm/token"
)

// EmitNil just emits the current token with a nil value.
//
func EmitNil(l *lexer.Lexer) lexer.StateFn {
	l.Emit(l.T, nil)
	return nil
}

// EmitString just emits the current token with a string value.
//
func EmitString(l *lexer.Lexer) lexer.StateFn {
	l.Emit(l.T, l.TokenString())
	return nil
}

// Int lexes an integer literal then emits it as a *big.Int.
// This function expects that the first digit has already been read.
//
// Integers are expected to start with a leading 0 for base 8, "0x" for base 16
// and "0b" for base 2.
//
func Int(l *lexer.Lexer) lexer.StateFn {
	if l.Last() == '0' {
		return lexIntBase
	}
	return IntDigits(10)
}

// lexIntBase reads the character following a leading 0 in order to determine
// the number base or directly emit a 0 literal.
//
// Supported number bases are 2, 8, 10 and 16.
//
func lexIntBase(l *lexer.Lexer) lexer.StateFn {
	r := l.Next()
	switch r {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// undo in order to let scanIntDigits read the whole number
		// (except the leading 0) or error appropriately if r is >= 8
		return IntDigits(8)
	case 'x', 'X':
		l.Next() // consume first digit
		return IntDigits(16)
	case 'b', 'B':
		l.Next() // consume first digit
		return IntDigits(2)
	default:
		l.Backup()
		l.Emit(l.T, &big.Int{})
		return nil
	}
}

// IntDigits returns a state function that lexes the digits of an int in the
// given base. This function expects that the first digit has been read.
//
// Supported bases are 2, 8, 10 and 16.
//
// Number lexing stops at the first non-digit character.
// For bases 2 and 8 any digits not belonging to that number base will cause an error.
// "0x" and "0b" followed by non-digits are not reported as errors, rather a "0" literal is
// emitted and lexing resumes at 'x' or 'b' respecively.
//
func IntDigits(base int32) lexer.StateFn {
	return func(l *lexer.Lexer) lexer.StateFn {
		var t big.Int
		v := new(big.Int)
		here := l.TokenLen()
		r := l.Last()
		for {
			rl := unicode.ToLower(r)
			if rl >= 'a' {
				rl -= 'a' - '0' - 10
			}
			rl -= '0'
			if rl >= 0 && rl < base {
				t.SetInt64(int64(base))
				v = v.Mul(v, &t)
				t.SetInt64(int64(rl))
				v = v.Add(v, &t)
				r = l.Next()
				continue
			}
			if rl >= base && rl <= 9 {
				l.Errorf("invalid character %#U in base %d immediate value", r, base)
				// skip remaining digits.
				for r := l.Next(); r >= '0' && r <= '9'; r = l.Next() {
				}
				l.Backup()
				return nil
			}
			l.Backup()
			if (base == 2 || base == 16) && here > l.TokenLen() {
				// undo the trailing 'x' or 'b'
				l.Backup()
			}
			l.Emit(l.T, v)
			return nil
		}
	}
}

// EOF places the lexer.Lexer in End-Of-File state.
// Once in this state, the lexer.Lexer will only emit EOF.
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

// QuotedString lexes a Go string literal.
//
// When entering the StateFn, the starting delimiter has already been read and
// will be reused as end-delimiter.
//
func QuotedString(l *lexer.Lexer) lexer.StateFn {
	s := make([]byte, 0, 64)
	var rb [utf8.UTFMax]byte
	quote := l.Last()
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
			l.Emit(l.T, string(s))
			return nil
		case errEOL:
			l.Backup()
			l.Errorf(msg[errEOL], "string")
			return nil // keep going
		case errInvalidEscape, errInvalidRune:
			l.Errorf(msg[err])
			return terminateString(l, quote)
		case errInvalidHex, errInvalidOctal:
			l.Errorf(msg[err], l.Last())
			return terminateString(l, quote)
		}
	}
}

// QuotedChar lexes a Go character literal.
//
// When entering the StateFn, the starting delimiter has already been read and
// will be reused as end-delimiter.
//
func QuotedChar(l *lexer.Lexer) lexer.StateFn {
	quote := l.Last()
	r, err := readChar(l, quote)
	switch err {
	case errNone, errRawByte:
		n := l.Next()
		if n == quote {
			l.Emit(l.T, r)
			return nil
		}
		l.Backup() // undo a potential EOF/EOL
		l.Errorf(msg[errSize])
		return terminateString(l, quote)
	case errEnd:
		l.Errorf(msg[errEmpty], quote)
		return nil
	case errEOL:
		l.Backup()
		l.Errorf(msg[errEOL], "character literal")
		return nil // keep going
	case errInvalidEscape, errInvalidRune:
		l.Errorf(msg[err])
		return terminateString(l, quote)
	case errInvalidHex, errInvalidOctal:
		l.Errorf(msg[err], l.Last())
		return terminateString(l, quote)
	default:
		panic("unexpected return value from readChar")
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
				l.Discard()
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
				l.Discard()
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
