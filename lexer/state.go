package lexer

import (
	"math/big"
	"strconv"
	"unicode"

	"github.com/db47h/asm/token"
)

// StateEmitTokenNilValue emits the current token with a nil value.
// Returns StateAny.
//
func StateEmitTokenNilValue(l *Lexer) StateFn {
	l.Emit(l.T, nil)
	return StateAny
}

// StateAny is the initial state function where any token is expected.
// StateAny calls Discard, then checks the return value from Next in order:
//
// EOF: emits token.EOF and returns nil.
//
// Tokens for exact rune matches in the language (registered with Match or MatchAny).
//
// Tokens registered with MatchFn.
//
// If no match is found, it will emit an Item with Token = token.RawChar and
// Value set to the rune value.
//
// This order must be taken into account when building a Lang.
//
func StateAny(l *Lexer) StateFn {
	l.Discard()
	r := l.Next()
	if r == EOF {
		l.Emit(token.EOF, nil)
		return StateEOF
	}
	if n := l.search(r); n != nil && n.s != nil {
		l.T = n.t
		return n.s
	}
	if f, t := l.l.filter(r); f != nil {
		l.T = t
		return f
	}
	l.Emit(token.RawChar, r)
	return StateAny
}

// StateInt lexes an integer literal.
//
func StateInt(l *Lexer) StateFn {
	if l.B[len(l.B)-1] == '0' {
		return lexIntBase
	}
	l.Backup()
	return lexIntDigits(10)
}

// lexIntBase reads the character following a leading 0 in order to determine
// the number base or directly emit a 0 literal.
//
// Supported number bases are 2, 8, 10 and 16.
//
func lexIntBase(l *Lexer) StateFn {
	r := l.Next()
	switch r {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// undo in order to let scanIntDigits read the whole number
		// (except the leading 0) or error appropriately if r is >= 8
		l.Backup()
		return lexIntDigits(8)
	case 'x', 'X':
		return lexIntDigits(16)
	case 'b', 'B':
		return lexIntDigits(2)
	default:
		l.Backup()
		l.Emit(l.T, &big.Int{})
		return StateAny
	}
}

// lexIntDigits returns a state function that scans the digits of an int in the given base.
//
// Supported bases are 2, 8, 10 and 16.
//
// Number lexing stops at the first non-digit character.
// For bases 2 and 8 any digits not belonging to that number base will cause an error.
// "0x" and "0b" followed by non-digits are not reported as errors, rather a "0" literal is
// emitted and lexing resumes at 'x' or 'b' respecively.
//
func lexIntDigits(base int32) StateFn {
	return func(l *Lexer) StateFn {
		var t big.Int
		v := new(big.Int)
		first := len(l.B) // position of first digit
		for {
			r := l.Next()
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
				l.Errorf("invalid character %#U in base %d immediate value", r, base)
				// skip remaining digits.
				for r := l.Next(); r >= '0' && r <= '9'; r = l.Next() {
				}
				l.Backup()
				return StateAny
			}
			l.Backup()
			if (base == 2 || base == 16) && len(l.B)-first < 3 {
				// undo the trailing 'x' or 'b'
				l.Backup()
			}
			l.Emit(l.T, v)
			return StateAny
		}
	}
}

// StateEOF places the lexer in End-Of-File state.
// Once in this state, the lexer will only emit EOF.
//
func StateEOF(l *Lexer) StateFn {
	l.Emit(token.EOF, nil)
	return StateEOF
}

// StateString lexes a " terminated string.
// TODO: split this in a helper function (akin to AcceptXXX) that matches a terminating quote
// not preceeded by an escape char "\"
// Also unquote reports just "invalid syntax", that's a pita.
func StateString(l *Lexer) StateFn {
	quote := l.B[0]
	for {
		r := l.Next()
		switch r {
		case quote:
			s, err := strconv.Unquote(l.TokenString())
			if err != nil {
				l.Errorf(err.Error())
				return StateAny
			}
			l.Emit(l.T, s)
			return StateAny

		case '\\':
			r = l.Next()
			if r != '\n' && r != EOF {
				continue
			}
			fallthrough
		case '\n', EOF:
			l.Backup()
			l.Errorf("unterminated string")
			return StateAny // keep going
		}
	}
}
