package state

import (
	"math/big"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/db47h/asm/lexer"
	"github.com/db47h/asm/token"
)

// EmitNil returns a state function that emits the given token type with a nil value.
//
func EmitNil(t token.Type) lexer.StateFn {
	return func(l *lexer.Lexer) lexer.StateFn {
		l.Emit(t, nil)
		return nil
	}
}

// EmitString returns a state function that emits the current token with the
// given token type and a string value.
//
func EmitString(t token.Type) lexer.StateFn {
	return func(l *lexer.Lexer) lexer.StateFn {
		l.Emit(t, l.TokenString())
		return nil
	}
}

// Int returns a state function that lexes the digits of an int in the given
// base amd emits it as a big.Float. This function expects that the first
// digit has been read.
//
// Supported bases are 2, 8, 10 and 16.
//
// Number lexing stops at the first non-digit character.
//
func Int(t token.Type, base int) lexer.StateFn {
	return func(l *lexer.Lexer) lexer.StateFn {
		start := l.TokenLen() - 1
		r := l.Last()
		for {
			rl := unicode.ToLower(r)
			if rl >= 'a' {
				rl -= 'a' - '0' - 10
			}
			rl -= '0'
			if rl >= 0 && rl < int32(base) {
				r = l.Next()
				continue
			}
			// for bases 2 and 8, consider that an invalid digit is an error instead
			// of an end of token: error then skip remaining digits.
			if rl >= int32(base) && rl <= 9 {
				l.Errorf(l.Pos(), "invalid character %#U in base %d immediate value", r, base)
				for r := l.Next(); r >= '0' && r <= '9'; r = l.Next() {
				}
				l.Backup()
				return nil
			}
			if l.TokenLen() == start+1 {
				// no digits!
				pos := l.Pos()
				l.Backup()
				l.Errorf(pos, "malformed base %d immediate value", base)
				return nil
			}
			l.Backup()

			z := new(big.Int)
			z, _ = z.SetString(string(l.Token()[start:]), base)
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

// Number returns a StateFn that lexes an integer or float literal then emits it
// with the given type and either *big.Int or *big.Float value. This function
// expects that the first digit or leading decimal separator has already been
// read. The octal parameter indicates if integer literals starting with a
// leading '0' should be treated as octal numbers.
//
func Number(tokInt, tokFloat token.Type, decimalSep rune, octal bool) lexer.StateFn {
	return func(l *lexer.Lexer) lexer.StateFn {
		s := l.TokenLen() - 1
		r := l.Last()
		pos := l.Pos()
		base := 10
		switch r {
		case decimalSep:
			return numFP(l, tokFloat, s)
		case '0':
			if octal {
				base = 8
			}
			fallthrough
		case '1', '2', '3', '4', '5', '6', '7', '8', '9':
			l.AcceptWhile(isDigit)
			if r = l.Peek(); r == decimalSep || r == 'e' {
				l.Next()
				return numFP(l, tokFloat, s)
			}
		default:
			panic("Not a number")
		}

		// integer
		tok := l.Token()[s:]
		if octal && tok[0] == '0' {
			// check digits
			for i, r := range tok[1:] {
				if r >= '0'+rune(base) {
					l.Errorf(pos+token.Pos(i)+1, "invalid character %#U in base %d immediate value", r, base)
					return nil
				}
			}
		}
		z, _ := new(big.Int).SetString(string(tok), base)
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
func numFP(l *lexer.Lexer, t token.Type, start int) lexer.StateFn {
	r := l.Last()
	if r != 'e' {
		l.Token()[l.TokenLen()-1] = '.'
		l.AcceptWhile(isDigit)
		r = l.Next()
	}
	if r == 'e' {
		r = l.Peek()
		switch r {
		case '+', '-':
			l.Next()
		}
		if l.AcceptWhile(isDigit) == 0 {
			l.Errorf(l.Pos()+1, "malformed malformed floating-point constant exponent")
			return nil
		}
	} else {
		l.Backup()
	}
	z, _, err := big.ParseFloat(string(l.Token()[start:]), 10, 1024, big.ToNearestEven)
	if err != nil {
		panic(err)
	}
	l.Emit(t, z)
	return nil
}

// EOF places the lexer.Lexer in End-Of-File state.
// Once in this state, the lexer.Lexer will only emit EOF.
//
func EOF(l *lexer.Lexer) lexer.StateFn {
	l.Emit(token.EOF, nil)
	return EOF
}

// Errorf returns a StateFn that will call Lexer.Errorf with the given arguments.
//
func Errorf(msg string, args ...interface{}) lexer.StateFn {
	return func(l *lexer.Lexer) lexer.StateFn {
		l.Errorf(l.Pos(), msg, args...)
		return nil
	}
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

// QuotedString returns a StateFn that lexes a Go string literal.
//
// When entering the StateFn, the starting delimiter has already been read and
// will be reused as end-delimiter.
//
func QuotedString(t token.Type) lexer.StateFn {
	return func(l *lexer.Lexer) lexer.StateFn {
		s := make([]byte, 0, 64)
		var rb [utf8.UTFMax]byte
		quote := l.Last()
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
				l.Errorf(l.Pos(), msg[err], l.Last())
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
		quote := l.Last()
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
			l.Errorf(l.Pos(), msg[err], l.Last())
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

// IfMatchAny builds a conditional StateFn that will call thenFn if the
// next rune matches any of the runes in the match string, otherwise it will
// call elseFn.
//
func IfMatchAny(match string, thenFn, elseFn lexer.StateFn) lexer.StateFn {
	return func(l *lexer.Lexer) lexer.StateFn {
		// TODO: sort match, use a binary search.
		r := l.Next()
		if strings.ContainsRune(match, r) {
			return thenFn(l)
		}
		l.Backup()
		return elseFn(l)
	}
}
