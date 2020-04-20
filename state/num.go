// Copyright 2017-2020 Denis Bernard <db047h@gmail.com>
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

package state

import (
	"math/big"

	"github.com/db47h/lex"
)

const (
	errMalformedInt      = "malformed base %d literal"
	errInvalidNumChar    = "invalid character %#U in base %d literal"
	errMalformedFloat    = "malformed floating-point literal"
	errMalformedExponent = "malformed floating-point literal exponent"
)

// A numberLexer lexes numbers.
//
type numberLexer struct {
	tokInt     lex.Token // token type for integers
	tokFloat   lex.Token // token type for floats
	buf        []byte
	base       int
	decimalSep rune // decimal separator

}

// Number returns a lex.StateFn that lexes numbers.
//
// For integers, the number base is determined by the number prefix. A prefix of
// “0x” or “0X” selects base 16; a “0b” or “0B”  prefix selects base 2 and the
// “0” prefix selects base 8 (unless the number is a floating point literal,
// which are always in base 10). Otherwise the selected base is 10.
//
// tokInt is the token type returned for integers.
//
// tokFloat is the token type returned for floats.
//
// decimalSep sets the decimal separator.
//
// The return value from Number is not safe to use concurrently.
//
// The StateFn will panic on invalid input. i.e. callers must make sure that
// the input starts with either a digit or a decimal separator followed by a
// digit:
//
//	switch s.Next() {
//	case EOF:
//		s.Emit(s.Pos(), tokEOF, nil)
//		return nil
//	case decimalSeparator:
//		if r := s.Peek(); r >= 0 && r <= 0 {
//			return state.Number(tokInt, tokFloat, decimalSeparator)
//		}
//		// return whatever token or error matches a single decimalSeparator
//	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
//		return state.Number(tokInt, tokFloat, decimalSeparator)
//	default:
//		// ...
//	}
//
// The implementation of this lexer makes heavy use of the state-as-function
// paradigm. As a result it is not the fastest by a long stretch. On the other
// hand it is a good example for the lexer package.
//
func Number(tokInt, tokFloat lex.Token, decimalSep rune) lex.StateFn {
	l := &numberLexer{
		tokInt:     tokInt,
		tokFloat:   tokFloat,
		decimalSep: decimalSep,
		buf:        make([]byte, 0, 64),
		base:       10,
	}
	return l.stateNumber
}

// stateNumber is the main entry point for numbers.
//
func (l *numberLexer) stateNumber(s *lex.State) lex.StateFn {
	r := s.Current()
	switch r {
	case '0':
		switch s.Next() {
		case 'x', 'X':
			s.Next()
			l.base = 16
			return l.stateInteger
		case 'b', 'B':
			s.Next()
			l.base = 2
			return l.stateInteger
		}
		s.Backup()
		fallthrough
	case l.decimalSep, '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return l.stateIntegerOrFloat
	}
	panic("not a number")
}

// integer returns a StateFn that lexes integers in the given base.
// Base must be between >= 2 and <= 36, no prefixes allowed.
//
func (l *numberLexer) stateInteger(s *lex.State) lex.StateFn {
	l.buf = l.buf[:0]
	l.scanDigits(s, l.base)
	// for bases < 10, consider digits >= base following the constant to be an error
	if r := s.Current(); r >= '0' && r <= '9' {
		s.Errorf(s.Pos(), errInvalidNumChar, r, l.base)
		// skip remaining digits
		for r = s.Next(); r >= '0' && r <= '9'; r = s.Next() {
		}
		s.Backup()
		return nil
	}
	return l.stateEmitInt
}

// base 8 integer, base 10 integer, base 10 integer with exponent, or float.
// i.e. anything of the form [0-9]*\.[0-9]*
func (l *numberLexer) stateIntegerOrFloat(s *lex.State) lex.StateFn {
	var (
		r8 rune // keep track of end-of base 8 literal
		p8 int  = -1
	)
	l.buf = l.buf[:0]
	if s.Current() == '0' {
		l.scanDigits(s, 8)
		r8 = s.Current()
		p8 = s.Pos()
	}
	// keep scanning as a base 10 integer, check later
	l.scanDigits(s, 10)

	// float ?
	switch s.Current() {
	case l.decimalSep:
		return l.stateFractional
	case 'e':
		return l.stateExponent
	}

	// integer
	if l.buf[0] == '0' {
		l.base = 8
		if p8 != s.Pos() {
			s.Errorf(p8, errInvalidNumChar, r8, 8)
			// second scanDigits call has already consumed remaining 0-9 digits
			s.Backup()
			return nil
		}
	}
	return l.stateEmitInt
}

func (l *numberLexer) stateEmitInt(s *lex.State) lex.StateFn {
	switch {
	case len(l.buf) == 0:
		s.Errorf(s.Pos(), errMalformedInt, l.base)
	default:
		i, ok := new(big.Int).SetString(string(l.buf), l.base)
		if !ok {
			panic("Int.SetString failed")
		}
		s.Emit(s.TokenPos(), l.tokInt, i)
	}
	s.Backup()
	return nil
}

func (l *numberLexer) stateFractional(s *lex.State) lex.StateFn {
	l.buf = append(l.buf, '.')
	s.Next()
	l.scanDigits(s, 10)
	if len(l.buf) == 1 {
		s.Errorf(s.Pos(), errMalformedFloat)
		s.Backup()
		return nil
	}
	if s.Current() == 'e' {
		return l.stateExponent
	}
	return l.stateEmitFloat
}

func (l *numberLexer) stateEmitFloat(s *lex.State) lex.StateFn {
	z, ok := new(big.Float).SetString(string(l.buf))
	if !ok {
		panic("Float.SetString failed")
	}
	s.Backup()
	s.Emit(s.TokenPos(), l.tokFloat, z)
	return nil
}

func (l *numberLexer) stateExponent(s *lex.State) lex.StateFn {
	l.buf = append(l.buf, 'e')
	if r := s.Next(); r == '-' || r == '+' {
		l.buf = append(l.buf, byte(r))
		s.Next()
	}
	bl := len(l.buf)
	l.scanDigits(s, 10)
	if len(l.buf) > bl {
		return l.stateEmitFloat
	}
	// no digits following 'e'
	s.Errorf(s.Pos(), errMalformedExponent)
	s.Backup()
	return nil
}

func (l *numberLexer) scanDigits(s *lex.State, base int) {
	r := s.Current()
	for {
		var rl rune
		switch {
		case r >= 'a':
			rl = r - 'a' + 10
		case r >= 'A':
			rl = r - 'A' + 10
		default:
			rl = r - '0'
		}
		if rl < 0 || int(rl) >= base {
			return
		}
		l.buf = append(l.buf, byte(r))
		r = s.Next()
	}
}
