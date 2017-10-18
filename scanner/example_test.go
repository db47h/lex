package scanner_test

import (
	"fmt"

	"github.com/db47h/asm/scanner"
	"github.com/db47h/asm/token"
)

// Idiomatic usage
func ExampleScanner() {
	input := `
.org 0x200
	; strcmp compares strings in a0 and a1
strcmp:
	lbu t0, 0+0(a0)
	lbu t1, 0-0(a1)
	bne t0, t1, 0f
	beq t0, zero, 1f
	addi a0, a0, 1
	addi a1, a1, 1
	jal zero, strcmp
0:  ; not equal 
	addi a1, zero, -1
	jalr zero, ra, 0
1:  ; equal
	addi a1, zero, 0
	jalr zero, ra, 0
`

	var s scanner.Scanner

	s.Init([]byte(input))
	defer s.Close()

	for {
		t := s.Scan()
		fmt.Println(t.String())
		if t.Token == token.EOF {
			break
		}
	}
	// Output:
	// This example delibeartely fails until all necessary
	// features are included
}
