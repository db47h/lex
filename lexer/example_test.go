package lexer_test

import (
	"fmt"
	gscanner "go/scanner"
	gtoken "go/token"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/db47h/asm/lexer"
	"github.com/db47h/asm/token"
)

var input = `// test data
package main

import(
  "fmt"
)

// main function
func main() {
  b := make([]int, 10) // zero-filled
  for i := 0; i < len(b); i++ {
	fmt.Println(b[i])
  }
}
`

// goScan scans input with the Go scanner
//
func goScan() {
	var gs gscanner.Scanner
	gfs := gtoken.NewFileSet()
	gf := gfs.AddFile("input", gfs.Base(), len(input))
	gs.Init(gf, []byte(input), nil, gscanner.ScanComments)
	for {
		p, t, l := gs.Scan()
		if t == gtoken.EOF {
			return
		}
		fmt.Print(gfs.Position(p).String(), " ", t.String(), " ")
		if l != "" {
			fmt.Print(strconv.Quote(l))
		}
		fmt.Println()
	}
}

// lexIdentifier scans identifiers then checks if the identifier is a keyword
// and emits the appropriate token value.
//
func lexIdentifier(l *lexer.Lexer) lexer.StateFn {
	l.AcceptWhile(func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' })
	// filter keywords
	s := l.TokenString()
	if i := sort.SearchStrings(keywords[:], s); i < len(keywords) && keywords[i] == s {
		return l.Emit(token.Custom+token.Token(i), s, lexer.StateAny)
	}
	return l.Emit(l.T, s, lexer.StateAny)
}

var lang lexer.Lang
var tokens map[token.Token]string
var keywords = [...]string{"package", "import", "func"}

// init() is a convenient place to define our language. In this case, we want to lex Go code.
//
func init() {
	// exact prefix matches

	// Line comment
	tokComment := lang.Match("//", func(l *lexer.Lexer) lexer.StateFn {
		l.AcceptUpTo([]rune{'\n'})
		l.Backup() // don't put trailing \n in comment
		return l.Emit(l.T, l.TokenString(), lexer.StateAny)
	})

	// C style comment
	lang.Match("/*", func(l *lexer.Lexer) lexer.StateFn {
		if ok := l.AcceptUpTo([]rune("*/")); !ok {
			if err := l.Errorf("unterminated block comment"); err != nil {
				return nil
			}
			return lexer.StateAny
		}
		return l.Emit(tokComment, l.TokenString(), lexer.StateAny) // use same token as //
	})

	// Integers
	tokInt := lang.MatchAny("0123456789", lexer.StateInt)

	// strings
	tokString := lang.Match("\"", lexer.StateString)

	// parens
	tokLeftParen := lang.Match("(", func(l *lexer.Lexer) lexer.StateFn {
		// eat any space following '('
		l.AcceptWhile(unicode.IsSpace)
		return l.Emit(l.T, nil, lexer.StateAny)
	})
	tokRightParen := lang.Match(")", lexer.StateEmitTokenNilValue)
	// brackets
	tokLeftBracket := lang.Match("[", lexer.StateEmitTokenNilValue)
	tokRightBracket := lang.Match("]", lexer.StateEmitTokenNilValue)
	// braces
	tokLeftBrace := lang.Match("{", func(l *lexer.Lexer) lexer.StateFn {
		// eat any space following '('
		l.AcceptWhile(unicode.IsSpace)
		return l.Emit(l.T, nil, lexer.StateAny)
	})
	tokRightBrace := lang.Match("}", lexer.StateEmitTokenNilValue)

	// assignment
	tokVar := lang.Match(":=", lexer.StateEmitTokenNilValue)

	// comma
	tokComma := lang.Match(",", lexer.StateEmitTokenNilValue)

	// dot
	tokDot := lang.Match(".", lexer.StateEmitTokenNilValue)

	// convert EOLs to ;
	tokEOL := lang.MatchAny("\n;", func(l *lexer.Lexer) lexer.StateFn {
		l.AcceptWhile(unicode.IsSpace)                     // eat all space, including additional EOLs
		return l.Emit(l.T, string(l.B[0]), lexer.StateAny) // use only the first rune as value
	})

	// boolean filters

	// ignore spaces
	lang.MatchFn(unicode.IsSpace, func(l *lexer.Lexer) lexer.StateFn {
		l.AcceptWhile(func(r rune) bool { return r != '\n' && unicode.IsSpace(r) })
		return lexer.StateAny
	})

	// identifiers start with a letter or underscore
	tokIdent := lang.MatchFn(
		func(r rune) bool { return r == '_' || unicode.IsLetter(r) },
		lexIdentifier)

	// map tokens names
	tokens = map[token.Token]string{
		tokComment:      "COMMENT",
		tokInt:          "INT",
		tokString:       "STRING",
		tokLeftParen:    "(",
		tokRightParen:   ")",
		tokLeftBracket:  "[",
		tokRightBracket: "]",
		tokLeftBrace:    "{",
		tokRightBrace:   "}",
		tokVar:          ":=",
		tokComma:        ",",
		tokDot:          ".",
		tokEOL:          ";",
		tokIdent:        "IDENT",
	}

	// keywords
	sort.Strings(keywords[:])
	for i := range keywords {
		tokens[token.Custom+token.Token(i)] = keywords[i]
	}
}

func tokenString(i *lexer.Item) string {
	s := tokens[i.Token]
	if s == "" {
		s = i.Token.String()
	}
	if s == "STRING" {
		// need to quote string to match Go's scanner behavior
		return fmt.Sprintf("%s %q", s, strconv.Quote(i.Value.(string)))
	}
	switch v := i.Value.(type) {
	case nil:
		return s
	case fmt.Stringer:
		return fmt.Sprintf("%s %s", s, v.String())
	default:
		return fmt.Sprintf("%s %q", s, i.Value)
	}
}

// Idiomatic usage
func ExampleLexer() {
	// goScan()
	fmt.Println()

	f := token.NewFile("input", strings.NewReader(input))
	l := lexer.New(f, &lang)

	// t := time.Now()
	for i := l.Lex(); i.Token != token.EOF; i = l.Lex() {
		// fmt.Println(f.Position(i.Pos).String(), tokenString(i))
	}
	// fmt.Printf("%v\n", time.Since(t))

	// Output:
}

func BenchmarkGo(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var gs gscanner.Scanner
		gfs := gtoken.NewFileSet()
		gf := gfs.AddFile("input", gfs.Base(), len(input))
		gs.Init(gf, []byte(input), nil, gscanner.ScanComments)
		for {
			_, t, _ := gs.Scan()
			if t == gtoken.EOF {
				break
			}
		}
	}
}

func BenchmarkLEx(b *testing.B) {
	for n := 0; n < b.N; n++ {
		f := token.NewFile("input", strings.NewReader(input))
		l := lexer.New(f, &lang)
		for i := l.Lex(); i.Token != token.EOF; i = l.Lex() {
		}
	}
}
