package state_test

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/db47h/asm/lexer"
	"github.com/db47h/asm/lexer/lang"
	"github.com/db47h/asm/lexer/state"
	"github.com/db47h/asm/token"
)

// tokens
const (
	tNumber token.Type = iota
	tDot
	tChar
)

// Define a simple language that handles constant numbers like Go and dots.
//
// One of the problems with numbers is that if we need to handle signle dots as
// tokens, we need to distinguish them from floats.
func initState() lexer.StateFn {
	l := lang.New(func(l *lexer.Lexer) lexer.StateFn {
		r := l.Next()
		switch {
		case unicode.IsSpace(r):
			// eat spaces
			l.AcceptWhile(unicode.IsSpace)
			l.Discard()
			return nil
		case r == lexer.EOF:
			return state.EOF
		default:
			// emit everything else as simple chars
			l.Emit(tChar, l.TokenString())
			return nil
		}
	})

	// // check EOF
	l.MatchRunes([]rune{lexer.EOF}, state.EOF)

	// // Numbers
	// // leading dot case
	l.Match(".", state.IfMatchAny("0123456789", state.Number(tNumber, '.', true), state.EmitString(tDot)))
	// // numbers starting with a digit are more straightforward
	l.MatchAny("0123456789", state.Number(tNumber, '.', true))
	// // hex numbers
	hexNum := state.IfMatchAny("0123456789abcdefABCDEF", state.Int(tNumber, 16), state.Errorf("malformed hex number"))
	l.Match("0x", hexNum)
	l.Match("0X", hexNum)
	// // bonus: binary numbers
	binNum := state.IfMatchAny("01", state.Int(tNumber, 2), state.Errorf("malformed binary number"))
	l.Match("0b", binNum)
	l.Match("0B", binNum)

	return l.Init()
}

func ExampleNumber() {
	input := "1.27 .e 0x13 3.10e4"
	l := lexer.New(token.NewFile("test", strings.NewReader(input)), initState())
	for it := l.Lex(); it.Type != token.EOF; it = l.Lex() {
		fmt.Printf("%d:%v\n", it.Type, it.Value)
	}
	// Output:
	// 0:1.27
	// 1:.
	// 2:e
	// 0:19
	// 0:31000
}
