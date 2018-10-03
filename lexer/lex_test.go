package lexer_test

import (
	"fmt"
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

func tString(t token.Type, p token.Pos, v interface{}) string {
	switch t {
	case token.EOF:
		return "EOF"
	case token.Error:
		return "error: " + v.(string)
	case tokSpace:
		return "SPACE"
	// case tokString:
	// 	return "STRING " + strconv.Quote(i.Value.(string))
	case tokChar:
		return "CHAR " + strconv.QuoteRune(v.(rune))
	default:
		panic(fmt.Sprintf("unknown token type %d", t))
	}
}

// Test proper behavior of Next/Peek/Backup
func TestLexer_Next(t *testing.T) {
	next := func(l *lexer.State) rune { return l.Next() }
	peek := func(l *lexer.State) rune { return l.Peek() }
	backup := func(l *lexer.State) rune { l.Backup(); return 0 }

	input := []string{
		"ab",
		"c",
		"\n\n",
	}

	data := [][]struct {
		name string
		fn   func(l *lexer.State) rune
		p    token.Pos
		r    rune
	}{
		{
			{"an", next, 0, 'a'},
			{"bn1", next, 1, 'b'},
			{"bb", backup, 0, 0},
			{"bp1", peek, 0, 'b'},
			{"bn2", next, 1, 'b'},
			{"bn3", next, 2, lexer.EOF},
			{"bb1", backup, 1, 0},
			{"bp2", peek, 1, lexer.EOF},
			{"eof1", next, 2, lexer.EOF},
			{"eofb", backup, 1, 0},
			{"eof2", next, 2, lexer.EOF},
			{"eof3", next, 2, lexer.EOF},
			{"eofb2", backup, 1, 0},
			{"eofp2", peek, 1, lexer.EOF},
		},
		{
			{"cn0", next, 0, 'c'},
			{"cbb0", backup, 0, 0},
			{"cn1", next, 0, 'c'},
			{"cbb1", backup, 0, 0},
			{"cn2", next, 0, 'c'},
			{"cn3", next, 1, lexer.EOF},
			{"eof0", next, 1, lexer.EOF},
			{"eof1", next, 1, lexer.EOF},
			{"eof2", next, 1, lexer.EOF},
			{"eofb", backup, 0, 0}, // unread EOF
			{"eofb", backup, 0, 0}, // here, Pos is garbage
			{"cn4", next, 0, 'c'},
		},
		{
			{"nl1", next, 0, '\n'},
			{"nl2", peek, 0, '\n'},
		},
	}

	for i, in := range input {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			lexer.New(token.NewFile("", strings.NewReader(in)), func(l *lexer.State) lexer.StateFn {
				for _, td := range data[i] {
					if td.name == "cb0" {
						t.Log("cb0")
					}
					r := td.fn(l)
					if r != td.r {
						t.Errorf("%s: expected %q, got %q", td.name, td.r, r)
					}
					if l.Pos() != td.p {
						t.Errorf("%s: expected pos %d, got %d", td.name, td.p, l.Pos())
					}
					l.Emit(token.EOF, nil)
					if t.Failed() {
						return nil
					}
				}
				return nil
			}).Lex()
		})
	}
}

func TestLexer_Lex(t *testing.T) {
	var stateEOF lexer.StateFn
	stateEOF = func(l *lexer.State) lexer.StateFn {
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

	stateNum := func(s *lexer.State) lexer.StateFn {
		num = 0
		base = 10
		r := s.Next()
		if r == '0' {
			r = s.Next()
			if r == 'x' || r == 'X' {
				base = 16
			} else {
				s.Backup()
				base = 8
			}
		} else {
			s.Backup()
		}
		s.AcceptWhile(scanDigit)
		s.Emit(0, int(num))
		if base == 8 {
			s.Errorf(s.Pos(), "piling up")
			s.Errorf(s.Pos(), "things")
		}
		return nil
	}
	//                                            01234567890123
	f := token.NewFile("test", strings.NewReader("0x24 12 0666 |"))
	data := []struct {
		t token.Type
		p token.Pos
		v interface{}
	}{
		{0, 0, 36},
		{0, 5, 12},
		{0, 8, 438},
		{token.Error, 11, "piling up"},
		{token.Error, 11, "things"},
		{token.EOF, 14, nil},
	}
	l := lexer.New(f,
		func(l *lexer.State) lexer.StateFn {
			r := l.Next()
			switch r {
			case lexer.EOF:
				return stateEOF
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				l.Backup()
				return stateNum
			case '|':
				// end marker
				if l.Peek() != lexer.EOF {
					panic("| not @EOF")
				}
			}
			return nil
		})
	for _, r := range data {
		tt, p, v := l.Lex()
		if tt != r.t || p != r.p || v != r.v {
			t.Errorf("Got: %d %d %v, expected: %d %d %v", tt, p, v, r.t, r.p, r.v)
		}
		if tt == token.EOF {
			break
		}
	}
}
