package parser_test

import (
	"strings"
	"testing"

	"github.com/db47h/asm/parser"
	"github.com/db47h/asm/token"
)

func TestNewParser(t *testing.T) {
	exp := "%lambda"
	f := token.NewFile("<stdin>", strings.NewReader(exp))
	p := parser.NewParser(f)
	n, err := p.ParseExpr()
	if err != nil {
		t.Error(err)
	}
	if n != nil {
		t.Log(n.String())
	}
}
