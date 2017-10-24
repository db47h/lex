package parser_test

import (
	"testing"

	"github.com/db47h/asm/parser"
	"github.com/db47h/asm/scanner"
)

func TestNewParser(t *testing.T) {
	var s scanner.Scanner
	exp := "--3(a2)(a1)"
	s.Init([]byte(exp))
	defer s.Close()
	p := parser.NewParser("<stdin>", &s)
	n, err := p.ParseExpr()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(n.String())
}
