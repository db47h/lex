package state_test

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/db47h/parsekit/lexer"
	"github.com/db47h/parsekit/lexer/state"
	"github.com/db47h/parsekit/token"
)

// Token types.
//
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

var tokNames = map[token.Type]string{
	token.EOF:    "EOF      ",
	token.Error:  "error:   ",
	goSemiColon:  "semicolon",
	goInt:        "integer  ",
	goFloat:      "float    ",
	goString:     "string   ",
	goChar:       "char     ",
	goIdentifier: "ident    ",
	goDot:        "dot      ",
	goRawChar:    "raw char ",
}

// tgInit returns the initial state function for our language.
// We implement it as a closure so that we can initialize state functions from
// the state package and take advantage of buffer pre-allocation.
//
func tgInit() lexer.StateFn {
	// Note that because of the buffer pre-allocation mentioned above, reusing
	// any of these variables in multiple goroutines is not safe. i.e. do not
	// turn these into global variables.
	// Instead, call tgInit() to get a new initial state function for each lexer
	// running in a goroutine.
	quotedString := state.QuotedString(goString)
	quotedChar := state.QuotedChar(goChar)
	ident := identifier()
	number := state.Number(goInt, goFloat, '.')

	return func(l *lexer.State) lexer.StateFn {
		// get current rune (read for us by the lexer upon entering the initial state)
		r := l.Next()
		pos := l.Pos()
		// THE big switch
		switch r {
		case lexer.EOF:
			// End of file
			return lexer.StateEOF
		case '\n', ';':
			// transform EOLs to semi-colons
			l.Emit(pos, goSemiColon, ';')
			return nil
		case '"':
			return quotedString
		case '\'':
			return quotedChar
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return number
		case '.':
			// we want to distinguish a float starting with a leading dot from a dot used as
			// a field/method selector between two identifiers.
			if r = l.Peek(); r >= '0' && r <= '9' {
				// dot followed by a digit => floating point number
				return number
			}
			// for a dot followed by any other char, we emit it as-is
			l.Emit(pos, goDot, nil)
		}

		// we're left with identifiers, spaces and raw chars.
		switch {
		case unicode.IsSpace(r):
			// eat spaces
			for r = l.Next(); unicode.IsSpace(r); r = l.Next() {
			}
			l.Backup()
			return nil
		case unicode.IsLetter(r) || r == '_':
			// r starts an identifier
			return ident
		default:
			l.Emit(pos, goRawChar, r)
			return nil
		}
	}
}

func identifier() lexer.StateFn {
	// preallocate a buffer to store the identifier. It will end-up being at
	// least as large as the largest identifier scanned.
	b := make([]rune, 0, 64)
	return func(l *lexer.State) lexer.StateFn {
		pos := l.Pos()
		// reset buffer and add first char
		b = append(b[:0], l.Current())
		// read identifier
		for r := l.Next(); unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'; r = l.Next() {
			b = append(b, r)
		}
		// the character returned by the last call to next is not part of the identifier. Undo it.
		l.Backup()
		l.Emit(pos, goIdentifier, string(b))
		return nil
	}
}

// TinyGo: a lexer for a minimal Go-like language.
//
func Example_go() {
	input := `var str = "some\tstring"
	var flt = -.42`

	// initialize lexer.
	//
	inputFile := token.NewFile("example", strings.NewReader(input))
	l := lexer.New(inputFile, tgInit())

	// loop over each token
	for tt, _, v := l.Lex(); tt != token.EOF; tt, _, v = l.Lex() {
		// print the token type and value.
		switch v := v.(type) {
		case nil:
			fmt.Println(tokNames[tt])
		case string:
			fmt.Println(tokNames[tt], strconv.Quote(v))
		case rune:
			fmt.Println(tokNames[tt], strconv.QuoteRune(v))
		default:
			fmt.Println(tokNames[tt], v)
		}
	}

	// Output:
	// ident     "var"
	// ident     "str"
	// raw char  '='
	// string    "some\tstring"
	// semicolon ';'
	// ident     "var"
	// ident     "flt"
	// raw char  '='
	// raw char  '-'
	// float     0.42
}
