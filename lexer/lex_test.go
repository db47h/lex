package lexer_test

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/db47h/parsekit/lexer"
	"github.com/db47h/parsekit/token"
)

type testData struct {
	name  string
	input string
	res   res
}

type res []string

const (
	tokSpace token.Type = iota
	tokChar
)

func tString(i *lexer.Item) string {
	if i.Type < 0 {
		return i.String()
	}
	switch i.Type {
	case tokSpace:
		return "SPACE"
	// case tokString:
	// 	return "STRING " + strconv.Quote(i.Value.(string))
	case tokChar:
		return "CHAR " + strconv.QuoteRune(i.Value.(rune))
	default:
		panic(fmt.Sprintf("unknown token type %d", i.Type))
	}
}

// Test proper behavior of Next/Peek/Backup
func TestLexer_Next(t *testing.T) {
	var l *lexer.Lexer
	next := func() rune { return l.Next() }
	cur := func() rune { return l.Current() }
	peek := func() rune { return l.Peek() }
	backup := func() rune { l.Backup(); return l.Current() }

	input := []string{
		"ab",
		"c",
		"\n\n",
	}

	data := [][]struct {
		name string
		fn   func() rune
		p    token.Pos
		r    rune
	}{
		{
			{"an", next, 0, 'a'},
			{"al", cur, 0, 'a'},
			{"bn1", next, 1, 'b'},
			{"bl1", cur, 1, 'b'},
			{"bb", backup, 0, 'a'},
			{"bl2", cur, 0, 'a'},
			{"bp1", peek, 0, 'b'},
			{"bn2", next, 1, 'b'},
			{"bp2", peek, 1, lexer.EOF},
			{"eof1", next, 2, lexer.EOF},
			{"eofb", backup, 1, 'b'},
			{"eof2", next, 2, lexer.EOF},
			{"eof3", next, 2, lexer.EOF},
			{"eofp1", peek, 2, lexer.EOF},
			{"eofb2", backup, 1, 'b'},
			{"eofp2", peek, 1, lexer.EOF},
		},
		{
			{"cn", next, 0, 'c'},
			{"cb", backup, -1, '\x00'}, // Pos() is invalid and Current() is garbage.
			{"cn", next, 0, 'c'},
			{"eofn", next, 1, lexer.EOF},
		},
		{
			{"nl1", next, 0, '\n'},
			{"nl2", peek, 0, '\n'},
		},
	}

	for i, in := range input {
		l = lexer.New(token.NewFile("", strings.NewReader(in)), nil).(*lexer.Lexer)
		for _, td := range data[i] {
			t.Run(td.name, func(t *testing.T) {
				r := td.fn()
				if r != td.r {
					t.Errorf("expected %q, got %q", td.r, r)
				}
				if l.Pos() != td.p {
					t.Errorf("expected pos %d, got %d", td.p, l.Pos())
				}
			})
		}
	}

	// check newlines on last test.
	for i := 0; i < 3; i++ {
		p := l.File().Position(token.Pos(i))
		if p.Line != i+1 {
			t.Errorf("expected line %d, got %d", i+1, p.Line)
		}
	}
}

func TestLexer_Lex(t *testing.T) {
	var stateEOF lexer.StateFn
	stateEOF = func(l *lexer.Lexer) lexer.StateFn {
		l.Emit(token.EOF, nil)
		return stateEOF
	}

	var num int64
	var base int64

	scanDigit := func(r rune) bool {
		r = unicode.ToLower(r)
		if r >= 'a' {
			r -= 'a' - '0' - 10
		}
		r -= '0'
		if r >= 0 && r < rune(base) {
			num = num*base + int64(r)
			return true
		}
		return false
	}

	stateNum := func(l *lexer.Lexer) lexer.StateFn {
		num = 0
		base = 10
		r := l.Current()
		if r == '0' {
			if l.Accept('x') || l.Accept('X') {
				base = 16
			} else {
				base = 8
			}
			r = l.Next()
		}
		if scanDigit(r) {
			l.AcceptWhile(scanDigit)
		}
		l.Emit(0, big.NewInt(num))
		if base == 8 {
			l.Errorf(l.Pos(), "piling up")
			l.Errorf(l.Pos(), "things")
		}
		return nil
	}

	f := token.NewFile("test", strings.NewReader("0x24 12 0666 |"))
	r := []string{
		"Type(0) 36",
		"Type(0) 12",
		"Type(0) 438",
		"Error piling up",
		"Error things",
		"EOF",
	}
	l := lexer.New(f,
		func(l *lexer.Lexer) lexer.StateFn {
			r := l.Current()
			switch r {
			case lexer.EOF:
				return stateEOF
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				return stateNum
			case '|':
				// end marker
				if !l.Expect(lexer.EOF) {
					panic("| not @EOF")
				}
			}
			return nil
		})
	for i := 0; ; i++ {
		it := l.Lex()
		if it.String() != r[i] {
			t.Errorf("Got: %v, expected: %v", it.String(), r[i])
		}
		if it.Type == token.EOF {
			break
		}
	}
}
