package lexer_test

import (
	"fmt"

	"github.com/db47h/asm/lexer"
	"github.com/db47h/asm/token"
)

// Idiomatic usage
func ExampleLexer() {
	input := `
.org 0x200
	; strcmp compares strings in a0 and a1
strcmp:
0:
	lbu t0, 0+0(a0)
	lbu t1, 0-0(a1)
	bne t0, t1, 0f
	beq t0, zero, 1f
	addi a0, a0, 1
	addi a1, a1, 1
	jal zero, 0b
0:  ; not equal 
	addi a1, zero, -1
	jalr zero, ra, 0
1:  ; equal
	addi a1, zero, 0
	jalr zero, ra, 0
`

	l := lexer.New([]byte(input))
	defer l.Close()

	for {
		lx := l.Lex()
		// eat spaces
		if lx.Token == token.Space {
			continue
		}
		fmt.Println(lx.String())
		if lx.Token == token.EOF {
			break
		}
	}
}
