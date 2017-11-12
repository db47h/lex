package lexer_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/db47h/asm/lexer"
	"github.com/db47h/asm/lexer/lang"
	"github.com/db47h/asm/lexer/state"
	"github.com/db47h/asm/token"
)

// Sample input
//
var input = `// test data
package main

import(
  "fmt"
)

// main function
func main() {
  b := make([]int, 10) /* trailing */
  for i := 0; i < len(b /* inline */); i++ {
	fmt.Println(b[i])
  }
}
`

// Custom token types
//
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
	tokLSS
	tokGTR
	tokInc
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
	tokLSS:          "<",
	tokGTR:          ">",
	tokInc:          "++",
}
var keywords = map[string]token.Type{
	"package": tokPackage,
	"import":  tokImport,
	"func":    tokFunc,
	"for":     tokFor,
}

func initFn() lexer.StateFn {
	// map keyword names
	for k, v := range keywords {
		tokens[v] = k
	}

	// Define language

	l := lang.New(func(l *lexer.Lexer) lexer.StateFn {
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
		l.Errorf("invalid character %#U", r)
		return nil
	})

	// EOF
	l.MatchRunes(token.EOF, []rune{lexer.EOF}, state.EOF)

	// Line comment
	l.Match(tokComment, "//", func(l *lexer.Lexer) lexer.StateFn {
		l.AcceptUpTo([]rune{'\n'})
		l.Backup() // don't put trailing \n in comment
		l.Emit(l.T, l.TokenString())
		return nil
	})

	// C style comment
	l.Match(tokComment, "/*", func(l *lexer.Lexer) lexer.StateFn {
		if ok := l.AcceptUpTo([]rune("*/")); !ok {
			l.Errorf("unterminated block comment")
			return nil
		}
		l.Emit(l.T, l.TokenString())
		return nil
	})

	// Integers
	l.MatchAny(tokInt, []rune("0123456789"), state.Int(10))

	// strings
	l.Match(tokString, "\"", state.QuotedString)

	// parens
	l.Match(tokLeftParen, "(", func(l *lexer.Lexer) lexer.StateFn {
		// eat any space following '('
		l.AcceptWhile(unicode.IsSpace)
		l.Emit(l.T, nil)
		return nil
	})
	l.Match(tokRightParen, ")", state.EmitNil)

	// brackets
	l.Match(tokLeftBracket, "[", state.EmitNil)
	l.Match(tokRightBracket, "]", state.EmitNil)

	// braces
	l.Match(tokLeftBrace, "{", func(l *lexer.Lexer) lexer.StateFn {
		// eat any space following '('
		l.AcceptWhile(unicode.IsSpace)
		l.Emit(l.T, nil)
		return nil
	})
	l.Match(tokRightBrace, "}", state.EmitNil)

	// assignment
	l.Match(tokVar, ":=", state.EmitNil)

	// comma
	l.Match(tokComma, ",", state.EmitNil)

	// dot. TODO: "." needs special handling along with numbers.
	l.Match(tokDot, ".", state.EmitNil)

	// others
	l.Match(tokLSS, "<", state.EmitNil)
	l.Match(tokGTR, ">", state.EmitNil)
	l.Match(tokInc, "++", state.EmitNil)

	// convert EOLs to ;
	l.MatchAny(tokEOL, []rune{'\n', ';'}, func(l *lexer.Lexer) lexer.StateFn {
		l.AcceptWhile(unicode.IsSpace)    // eat all space, including additional EOLs
		l.Emit(l.T, string(l.Token()[0])) // use only the first rune as value
		return nil
	})

	return l.Init()
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
func Example_go_lexer() {
	fmt.Println()

	f := token.NewFile("input", strings.NewReader(input))
	l := lexer.New(f, initFn())

	// t := time.Now()
	for i := l.Lex(); i.Type != token.EOF; i = l.Lex() {
		fmt.Println(f.Position(i.Pos).String(), goTokenString(&i))
	}
}

func BenchmarkLEx(b *testing.B) {
	init := initFn()
	for n := 0; n < b.N; n++ {
		f := token.NewFile("input", strings.NewReader(input))
		l := lexer.New(f, init)
		for i := l.Lex(); i.Type != token.EOF; i = l.Lex() {
		}
	}
}
