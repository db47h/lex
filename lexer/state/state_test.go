package state_test

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/db47h/asm/lexer"
	"github.com/db47h/asm/lexer/lang"
	"github.com/db47h/asm/lexer/state"
	"github.com/db47h/asm/token"
)

const (
	tokNumber token.Type = iota
	tokIdentifier
	tokString
	tokChar
	tokColon
	tokRawChar
)

// define a simple language with Go-like identifiers, strings, chars, numbers,
// ":" has its own token type, everything else is emitted as token.RawChar
func initLang1() lexer.StateFn {
	l := lang.New(func(l *lexer.Lexer) lexer.StateFn {
		r := l.Next()

		switch {
		// identifier
		case unicode.IsLetter(r) || r == '_':
			l.AcceptWhile(func(r rune) bool {
				return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
			})
			return state.EmitString(tokIdentifier)
		case unicode.IsSpace(r):
			// eat spaces
			l.AcceptWhile(unicode.IsSpace)
			l.Discard()
			return nil
		default:
			l.Emit(tokRawChar, r)
			return nil
		}
	})

	l.MatchRunes([]rune{lexer.EOF}, state.EOF)

	// Consume first digit for state.Int() when lexing numbers in base 2 or 16
	intBase2or16 := func(base int) lexer.StateFn {
		return func(l *lexer.Lexer) lexer.StateFn {
			l.Next()
			return state.Int(tokNumber, base)(l)
		}
	}

	// Numbers: integers only
	l.MatchAny([]rune("123456789"), state.Int(tokNumber, 10))
	l.Match("0", state.Int(tokNumber, 8))
	l.Match("0b", intBase2or16(2))
	l.Match("0B", intBase2or16(2))
	l.Match("0x", intBase2or16(16))
	l.Match("0X", intBase2or16(16))

	l.Match("\"", state.QuotedString(tokString))
	l.Match("'", state.QuotedChar(tokChar))
	l.Match(":", state.EmitNil(tokColon))

	return l.Init()
}

type res []string

func itemString(l *lexer.Lexer, i *lexer.Item) string {
	p := l.File().Position(i.Pos)
	s := fmt.Sprintf("%d:%d ", p.Line, p.Column)
	if i.Type < 0 {
		return s + i.String()
	}
	switch i.Type {
	case tokNumber:
		switch v := i.Value.(type) {
		case *big.Int:
			return s + "INT " + v.String()
		case *big.Float:
			return s + "FLOAT " + v.String()
		default:
			panic("illegal number type")
		}
	case tokIdentifier:
		return s + "IDENT " + i.Value.(string)
	case tokString:
		return s + "STRING " + strconv.Quote(i.Value.(string))
	case tokChar:
		return s + "CHAR " + strconv.QuoteRune(i.Value.(rune))
	case tokRawChar:
		return s + "RAWCHAR " + strconv.QuoteRune(i.Value.(rune))
	case tokColon:
		return s + "COLON"
	}
	panic("unknown type")
}

type testData struct {
	name string
	in   string
	res  res
}

func runTests(t *testing.T, init lexer.StateFn, td []testData) {
	for _, sample := range td {
		t.Run(sample.name, func(t *testing.T) {
			l := lexer.New(token.NewFile(sample.name, strings.NewReader(sample.in)), init)
			var it lexer.Item
			for i := range sample.res {
				it = l.Lex()
				got := itemString(l, &it)
				if got != sample.res[i] {
					t.Errorf("\nGot     : %v\nExpected: %v", got, sample.res[i])
				}
			}
			it = l.Lex()
			if it.Type != token.EOF || int(it.Pos) != utf8.RuneCountInString(sample.in) {
				p := l.File().Position(token.Pos(utf8.RuneCountInString(sample.in)))
				t.Errorf("Got: %s, Expected: %d:%d EOF", itemString(l, &it), p.Line, p.Column)
			}
		})
	}
}

func Test_all(t *testing.T) {
	var td = []testData{
		{"str1", `"abcd\"\\\a\b\f\n\r\v\t"`, res{`1:1 STRING "abcd\"\\\a\b\f\n\r\v\t"`}},
		{"str2", `"\xcC"`, res{`1:1 STRING "\xcc"`}},
		{"str3", `"\U0010FFFF \u2224"`, res{`1:1 STRING "\U0010ffff ∤"`}},
		{"str4", `"a\UFFFFFFFF" "\ud800 " "x\ud800\"\`, res{
			`1:12 Error escape sequence is invalid Unicode code point`,
			`1:21 Error escape sequence is invalid Unicode code point`,
			`1:32 Error escape sequence is invalid Unicode code point`,
		}},
		{"str5", `"a`, res{`1:2 Error unterminated string`}},
		{"str6", `"\x2X"`, res{`1:5 Error non-hex character in escape sequence: U+0058 'X'`}},
		{"str7", `"\277" "\28"`, res{`1:1 STRING "\xbf"`, `1:11 Error non-octal character in escape sequence: U+0038 '8'`}},
		{"str8", `"\w"`, res{`1:3 Error unknown escape sequence`}},
		{"str9", "\"a\n", res{`1:2 Error unterminated string`}},
		{"str10", "\"a\\\n", res{`1:3 Error unterminated string`}},
		{"str11", "\"\\21\n", res{`1:4 Error unterminated string`}},
		{"char1", `'a' ''`, res{`1:1 CHAR 'a'`, `1:6 Error empty character literal or unescaped ' in character literal`}},
		{"char2", `'aa'`, res{`1:2 Error invalid character literal (more than 1 character)`}},
		{"char4", `'\z' '
			`, res{`1:3 Error unknown escape sequence`, `1:6 Error unterminated character literal`}},
		{"char5", `'\18`, res{`1:4 Error non-octal character in escape sequence: U+0038 '8'`}},
		{"int10", ":12 0 4", res{"1:1 COLON", "1:2 INT 12", "1:5 INT 0", "1:7 INT 4"}},
		{"int2", "0b011 0b111 0b0 0b", res{"1:1 INT 3", "1:7 INT 7", "1:13 INT 0", "1:18 Error malformed base 2 immediate value"}},
		{"int16", "0x0f0 0x101 0x2 0x", res{"1:1 INT 240", "1:7 INT 257", "1:13 INT 2", "1:18 Error malformed base 16 immediate value"}},
		{"int8", "017 07 0 08", res{"1:1 INT 15", "1:5 INT 7", "1:8 INT 0", "1:11 Error invalid character U+0038 '8' in base 8 immediate value"}},
	}
	runTests(t, initLang1(), td)
}

func initLang2() lexer.StateFn {
	l := lang.New(func(l *lexer.Lexer) lexer.StateFn {
		r := l.Next()

		switch {
		// identifier
		case unicode.IsLetter(r) || r == '_':
			l.AcceptWhile(func(r rune) bool {
				return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
			})
			return state.EmitString(tokIdentifier)
		case unicode.IsSpace(r):
			// eat spaces
			l.AcceptWhile(unicode.IsSpace)
			l.Discard()
			return nil
		default:
			l.Emit(tokRawChar, r)
			return nil
		}
	})

	l.MatchRunes([]rune{lexer.EOF}, state.EOF)
	l.MatchAny([]rune(".0123456789"), func(l *lexer.Lexer) lexer.StateFn {
		if l.Last() == '.' {
			r := l.Peek()
			if r < '0' || r > '9' {
				// leading '.' not followed by a digit
				l.Emit(tokRawChar, '.')
				return nil
			}
		}
		return state.Number(tokNumber, '.', true)
	})
	l.Match(":", state.EmitNil(tokColon))

	return l.Init()
}

func Test_floats(t *testing.T) {
	var td = []testData{
		{`float1`, `1.23`, res{`1:1 FLOAT 1.23`}},
		{`float2`, `10.e3`, res{`1:1 FLOAT 10000`}},
		{`float2b`, `10.`, res{`1:1 FLOAT 10`}},
		{`float3`, `10e-2`, res{`1:1 FLOAT 0.1`}},
		{`float4`, `a.b`, res{`1:1 IDENT a`, `1:2 RAWCHAR '.'`, `1:3 IDENT b`}},
		{`float5`, `.b`, res{`1:1 RAWCHAR '.'`, `1:2 IDENT b`}},
		{`float6`, `13.23e2`, res{`1:1 FLOAT 1323`}},
		{`float7`, `13.23e+2`, res{`1:1 FLOAT 1323`}},
		{`float8`, `13.23e-2`, res{`1:1 FLOAT 0.1323`}},
		{`float9`, `.23e3`, res{`1:1 FLOAT 230`}},
		{`float10`, `0777:123`, res{`1:1 INT 511`, `1:5 COLON`, `1:6 INT 123`}},
		{`float11`, `1eB:.e7:1e`, res{
			`1:2 Error malformed malformed floating-point constant exponent`, `1:3 IDENT B`, `1:4 COLON`,
			`1:5 RAWCHAR '.'`, `1:6 IDENT e7`, `1:8 COLON`,
			`1:10 Error malformed malformed floating-point constant exponent`}},
	}
	runTests(t, initLang2(), td)
}
