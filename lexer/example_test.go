package lexer_test

import (
	"fmt"
	gscanner "go/scanner"
	gtoken "go/token"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/db47h/asm/lexer"
	"github.com/db47h/asm/lexer/state"
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
	if t, ok := keywords[s]; ok {
		l.Emit(t, s)
	} else {
		l.Emit(l.T, s)
	}
	return nil
}

const (
	tokComment token.Type = iota
	tokInt
	tokString
	tokLeftParen
	tokRightParen
	tokLeftBracket
	tokRightBracket
	tokLeftBrace
	tokRightBrace
	tokVar
	tokComma
	tokDot
	tokEOL
	tokIdent
	tokPackage
	tokImport
	tokFunc
	tokFor
)

var tokens = map[token.Type]string{
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
var keywords = map[string]token.Type{
	"package": tokPackage,
	"import":  tokImport,
	"func":    tokFunc,
	"for":     tokFor,
}
var lang *lexer.Lang

// init() is a convenient place to define our language. In this case, we want to lex Go code.
//
func init() {
	// map keyword names
	for k, v := range keywords {
		tokens[v] = k
	}

	// Define language

	lang = lexer.NewLang(func(l *lexer.Lexer) lexer.StateFn {
		// exact prefix matches and EOF have already been checked for us
		// we just need to skip spaces and lex identifiers.
		r := l.Next()

		if unicode.IsSpace(r) {
			l.AcceptWhile(func(r rune) bool { return r != '\n' && unicode.IsSpace(r) })
			l.Discard()
			return nil // return to initial state
		}

		// identifiers start with a letter or underscore
		if r == '_' || unicode.IsLetter(r) {
			l.T = tokIdent
			return lexIdentifier(l)
		}

		// fallback
		l.Emit(token.RawChar, r)
		return nil
	})

	// EOF
	lang.MatchRunes(token.EOF, []rune{lexer.EOF}, state.EOF)

	// Line comment
	lang.Match(tokComment, "//", func(l *lexer.Lexer) lexer.StateFn {
		l.AcceptUpTo([]rune{'\n'})
		l.Backup() // don't put trailing \n in comment
		l.Emit(l.T, l.TokenString())
		return nil
	})

	// C style comment
	lang.Match(tokComment, "/*", func(l *lexer.Lexer) lexer.StateFn {
		if ok := l.AcceptUpTo([]rune("*/")); !ok {
			l.Errorf("unterminated block comment")
			return nil
		}
		l.Emit(l.T, l.TokenString())
		return nil
	})

	// Integers
	lang.MatchAny(tokInt, []rune("0123456789"), state.Int)

	// strings
	lang.Match(tokString, "\"", state.String)

	// parens
	lang.Match(tokLeftParen, "(", func(l *lexer.Lexer) lexer.StateFn {
		// eat any space following '('
		l.AcceptWhile(unicode.IsSpace)
		l.Emit(l.T, nil)
		return nil
	})
	lang.Match(tokRightParen, ")", state.EmitNil)

	// brackets
	lang.Match(tokLeftBracket, "[", state.EmitNil)
	lang.Match(tokRightBracket, "]", state.EmitNil)

	// braces
	lang.Match(tokLeftBrace, "{", func(l *lexer.Lexer) lexer.StateFn {
		// eat any space following '('
		l.AcceptWhile(unicode.IsSpace)
		l.Emit(l.T, nil)
		return nil
	})
	lang.Match(tokRightBrace, "}", state.EmitNil)

	// assignment
	lang.Match(tokVar, ":=", state.EmitNil)

	// comma
	lang.Match(tokComma, ",", state.EmitNil)

	// dot
	lang.Match(tokDot, ".", state.EmitNil)

	// convert EOLs to ;
	lang.MatchAny(tokEOL, []rune{'\n', ';'}, func(l *lexer.Lexer) lexer.StateFn {
		l.AcceptWhile(unicode.IsSpace)    // eat all space, including additional EOLs
		l.Emit(l.T, string(l.Token()[0])) // use only the first rune as value
		return nil
	})

}

func goTokenString(i *lexer.Item) string {
	s := tokens[i.Type]
	if s == "" {
		s = fmt.Sprintf("%v", i.Type)
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
	l := lexer.New(f, lang)

	// t := time.Now()
	for i := l.Lex(); i.Type != token.EOF; i = l.Lex() {
		// fmt.Println(f.Position(i.Pos).String(), tokenString(&i))
	}
	// i := l.Lex()
	// fmt.Println(f.Position(i.Pos).String(), tokenString(&i))
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
		l := lexer.New(f, lang)
		for i := l.Lex(); i.Type != token.EOF; i = l.Lex() {
		}
	}
}
