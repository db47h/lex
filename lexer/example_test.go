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
			return lexIdentifier(l)
		}

		// fallback
		l.Errorf(l.Pos(), "invalid character %#U", r)
		return nil
	})

	// EOF
	l.MatchRunes([]rune{lexer.EOF}, state.EOF)

	// Line comment
	l.Match("//", func(l *lexer.Lexer) lexer.StateFn {
		l.AcceptUpTo([]rune{'\n'})
		l.Backup() // don't put trailing \n in comment
		l.Emit(tokComment, l.TokenString())
		return nil
	})

	// C style comment
	l.Match("/*", func(l *lexer.Lexer) lexer.StateFn {
		pos := l.Pos()
		if ok := l.AcceptUpTo([]rune("*/")); !ok {
			l.Errorf(pos, "unterminated block comment")
			return nil
		}
		l.Emit(tokComment, l.TokenString())
		return nil
	})

	// Integers
	l.MatchAny("0123456789", state.Int(tokInt, 10))

	// strings
	l.Match("\"", state.QuotedString(tokString))

	// parens
	l.Match("(", func(l *lexer.Lexer) lexer.StateFn {
		// eat any space following '('
		l.AcceptWhile(unicode.IsSpace)
		l.Emit(tokLeftParen, nil)
		return nil
	})
	l.Match(")", state.EmitNil(tokRightParen))

	// brackets
	l.Match("[", state.EmitNil(tokLeftBracket))
	l.Match("]", state.EmitNil(tokRightBracket))

	// braces
	l.Match("{", func(l *lexer.Lexer) lexer.StateFn {
		// eat any space following '('
		l.AcceptWhile(unicode.IsSpace)
		l.Emit(tokLeftBrace, nil)
		return nil
	})
	l.Match("}", state.EmitNil(tokRightBrace))

	// assignment
	l.Match(":=", state.EmitNil(tokVar))

	// comma
	l.Match(",", state.EmitNil(tokComma))

	// dot. TODO: "." needs special handling along with numbers.
	l.Match(".", state.EmitNil(tokDot))

	// others
	l.Match("<", state.EmitNil(tokLSS))
	l.Match(">", state.EmitNil(tokGTR))
	l.Match("++", state.EmitNil(tokInc))

	// convert EOLs to ;
	l.MatchAny("\n;", func(l *lexer.Lexer) lexer.StateFn {
		l.AcceptWhile(unicode.IsSpace)       // eat all space, including additional EOLs
		l.Emit(tokEOL, string(l.Token()[0])) // use only the first rune as value
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
		l.Emit(tokIdent, s)
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

func BenchmarkLexGo(b *testing.B) {
	init := initFn()
	for n := 0; n < b.N; n++ {
		f := token.NewFile("input", strings.NewReader(input))
		l := lexer.New(f, init)
		for i := l.Lex(); i.Type != token.EOF; i = l.Lex() {
		}
	}
}
