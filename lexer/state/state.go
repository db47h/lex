package state

import (
	"math/big"
	"strconv"
	"unicode"

	"github.com/db47h/asm/lexer"
	"github.com/db47h/asm/token"
)

// EmitTokenNil emits the current token with a nil value.
//
func EmitTokenNil(l *lexer.Lexer) lexer.StateFn {
	l.Emit(l.T, nil)
	return nil
}

// EmitTokenString emits the current token with a string value.
//
func EmitTokenString(l *lexer.Lexer) lexer.StateFn {
	l.Emit(l.T, l.TokenString())
	return nil
}

// Int lexes an integer literal.
//
func Int(l *lexer.Lexer) lexer.StateFn {
	if t := l.Token(); t[len(t)-1] == '0' {
		return lexIntBase
	}
	l.Backup()
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
		l.Backup()
		return IntDigits(8)
	case 'x', 'X':
		return IntDigits(16)
	case 'b', 'B':
		return IntDigits(2)
	default:
		l.Backup()
		l.Emit(l.T, &big.Int{})
		return nil
	}
}

// IntDigits returns a state function that scans the digits of an int in the given base.
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
		var dc int
		for {
			r := l.Next()
			rl := unicode.ToLower(r)
			if rl >= 'a' {
				rl -= 'a' - '0' + 10
			}
			rl -= '0'
			if rl >= 0 && rl < base {
				dc++
				t.SetInt64(int64(base))
				v = v.Mul(v, &t)
				t.SetInt64(int64(rl))
				v = v.Add(v, &t)
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
			if (base == 2 || base == 16) && dc == 0 {
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

// String lexes a " terminated string.
// TODO: split this in a helper function (akin to AcceptXXX) that matches a terminating quote
// not preceeded by an escape char "\"
// Also unquote reports just "invalid syntax", that's a pita.
func String(l *lexer.Lexer) lexer.StateFn {
	quote := l.Token()[0]
	for {
		r := l.Next()
		switch r {
		case quote:
			s, err := strconv.Unquote(l.TokenString())
			if err != nil {
				l.Errorf(err.Error())
				return nil
			}
			l.Emit(l.T, s)
			return nil

		case '\\':
			if r = l.Next(); r != '\n' && r != lexer.EOF {
				continue
			}
			fallthrough
		case '\n', lexer.EOF:
			l.Backup()
			l.Errorf("unterminated string")
			return nil // keep going
		}
	}
}
