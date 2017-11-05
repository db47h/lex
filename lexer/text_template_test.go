package lexer_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/db47h/asm/lexer/state"

	"github.com/db47h/asm/lexer"
	"github.com/db47h/asm/token"
)

// constants copied from $GOROOT/src/text/template/parse/lex.go
// we don't use itemError or itemEOF since these are already
// provided in the token package.
//
const (
	itemError        token.Type = iota // error occurred; value is text of error
	itemBool                           // boolean constant
	itemChar                           // printable ASCII character; grab bag for comma etc.
	itemCharConstant                   // character constant
	itemComplex                        // complex constant (1+2i); imaginary is just a number
	itemColonEquals                    // colon-equals (':=') introducing a declaration
	itemEOF
	itemField      // alphanumeric identifier starting with '.'
	itemIdentifier // alphanumeric identifier not starting with '.'
	itemLeftDelim  // left action delimiter
	itemLeftParen  // '(' inside action
	itemNumber     // simple number, including imaginary
	itemPipe       // pipe symbol
	itemRawString  // raw quoted string (includes quotes)
	itemRightDelim // right action delimiter
	itemRightParen // ')' inside action
	itemSpace      // run of spaces separating arguments
	itemString     // quoted string (includes quotes)
	itemText       // plain text
	itemVariable   // variable starting with '$', such as '$' or  '$1' or '$hello'
	// Keywords appear after all the rest.
	itemKeyword  // used only to delimit the keywords
	itemBlock    // block keyword
	itemDot      // the cursor, spelled '.'
	itemDefine   // define keyword
	itemElse     // else keyword
	itemEnd      // end keyword
	itemIf       // if keyword
	itemNil      // the untyped nil constant, easiest to treat as a keyword
	itemRange    // range keyword
	itemTemplate // template keyword
	itemWith     // with keyword
)

var key = map[string]token.Type{
	".":        itemDot,
	"block":    itemBlock,
	"define":   itemDefine,
	"else":     itemElse,
	"end":      itemEnd,
	"if":       itemIf,
	"range":    itemRange,
	"nil":      itemNil,
	"template": itemTemplate,
	"with":     itemWith,
}

// isAlphaNumeric is a helper function that checks whether r represents an alphanumeric character or '_'.
//
func isAlphaNumeric(r rune) bool {
	return r != lexer.EOF && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_')
}

// tmplState encapsulates additional state for our custom lexer.
// Here we need to check that parenthesis are properly balanced.
// This is usually done in the parser, but this gives us an oportunity to show
// how to implement additional state.
//
type tmplState struct {
	parenDepth int
	langAction *lexer.Lang
	langText   *lexer.Lang
}

// initText returns the StateFn for the "text" initial state.
//
func (s *tmplState) initText() lexer.StateFn {
	return func(l *lexer.Lexer) lexer.StateFn {
		// just read everything up to delim
		if ok := l.AcceptUpTo([]rune("{{")); !ok {
			// end of file
			l.Emit(itemText, l.TokenString())
			return nil
		}
		t := string(l.Token()[:l.TokenLen()-2])
		// check trim
		if l.Accept([]rune("- ")) {
			t = strings.TrimRight(t, " \t\r\n")
		}
		if len(t) > 0 {
			l.Emit(itemText, t)
		} else {
			l.Discard()
		}
		// check for comments
		if l.Accept([]rune("/*")) {
			return s.comment()
		}
		l.Emit(itemLeftDelim, "{{") // we could add the trim, but the Go version doesn't
		// switch languages
		l.L = s.langAction
		s.parenDepth = 0
		// this will switch the lexer's state to the langAction's initial state
		return nil
	}
}

// comment returns the StateFn to handle comments.
//
func (s *tmplState) comment() lexer.StateFn {
	return func(l *lexer.Lexer) lexer.StateFn {
		// The '/*' has already been seen.
		// Discard everything until */ and expect " -}}" or "}}" following it.
		if !l.AcceptUpTo([]rune("*/")) {
			l.Errorf("unclosed comment")
			return nil
		}
		l.Discard()
		switch {
		case l.Accept([]rune(" -}}")):
			l.AcceptWhile(unicode.IsSpace)
			fallthrough
		case l.Accept([]rune("}}")):
			l.Discard()
		default:
			l.Errorf("comment ends before closing delimiter")
		}
		return nil
	}
}

// rightDelim returns the StateFn to handle right delimiter (in langAction).
//
func (s *tmplState) rightDelim(trim bool) lexer.StateFn {
	return func(l *lexer.Lexer) lexer.StateFn {
		if s.parenDepth != 0 {
			l.Errorf("unclosed left paren")
		} else {
			if trim {
				l.AcceptWhile(unicode.IsSpace)
			}
			l.Emit(itemRightDelim, "}}")
		}
		l.L = s.langText
		return nil
	}
}

// leftParen is a metch function for '('
//
func (s *tmplState) leftParen() lexer.StateFn {
	return func(l *lexer.Lexer) lexer.StateFn {
		s.parenDepth++
		return state.EmitTokenString
	}
}

// rightParen is a metch function for ')'
//
func (s *tmplState) rightParen() lexer.StateFn {
	return func(l *lexer.Lexer) lexer.StateFn {
		s.parenDepth--
		return state.EmitTokenString
	}
}

// initTmplLang creates a new tmplState, builds the language states
// and returns a pointer to s.langText.
//
func initTmplLang() *lexer.Lang {
	s := tmplState{}

	// "text" language: an initial state function that looks for '{{',
	// optionally trims preceding space, then switches to the "action" language.
	s.langText = lexer.NewLang(s.initText())
	// check for EOF. This is the trivial case.
	s.langText.MatchRunes(token.EOF, []rune{lexer.EOF}, state.EOF)

	s.langAction = lexer.NewLang(func(l *lexer.Lexer) lexer.StateFn {
		r := l.Next()

		// This switch checks only the more complex cases not already handled
		// by simple matches registered with the MatchXXX() functions.
		switch {
		case unicode.IsLetter(r) || r == '_':
			// identifiers, keywords, true/false
			l.AcceptWhile(isAlphaNumeric)
			s := l.TokenString()
			t := key[s]
			switch {
			case t > itemKeyword:
				l.Emit(t, s)
			case s == "true" || s == "false":
				l.Emit(itemBool, s)
			default:
				l.Emit(itemIdentifier, s)
			}
			return nil
		case unicode.IsSpace(r):
			l.AcceptWhile(unicode.IsSpace)
			l.Emit(itemSpace, ' ')
			return nil
		}

		l.Emit(itemChar, string(r))
		return nil
	})

	// check EOF/EOL
	s.langAction.MatchAny(token.EOF, []rune{lexer.EOF, '\n'}, func(l *lexer.Lexer) lexer.StateFn {
		r := l.Token()[0]
		l.Errorf("unclosed action")
		if r == '\n' {
			return nil // keep going
		}
		return state.EOF
	})

	// end of action  switch back to plain text lexing.
	// register two versions: with and without trim marker.
	s.langAction.Match(token.Invalid, " -}}", s.rightDelim(true))
	s.langAction.Match(token.Invalid, "}}", s.rightDelim(false))

	// match field or variable
	s.langAction.MatchAny(itemDot, []rune("$."), func(l *lexer.Lexer) lexer.StateFn {
		if l.Token()[0] == '$' {
			l.T = itemVariable
		}
		r := l.Next()
		if unicode.IsLetter(r) || r == '_' {
			l.T = itemVariable
			l.AcceptWhile(isAlphaNumeric)
			l.Emit(l.T, l.TokenString())
			return nil
		}
		l.Backup()
		l.Emit(l.T, l.TokenString())
		return nil
	})

	// strings
	s.langAction.Match(itemString, "\"", state.String)

	// left/right paren
	s.langAction.Match(itemLeftParen, "(", s.leftParen())
	s.langAction.Match(itemRightParen, ")", s.rightParen())

	s.langAction.Match(itemColonEquals, ":=", state.EmitTokenString)

	// numbers
	s.langAction.MatchAny(itemNumber, []rune("0123456789"), state.Int)

	// pipe
	s.langAction.Match(itemPipe, "|", state.EmitTokenString)

	return s.langText
}

func Test_text_template(t *testing.T) {
	f, err := os.Open("testdata/slides.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	l := lexer.New(token.NewFile("slides.tmpl", bufio.NewReader(f)), initTmplLang())
	for {
		i := l.Lex()
		if i.Type == token.EOF {
			return
		}
		fmt.Fprintf(os.Stderr, "%d:%d %q\n", l.File().Position(i.Pos).Line, i.Pos, i.Value)
	}
}

func BenchmarkLexer(b *testing.B) {
	buf, err := ioutil.ReadFile("testdata/slides.tmpl")
	if err != nil {
		b.Fatal(err)
	}

	lang := initTmplLang()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := lexer.New(token.NewFile("test", bytes.NewReader(buf)), lang)
		for it := l.Lex(); it.Type != token.EOF; it = l.Lex() {

		}
	}
}
