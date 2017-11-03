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
	return l.Emit(l.T, nil, StateAny)
}

// StateAny is the initial state function where any token is expected.
// StateAny checks in order:
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
	r := l.Next()
	if r == EOF {
		return l.Emit(token.EOF, nil, StateAny)
	}
	if n := l.search(r); n != nil && n.s != nil {
		l.T = n.t
		return n.s
	}
	if f, t := l.l.filter(r); f != nil {
		l.T = t
		return f
	}
	return l.Emit(token.RawChar, r, StateAny)
}

// StateInt lexes an integer literal.
//
func StateInt(l *Lexer) StateFn {
	if l.B[len(l.B)-1] == '0' {
		return lexIntBase
	}
	return lexIntDigits(10, big.NewInt(int64(l.B[0]-'0')))
}

// lexIntBase reads the character following a leading 0 in order to determine
// the number base or directly emit a 0 literal.
//
// Supported number bases are 2, 8, 10 and 16.
//
func lexIntBase(l *Lexer) StateFn {
	r := l.Next()
	switch {
	case r >= '0' && r <= '9':
		// undo in order to let scanIntDigits read the whole number
		// (except the leading 0) or error appropriately if r is >= 8
		l.Backup()
		return lexIntDigits(8, new(big.Int))
	case r == 'x' || r == 'X':
		return lexIntDigits(16, new(big.Int))
	case r == 'b': // possible LocalLabel caught in scanIntDigits
		return lexIntDigits(2, new(big.Int))
	default:
		l.Backup()
		return l.Emit(l.T, &big.Int{}, StateAny)
	}
}

// lexIntDigits returns a state function that scans the 2nd to n digit of an
// int in the given base.
//
// Supported bases are 2, 8, 10 and 16.
//
// Number lexing stops at the first non-digit character.
// For bases 2 and 8 any digits not belonging to that number base will cause an error.
// "0x" and "0b" followed by non-digits are not reported as errors, rather a "0" literal is
// emitted and lexing resumes at 'x' or 'b' respecively.
//
func lexIntDigits(base int32, v *big.Int) StateFn {
	return func(l *Lexer) StateFn {
		var t big.Int
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
				if err := l.Errorf("invalid character %#U in base %d immediate value", r, base); err != nil {
					return nil
				}
				// skip remaining digits.
				for r := l.Next(); r >= '0' && r <= '9'; r = l.Next() {
				}
				l.Backup()
				l.Discard()
				return StateAny
			}
			l.Backup()
			return emitInt(l, base, v)
		}
	}
}

// emitInt is the final stage of int lexing for ints with len > 1. It checks if the
// immediate value is well-formed. (i.e the minimum amount of digits)
// then emits the appropriate value(s).
//
func emitInt(l *Lexer, base int32, value *big.Int) StateFn {
	// len is at least 2 for bases 2 and 16. i.e. we've read at least
	// "0b" or "0x").
	sz := len(l.B)
	if (base == 2 || base == 16) && sz < 3 {
		// undo the trailing 'x' or 'b'
		l.Backup()
	}
	return l.Emit(l.T, value, StateAny)
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
				if err = l.Errorf(err.Error()); err != nil {
					return nil
				}
				return StateAny
			}
			return l.Emit(l.T, s, StateAny)
		case '\\':
			r = l.Next()
			if r != '\n' && r != EOF {
				continue
			}
			fallthrough
		case '\n', EOF:
			l.Backup()
			if err := l.Errorf("unterminated string"); err != nil {
				return nil
			}
			return StateAny // keep going
		}
	}
}
