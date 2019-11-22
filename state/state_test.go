package state_test

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/db47h/lex"
	"github.com/db47h/lex/state"
)

const (
	tokEOF lex.Token = iota
	tokInt
	tokFloat
	tokString
	tokChar
	tokColon
	tokRawChar
)

func itemString(l *lex.Lexer, t lex.Token, p int, v interface{}) string {
	var b strings.Builder
	pos := l.File().Position(p)
	b.WriteString(fmt.Sprintf("%d:%d ", pos.Line, pos.Column))
	var ts, vs string
	switch t {
	case lex.Error:
		ts = "Error"
		vs = v.(string)
	case tokEOF:
		ts = "EOF"
	case tokFloat:
		ts = "FLOAT"
		vs = v.(*big.Float).String()
	case tokInt:
		ts = "INT"
		vs = v.(*big.Int).String()
	case tokString:
		ts = "STRING"
		vs = strconv.Quote(v.(string))
	case tokChar:
		ts = "CHAR"
		vs = strconv.QuoteRune(v.(rune))
	case tokRawChar:
		ts = "RAWCHAR"
		vs = strconv.QuoteRune(v.(rune))
	case tokColon:
		ts = "COLON"
	default:
		panic("unknown type")
	}
	b.WriteString(ts)
	if vs != "" {
		b.WriteRune(' ')
		b.WriteString(vs)
	}
	return b.String()
}

type res []string

type testData struct {
	name string
	in   string
	res  res
}

func runTests(t *testing.T, td []testData, init lex.StateFn) {
	t.Helper()
	for _, sample := range td {
		t.Run(sample.name, func(t *testing.T) {
			l := lex.NewLexer(lex.NewFile(sample.name, strings.NewReader(sample.in)), init)
			var (
				tt lex.Token
				p  int
				v  interface{}
			)
			for i := range sample.res {
				tt, p, v = l.Lex()
				got := itemString(l, tt, p, v)
				if got != sample.res[i] {
					t.Errorf("\nGot     : %v\nExpected: %v", got, sample.res[i])
				}
			}
			tt, p, v = l.Lex()
			if tt != tokEOF || int(p) != utf8.RuneCountInString(sample.in) {
				pos := l.File().Position(utf8.RuneCountInString(sample.in))
				t.Errorf("Got: %s (Pos: %d), Expected: %d:%d EOF. ", itemString(l, tt, p, v), p, pos.Line, pos.Column)
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
	runTests(t, td, func(s *lex.State) lex.StateFn {
		r := s.Next()
		switch r {
		case '"':
			return state.QuotedString(tokString)
		case lex.EOF:
			s.Emit(s.Pos(), tokEOF, nil)
		case ' ', '\n', '\t':
			for r = s.Next(); r == ' ' || r == '\n' || r == '\t'; r = s.Next() {
			}
			s.Backup()
		default:
			s.Emit(s.Pos(), tokRawChar, r)
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
	runTests(t, td, func(s *lex.State) lex.StateFn {
		r := s.Next()
		switch r {
		case '\'':
			return state.QuotedChar(tokChar)
		case lex.EOF:
			s.Emit(s.Pos(), tokEOF, nil)
		case ' ', '\n', '\t':
			for r = s.Next(); r == ' ' || r == '\n' || r == '\t'; r = s.Next() {
			}
			s.Backup()
		default:
			s.Emit(s.Pos(), tokRawChar, r)
		}
		return nil
	})
}

func Test_Number(t *testing.T) {
	var td = []testData{
		{"int10", ":12 0 4", res{"1:1 COLON", "1:2 INT 12", "1:5 INT 0", "1:7 INT 4"}},
		{"int2", "0b011 0b111 0b0 0b", res{"1:1 INT 3", "1:7 INT 7", "1:13 INT 0", "1:19 Error malformed base 2 literal"}},
		{"int16", "0x0f0 0x101 0x2 0x", res{"1:1 INT 240", "1:7 INT 257", "1:13 INT 2", "1:19 Error malformed base 16 literal"}},
		{"int8", "017 07 0 08", res{"1:1 INT 15", "1:5 INT 7", "1:8 INT 0", "1:11 Error invalid character U+0038 '8' in base 8 literal"}},
		{`float1`, `.23 1.23`, res{"1:1 FLOAT 0.23", "1:5 FLOAT 1.23"}},
		{`float2`, `10e3`, res{`1:1 FLOAT 10000`}},
		{`float2b`, `10.e1`, res{`1:1 FLOAT 100`}},
		{`float3`, `10e-2`, res{`1:1 FLOAT 0.1`}},
		{`float4`, `a.b`, res{`1:1 RAWCHAR 'a'`, `1:2 RAWCHAR '.'`, `1:3 RAWCHAR 'b'`}},
		{`float5`, `.b`, res{`1:1 RAWCHAR '.'`, `1:2 RAWCHAR 'b'`}},
		{`float6`, `13.23e2`, res{`1:1 FLOAT 1323`}},
		{`float7`, `13.23e+2`, res{`1:1 FLOAT 1323`}},
		{`float8`, `13.23e-2`, res{`1:1 FLOAT 0.1323`}},
		{`float9`, `.23e3`, res{`1:1 FLOAT 230`}},
		{`float10`, `0777:123`, res{`1:1 INT 511`, `1:5 COLON`, `1:6 INT 123`}},
		{`float11`, `1eB:.e7:1ee`, res{
			`1:3 Error malformed floating-point literal exponent`, `1:3 RAWCHAR 'B'`, `1:4 COLON`,
			`1:5 RAWCHAR '.'`, `1:6 RAWCHAR 'e'`, `1:7 INT 7`, `1:8 COLON`,
			`1:11 Error malformed floating-point literal exponent`,
			`1:11 RAWCHAR 'e'`}},
		{`float12`, `:0238:`, res{`1:1 COLON`, `1:5 Error invalid character U+0038 '8' in base 8 literal`, `1:6 COLON`}},
	}
	runTests(t, td, func(s *lex.State) lex.StateFn {
		r := s.Next()
		s.StartToken(s.Pos())
		switch r {
		case lex.EOF:
			s.Emit(s.Pos(), tokEOF, nil)
		case ':':
			s.Emit(s.TokenPos(), tokColon, nil)
		case '.':
			r = s.Peek()
			if r < '0' || r > '9' {
				s.Emit(s.TokenPos(), tokRawChar, '.')
				return nil
			}
			fallthrough
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return state.Number(tokInt, tokFloat, '.')
		case ' ', '\n', '\t':
			for r = s.Next(); r == ' ' || r == '\n' || r == '\t'; r = s.Next() {
			}
			s.Backup()
		default:
			s.Emit(s.TokenPos(), tokRawChar, r)
		}
		return nil
	})
}
