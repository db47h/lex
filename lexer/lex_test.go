package lexer_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

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

func initState(l *lexer.Lexer) {
	//	r :=
}

// Test proper behavior of Next/Peek/Backup
func TestLexer_Next(t *testing.T) {
	var l *lexer.Lexer
	next := func() rune { return l.Next() }
	last := func() rune { return l.Last() }
	peek := func() rune { return l.Peek() }
	backup := func() rune { l.Backup(); return l.Last() }

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
			{"al", last, 0, 'a'},
			{"bn1", next, 1, 'b'},
			{"bl1", last, 1, 'b'},
			{"bb", backup, 0, 'a'},
			{"bl2", last, 0, 'a'},
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
			{"cb", backup, -1, '\x00'}, // Pos() is invalid and Last() is garbage.
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
