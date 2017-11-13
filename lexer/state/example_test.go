package state_test

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/db47h/asm/lexer"
	"github.com/db47h/asm/lexer/state"
	"github.com/db47h/asm/token"
)

// TinyGo returns a lexer Interface for a simple go-like language.
//
func TinyGo(file *token.File) lexer.Interface {
	return lexer.New(file, tgInit())
}

// tokens
const (
	goSemiColon  token.Type = iota // 0 semi-colon, EOL
	goInt                          // 1 integer literal
	goFloat                        // 2 float literal
	goString                       // 3 quoted string
	goChar                         // 4 quoted char
	goIdentifier                   // 5 identifier
	goDot                          // 6 '.' field/method selector
	goRawChar                      // 7 any other single character
)

// tgInit returns the initial state function for our language.
// We implement it as a closure so that we can initialize state functions from
// the state package and take advantage of buffer pre-allocation.
//
func tgInit() lexer.StateFn {
	quotedString := state.QuotedString(tokString)
	quotedChar := state.QuotedString(tokString)
	bin := state.Int(goInt, 2)
	hex := state.Int(goInt, 2)
	number := state.Number(goInt, goFloat, '.', true)
	ident := identifier()

	return func(l *lexer.Lexer) lexer.StateFn {
		r := l.Next()
		switch r {
		case lexer.EOF:
			// End of file. Must be handled manually.
			return state.EOF
		case '\n', ';':
			// transform EOLs to semi-colons
			l.Emit(goSemiColon, nil)
			return nil
		case '"':
			return quotedString
		case '\'':
			return quotedChar
		// numbers are trickier. We have the leading 0 case:
		case '0':
			r = l.Next()
			switch r {
			case 'x', 'X':
				l.Next() // read first digit for Int
				return hex
			case 'b', 'B':
				l.Next() // read first digit for Int
				return bin
			default:
				// At this point, the input could be an octal integer or a float with a leading 0.
				// Also, l.Last() would return the character following the leading 0, so we un-read
				// it to that state.Number() can see that leading 0 and emit an octal if this is an
				// integer literal.
				l.Backup()
				return number
			}
		case '1', '2', '3', '4', '5', '6', '7', '8', '9':
			// let state.Number decide
			return number
		case '.':
			// we want to distinguish a float starting with a leading dot from a dot used as
			// a field/method selector between two identifiers.
			if r = l.Peek(); r >= '0' && r <= '9' {
				// dot followed by a digit => floating point number
				return number
			}
			// for a dot followed by any other char, we emit it as-is
			l.Emit(goDot, nil)
		}

		// we're left with identifiers, spaces and raw chars.
		switch {
		case unicode.IsSpace(r):
			// ignore spaces
			l.AcceptWhile(unicode.IsSpace)
			return nil // this will discard the current token
		case unicode.IsLetter(r) || r == '_':
			// r starts an identifier
			return ident
		default:
			l.Emit(goRawChar, r)
			return nil
		}
	}
}

func identifier() lexer.StateFn {
	// preallocate a buffer. It will end-up being at least as large as the largest
	// identifier scanned.
	b := make([]rune, 0, 64)
	return func(l *lexer.Lexer) lexer.StateFn {
		// reset buffer and add first char
		b = append(b[:0], l.Last())
		// read identifier
		for r := l.Next(); unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'; r = l.Next() {
			b = append(b, r)
		}
		// the character returned by the last call to next is not part of the identifier. Undo it.
		l.Backup()
		l.Emit(goIdentifier, string(b))
		return nil
	}
}

// Demonstrate our simple go lexer.
//
func Example_go() {
	// initialize lexer.
	//
	l := TinyGo(token.NewFile("example", strings.NewReader(
		`var str = "some string"
var flt = 1.275
`)))

	for item := l.Lex(); item.Type != token.EOF; item = l.Lex() {
		switch v := item.Value.(type) {
		case nil:
			fmt.Println(item.Type)
		case string:
			fmt.Println(item.Type, strconv.Quote(v))
		case rune:
			fmt.Println(item.Type, strconv.QuoteRune(v))
		default:
			fmt.Println(item.Type, item.Value)
		}
	}

	// Output:
	// Type(5) "var"
	// Type(5) "str"
	// Type(7) '='
	// Type(2) "some string"
	// Type(0)
	// Type(5) "var"
	// Type(5) "flt"
	// Type(7) '='
	// Type(2) 1.275
	// Type(0)
}
