package lex_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/db47h/lex"
)

type testData struct {
	name  string
	input string
	res   res
}

type res []string

const (
	tokEOF lex.Token = iota
	tokSpace
	tokChar
)

func tString(t lex.Token, p int, v interface{}) string {
	switch t {
	case tokEOF:
		return "EOF"
	case lex.Error:
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
	next := func(s *lex.State) rune { return s.Next() }
	peek := func(s *lex.State) rune { return s.Peek() }
	backup := func(s *lex.State) rune { s.Backup(); return s.Current() }
	cur := func(s *lex.State) rune { return s.Current() }

	input := []string{
		"aéb",
		"c",
		"\n\n",
	}

	data := [][]struct {
		name string
		fn   func(l *lex.State) rune
		p    int
		r    rune
	}{
		{
			{"aéb", cur, -1, utf8.RuneSelf},
			{"an", next, 0, 'a'},
			{"én1", next, 1, 'é'},
			{"bn1", next, 3, 'b'},
			{"_b", backup, 1, 'é'},
			{"bp1", peek, 1, 'b'},
			{"bn2", next, 3, 'b'},
			{"bn3", next, 4, lex.EOF},
			{"bb1", backup, 3, 'b'},
			{"bp2", peek, 3, lex.EOF},
			{"eof1", next, 4, lex.EOF},
			{"eofb", backup, 3, 'b'},
			{"eof2", next, 4, lex.EOF},
			{"eofp1", peek, 4, lex.EOF},
			{"eof4", next, 4, lex.EOF},
			{"eofb2", backup, 3, 'b'},
			{"eofp2", peek, 3, lex.EOF},
		},
		{
			{"cb0", cur, -1, utf8.RuneSelf},
			{"cn0", next, 0, 'c'},
			{"cb1", backup, -1, utf8.RuneSelf},
			{"cn1", next, 0, 'c'},
			{"cb2", backup, -1, utf8.RuneSelf},
			{"cn2", next, 0, 'c'},
			{"cn3", next, 1, lex.EOF},
			{"eof0", next, 1, lex.EOF},
			{"eof1", next, 1, lex.EOF},
			{"eof2", next, 1, lex.EOF},
			{"eofb", backup, 0, 'c'},
			{"eofb", backup, -1, utf8.RuneSelf},
			{"cn4", next, 0, 'c'},
		},
		{
			{"nlb", cur, -1, utf8.RuneSelf},
			{"nl1", next, 0, '\n'},
			{"nl2", peek, 0, '\n'},
		},
	}

	for i, in := range input {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			lex.NewLexer(lex.NewFile("", strings.NewReader(in)), func(s *lex.State) lex.StateFn {
				for _, td := range data[i] {
					r := td.fn(s)
					if r != td.r {
						t.Errorf("%s: expected %q, got %q", td.name, td.r, r)
					}
					if s.Pos() != td.p {
						t.Errorf("%s: expected pos %d, got %d", td.name, td.p, s.Pos())
					}
					s.Emit(s.Pos(), tokEOF, nil)
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

	stateNum := func(s *lex.State) lex.StateFn {
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
	f := lex.NewFile("test", strings.NewReader("0x24 12 0666"))
	data := []struct {
		t lex.Token
		p int
		v interface{}
	}{
		{0, 0, 36},
		{0, 5, 12},
		{0, 8, 438},
		{lex.Error, 12, "test queue"},
		{lex.Error, 12, "twice"},
		{tokEOF, 12, nil},
	}
	l := lex.NewLexer(f,
		func(s *lex.State) lex.StateFn {
			r := s.Next()
			s.StartToken(s.Pos())
			switch r {
			case lex.EOF:
				s.Emit(s.Pos(), tokEOF, nil)
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
		if tt == tokEOF {
			break
		}
	}
}
