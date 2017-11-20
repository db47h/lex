package state_test

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/db47h/parsekit/lexer/state"

	"github.com/db47h/parsekit/lexer"
	"github.com/db47h/parsekit/token"
)

const (
	tokInt token.Type = iota
	tokFloat
	tokString
	tokChar
	tokColon
	tokRawChar
)

func itemString(l lexer.Interface, i *lexer.Item) string {
	p := l.File().Position(i.Pos)
	s := fmt.Sprintf("%d:%d ", p.Line, p.Column)
	if i.Type < 0 {
		return s + i.String()
	}
	switch i.Type {
	case tokFloat:
		return s + "FLOAT " + i.Value.(*big.Float).String()
	case tokInt:
		return s + "INT " + i.Value.(*big.Int).String()
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

type res []string

type testData struct {
	name string
	in   string
	res  res
}

func runTests(t *testing.T, td []testData, init lexer.StateFn) {
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
				t.Errorf("Got: %s (Pos: %d), Expected: %d:%d EOF. ", itemString(l, &it), it.Pos, p.Line, p.Column)
			}
		})
	}
}

func Test_QuotedString(t *testing.T) {
	var td = []testData{
		{"str1", `"abcd\"\\\a\b\f\n\r\v\t"`, res{`1:1 STRING "abcd\"\\\a\b\f\n\r\v\t"`}},
		{"str2", `"\xcC"`, res{`1:1 STRING "\xcc"`}},
		{"str3", `"\U0010FFFF \u2224"`, res{`1:1 STRING "\U0010ffff ∤"`}},
		{"str4", `"a\UFFFFFFFF" "\ud800 " "x\ud800\"\`, res{
			`1:12 Error escape sequence is invalid Unicode code point`,
			`1:21 Error escape sequence is invalid Unicode code point`,
			`1:32 Error escape sequence is invalid Unicode code point`,
		}},
		{"str5", `"a`, res{`1:1 Error unterminated string`}},
		{"str6", `"\x2X"`, res{`1:5 Error non-hex character in escape sequence: U+0058 'X'`}},
		{"str7", `"\277" "\28"`, res{`1:1 STRING "\xbf"`, `1:11 Error non-octal character in escape sequence: U+0038 '8'`}},
		{"str8", `"\w"`, res{`1:3 Error unknown escape sequence`}},
		{"str9", "\"a\n", res{`1:1 Error unterminated string`}},
		{"str10", "\"a\\\n", res{`1:1 Error unterminated string`}},
		{"str11", "\"\\21\n", res{`1:1 Error unterminated string`}},
	}
	runTests(t, td, func(l *lexer.Lexer) lexer.StateFn {
		r := l.Next()
		switch r {
		case '"':
			return state.QuotedString(tokString)
		case lexer.EOF:
			return state.EOF
		case ' ', '\n', '\t':
			l.AcceptWhile(func(r rune) bool { return r == ' ' || r == '\n' || r == '\t' })
		default:
			l.Emit(tokRawChar, r)
		}
		return nil
	})
}

func Test_QuotedChar(t *testing.T) {
	var td = []testData{
		{"char1", `'a' ''`, res{`1:1 CHAR 'a'`, `1:6 Error empty character literal or unescaped ' in character literal`}},
		{"char2", `'aa'`, res{`1:3 Error invalid character literal (more than 1 character)`}},
		{"char4", `'\z' '
			`, res{`1:3 Error unknown escape sequence`, `1:6 Error unterminated character literal`}},
		{"char5", `'\18`, res{`1:4 Error non-octal character in escape sequence: U+0038 '8'`}},
	}
	runTests(t, td, func(l *lexer.Lexer) lexer.StateFn {
		r := l.Next()
		switch r {
		case '\'':
			return state.QuotedChar(tokChar)
		case lexer.EOF:
			return state.EOF
		case ' ', '\n', '\t':
			l.AcceptWhile(func(r rune) bool { return r == ' ' || r == '\n' || r == '\t' })
		default:
			l.Emit(tokRawChar, r)
		}
		return nil
	})
}

func Test_Int(t *testing.T) {
	var td = []testData{
		{"int10", ":12 0 4", res{"1:1 COLON", "1:2 INT 12", "1:5 INT 0", "1:7 INT 4"}},
		{"int2", "0b011 0b111 0b0 0b", res{"1:1 INT 3", "1:7 INT 7", "1:13 INT 0", "1:19 Error malformed base 2 immediate value"}},
		{"int16", "0x0f0 0x101 0x2 0x", res{"1:1 INT 240", "1:7 INT 257", "1:13 INT 2", "1:19 Error malformed base 16 immediate value"}},
		{"int8", "017 07 0 08", res{"1:1 INT 15", "1:5 INT 7", "1:8 INT 0", "1:11 Error invalid character U+0038 '8' in base 8 immediate value"}},
	}
	runTests(t, td, func(l *lexer.Lexer) lexer.StateFn {
		r := l.Next()
		switch r {
		case '0':
			r = l.Next()
			switch r {
			case 'x', 'X':
				l.Next()
				return state.Int(tokInt, 16)
			case 'b', 'B':
				l.Next()
				return state.Int(tokInt, 2)
			default:
				l.Backup() // backup leading 0 just in case it's a single 0.
				return state.Int(tokInt, 8)
			}
		case '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return state.Int(tokInt, 10)
		case lexer.EOF:
			return state.EOF
		case ':':
			l.Emit(tokColon, nil)
			return nil
		case ' ', '\n', '\t':
			l.AcceptWhile(func(r rune) bool { return r == ' ' || r == '\n' || r == '\t' })
		default:
			l.Emit(tokRawChar, r)
		}
		return nil
	})
}

func Test_Number(t *testing.T) {
	var td = []testData{
		{`float1`, `1.23`, res{`1:1 FLOAT 1.23`}},
		{`float2`, `10.e3`, res{`1:1 FLOAT 10000`}},
		{`float2b`, `10.`, res{`1:1 FLOAT 10`}},
		{`float3`, `10e-2`, res{`1:1 FLOAT 0.1`}},
		{`float4`, `a.b`, res{`1:1 RAWCHAR 'a'`, `1:2 RAWCHAR '.'`, `1:3 RAWCHAR 'b'`}},
		{`float5`, `.b`, res{`1:1 RAWCHAR '.'`, `1:2 RAWCHAR 'b'`}},
		{`float6`, `13.23e2`, res{`1:1 FLOAT 1323`}},
		{`float7`, `13.23e+2`, res{`1:1 FLOAT 1323`}},
		{`float8`, `13.23e-2`, res{`1:1 FLOAT 0.1323`}},
		{`float9`, `.23e3`, res{`1:1 FLOAT 230`}},
		{`float10`, `0777:123`, res{`1:1 INT 511`, `1:5 COLON`, `1:6 INT 123`}},
		{`float11`, `1eB:.e7:1ee`, res{
			`1:3 Error malformed floating-point constant exponent`, `1:3 RAWCHAR 'B'`, `1:4 COLON`,
			`1:5 RAWCHAR '.'`, `1:6 RAWCHAR 'e'`, `1:7 INT 7`, `1:8 COLON`,
			`1:11 Error malformed floating-point constant exponent`,
			`1:11 RAWCHAR 'e'`}},
		{`float12`, `:0238:`, res{`1:1 COLON`, `1:5 Error invalid character U+0038 '8' in base 8 immediate value`, `1:6 COLON`}},
	}
	runTests(t, td, func(l *lexer.Lexer) lexer.StateFn {
		r := l.Next()
		switch r {
		case '.':
			r = l.Peek()
			if r < '0' || r > '9' {
				l.Emit(tokRawChar, '.')
				return nil
			}
			fallthrough
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return state.Number(tokInt, tokFloat, '.', true)
		case ':':
			l.Emit(tokColon, nil)
		case lexer.EOF:
			return state.EOF
		case ' ', '\n', '\t':
			l.AcceptWhile(func(r rune) bool { return r == ' ' || r == '\n' || r == '\t' })
		default:
			l.Emit(tokRawChar, r)
		}
		return nil
	})
}
