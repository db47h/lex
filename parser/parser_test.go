package parser_test

import (
	"testing"

	"github.com/db47h/asm/lexer"
	"github.com/db47h/asm/parser"
)

func TestNewParser(t *testing.T) {
	exp := "three(5)(7), 12+4, 17+4+3+9"
	l := lexer.New([]byte(exp))
	defer l.Close()
	p := parser.NewParser("<stdin>", l)
	n, err := p.ParseExpr()
	if err != nil {
		t.Error(err)
	}
	if n != nil {
		t.Log(n.String())
	}
}
