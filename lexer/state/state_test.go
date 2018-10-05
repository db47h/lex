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

func itemString(l *lexer.Lexer, t token.Type, p token.Pos, v interface{}) string {
	var b strings.Builder
	pos := l.File().Position(p)
	b.WriteString(fmt.Sprintf("%d:%d ", pos.Line, pos.Column))
	var ts, vs string
	switch t {
	case token.Error:
		ts = "Error"
		vs = v.(string)
	case token.EOF:
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

func runTests(t *testing.T, td []testData, init lexer.StateFn) {
	t.Helper()
	for _, sample := range td {
		t.Run(sample.name, func(t *testing.T) {
			l := lexer.New(token.NewFile(sample.name, strings.NewReader(sample.in)), init)
			var (
				tt token.Type
				p  token.Pos
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
			if tt != token.EOF || int(p) != utf8.RuneCountInString(sample.in) {
				pos := l.File().Position(token.Pos(utf8.RuneCountInString(sample.in)))
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
	runTests(t, td, func(l *lexer.State) lexer.StateFn {
		r := l.Next()
		switch r {
		case '"':
			return state.QuotedString(tokString)
		case lexer.EOF:
			return lexer.StateEOF
		case ' ', '\n', '\t':
			for r = l.Next(); r == ' ' || r == '\n' || r == '\t'; r = l.Next() {
			}
			l.Backup()
		default:
			l.Emit(l.Pos(), tokRawChar, r)
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
	runTests(t, td, func(l *lexer.State) lexer.StateFn {
		r := l.Next()
		switch r {
		case '\'':
			return state.QuotedChar(tokChar)
		case lexer.EOF:
			return lexer.StateEOF
		case ' ', '\n', '\t':
			for r = l.Next(); r == ' ' || r == '\n' || r == '\t'; r = l.Next() {
			}
			l.Backup()
		default:
			l.Emit(l.Pos(), tokRawChar, r)
		}
		return nil
	})
}
