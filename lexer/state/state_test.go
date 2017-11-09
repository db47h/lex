package state_test

import (
	"fmt"
	"math/big"
	"strings"
	"testing"
	"unicode"

	"github.com/db47h/asm/lexer"
	"github.com/db47h/asm/lexer/state"
	"github.com/db47h/asm/token"
)

var lang *lexer.Lang

const (
	tokInt token.Type = iota
	tokIdentifier
	tokString
	tokColon
)

// define a simple language with Go-like identifiers, strings, numbers,
// ":" has its own token type, everything else is emitted as token.RawChar
func init() {
	lang = lexer.NewLang(func(l *lexer.Lexer) lexer.StateFn {
		r := l.Next()

		switch {
		// identifier
		case unicode.IsLetter(r) || r == '_':
			l.T = tokIdentifier
			l.AcceptWhile(func(r rune) bool {
				return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
			})
			return state.EmitString
		case unicode.IsSpace(r):
			// eat spaces
			l.AcceptWhile(unicode.IsSpace)
			l.Discard()
			return nil
		default:
			l.Emit(token.RawChar, r)
			return nil
		}
	})

	lang.MatchRunes(token.EOF, []rune{lexer.EOF}, state.EOF)
	lang.MatchAny(tokInt, []rune("0123456789"), state.Int)
	lang.Match(tokString, "\"", state.String)
	lang.Match(tokColon, ":", state.EmitNil)
}

type res []string

func itemString(l *lexer.Lexer, i *lexer.Item) string {
	p := l.File().Position(i.Pos)
	s := fmt.Sprintf("%d:%d ", p.Line, p.Column)
	if i.Type < 0 {
		return s + i.String()
	}
	switch i.Type {
	case tokInt:
		return s + "INT " + i.Value.(*big.Int).String()
	case tokIdentifier:
		return s + "IDENT " + i.Value.(string)
	case tokString:
		return s + "STRING " + i.Value.(string)
	case tokColon:
		return s + "COLON"
	}
	panic("unknown type")
}

func Test_all(t *testing.T) {
	var td = []struct {
		name string
		in   string
		res  res
	}{
		{"int10", ":12 0 4", res{"1:1 COLON", "1:2 INT 12", "1:5 INT 0", "1:7 INT 4"}},
		{"int2", "0b011 0b111 0b0 0b", res{"1:1 INT 3", "1:7 INT 7", "1:13 INT 0", "1:17 INT 0", "1:18 IDENT b"}},
		{"int16", "0x0f0 0x101 0x2 0x", res{"1:1 INT 240", "1:7 INT 257", "1:13 INT 2", "1:17 INT 0", "1:18 IDENT x"}},
		{"int8", "017 07 0 08", res{"1:1 INT 15", "1:5 INT 7", "1:8 INT 0", "1:11 Error invalid character U+0038 '8' in base 8 immediate value"}},
	}
	for _, sample := range td {
		t.Run(sample.name, func(t *testing.T) {
			l := lexer.New(token.NewFile(sample.name, strings.NewReader(sample.in)), lang)
			var it lexer.Item
			for i := range sample.res {
				it = l.Lex()
				got := itemString(l, &it)
				if got != sample.res[i] {
					t.Errorf("Got: %v, Expected: %v", got, sample.res[i])
				}
			}
			it = l.Lex()
			if it.Type != token.EOF {
				t.Errorf("Got: %s, Expected: EOF", itemString(l, &it))
			}
		})
	}
}
