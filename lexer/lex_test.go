package lexer_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

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
	next := func(s *lexer.State) rune { return s.Next() }
	peek := func(s *lexer.State) rune { return s.Peek() }
	backup := func(s *lexer.State) rune { s.Backup(); return s.Current() }
	cur := func(s *lexer.State) rune { return s.Current() }

	input := []string{
		"ab",
		"c",
		"\n\n",
		"abcdefghi",
	}

	data := [][]struct {
		name string
		fn   func(l *lexer.State) rune
		p    token.Pos
		r    rune
	}{
		{
			{"ab", cur, -1, utf8.RuneSelf},
			{"an", next, 0, 'a'},
			{"bn1", next, 1, 'b'},
			{"bb", backup, 0, 'a'},
			{"bp1", peek, 0, 'b'},
			{"bn2", next, 1, 'b'},
			{"bn3", next, 2, lexer.EOF},
			{"bb1", backup, 1, 'b'},
			{"bp2", peek, 1, lexer.EOF},
			{"eof1", next, 2, lexer.EOF},
			{"eofb", backup, 1, 'b'},
			{"eof2", next, 2, lexer.EOF},
			{"eof3", next, 2, lexer.EOF},
			{"eof4", next, 2, lexer.EOF},
			{"eofb2", backup, 1, 'b'},
			{"eofp2", peek, 1, lexer.EOF},
		},
		{
			{"cb0", cur, -1, utf8.RuneSelf},
			{"cn0", next, 0, 'c'},
			{"cb1", backup, -1, utf8.RuneSelf},
			{"cn1", next, 0, 'c'},
			{"cb2", backup, -1, utf8.RuneSelf},
			{"cn2", next, 0, 'c'},
			{"cn3", next, 1, lexer.EOF},
			{"eof0", next, 1, lexer.EOF},
			{"eof1", next, 1, lexer.EOF},
			{"eof2", next, 1, lexer.EOF},
			{"eofb", backup, 0, 'c'},
			{"eofb", backup, -1, utf8.RuneSelf},
			{"cn4", next, 0, 'c'},
		},
		{
			{"nlb", cur, -1, utf8.RuneSelf},
			{"nl1", next, 0, '\n'},
			{"nl2", peek, 0, '\n'},
		},
		{
			{"ob", cur, -1, utf8.RuneSelf},
			{"on0", next, 0, 'a'},
			{"on1", next, 1, 'b'},
			{"on2", next, 2, 'c'},
			{"on3", next, 3, 'd'},
			{"on4", next, 4, 'e'},
			{"on5", next, 5, 'f'},
			{"on6", next, 6, 'g'},
			{"on7", next, 7, 'h'},
			{"on8", next, 8, 'i'},
			{"on9", next, 9, lexer.EOF},
			{"ob0", backup, 8, 'i'},
			{"ob1", backup, 7, 'h'},
			{"ob2", backup, 6, 'g'},
			{"ob3", backup, 5, 'f'},
			{"ob4", backup, 4, 'e'},
			{"ob5", backup, 3, 'd'},
			{"ob6", backup, -1, utf8.RuneSelf},
		},
	}

	for i, in := range input {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			lexer.New(token.NewFile("", strings.NewReader(in)), func(s *lexer.State) lexer.StateFn {
				for _, td := range data[i] {
					r := td.fn(s)
					if r != td.r {
						t.Errorf("%s: expected %q, got %q", td.name, td.r, r)
					}
					if s.Pos() != td.p {
						t.Errorf("%s: expected pos %d, got %d", td.name, td.p, s.Pos())
					}
					s.Emit(s.Pos(), token.EOF, nil)
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
		r := s.Current()
		if r == '0' {
			r = s.Next()
			if r == 'x' || r == 'X' {
				s.Next()
				base = 16
			} else {
				base = 8
			}
		}
		for r := s.Current(); scanDigit(r); r = s.Next() {
		}
		s.Emit(s.TokenPos(), 0, int(num))
		if base == 8 {
			s.Errorf(s.Pos(), "test queue")
			s.Errorf(s.Pos(), "twice")
		}
		s.Backup()
		return nil
	}
	//                                            0123456789012
	f := token.NewFile("test", strings.NewReader("0x24 12 0666"))
	data := []struct {
		t token.Type
		p token.Pos
		v interface{}
	}{
		{0, 0, 36},
		{0, 5, 12},
		{0, 8, 438},
		{token.Error, 12, "test queue"},
		{token.Error, 12, "twice"},
		{token.EOF, 12, nil},
	}
	l := lexer.New(f,
		func(s *lexer.State) lexer.StateFn {
			r := s.Next()
			s.StartToken(s.Pos())
			switch r {
			case lexer.EOF:
				return lexer.StateEOF
			case ' ', '\t':
				// ignore spaces
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				return stateNum
			default:
				s.Errorf(s.Pos(), "invalid character %#U", r)
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
