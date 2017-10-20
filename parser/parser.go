package parser

import (
	"fmt"

	"github.com/db47h/asm/scanner"
	"github.com/db47h/asm/token"
)

type Node struct {
	t *scanner.Token
	l *Node
	r *Node
}

func (n *Node) String() string {
	if n.l == nil && n.r == nil {
		return string(n.t.Raw)
	}
	s := "("
	if n.r != nil {
		// binary
		s += n.l.String() + string(n.t.Raw) + n.r.String()
	} else {
		// unary
		s += string(n.t.Raw) + n.l.String()
	}
	return s + ")"
}

type prec struct {
	prec int
	ltr  bool
}

var precBin = map[token.Token]prec{
	token.OpOr:     {0, true},
	token.OpXor:    {1, true},
	token.OpAnd:    {2, true},
	token.OpPlus:   {3, true},
	token.OpMinus:  {3, true},
	token.OpFactor: {4, true},
	token.OpDiv:    {4, true},
	token.OpMod:    {4, true},
}

var precUnary = map[token.Token]prec{
	token.OpPlus:  {5, false},
	token.OpMinus: {5, false},
	token.OpXor:   {5, false},
}

type Parser struct {
	f string
	s *scanner.Scanner
	n *scanner.Token
}

func NewParser(f string, s *scanner.Scanner) *Parser {
	return &Parser{f, s, nil}
}

// ParseExpr parses expressions using a precedence climbing algorithm.
// See http://www.engr.mun.ca/~theo/Misc/exp_parsing.htm#climbing.
//
func (p *Parser) ParseExpr() (*Node, error) {
	n, err := p.parseExpr(0)
	if err != nil {
		return n, err
	}
	return n, p.expectEndOfExpr()
}

func (p *Parser) skipToEOL() {
	for {
		t := p.next()
		if t.Token == token.EOL || t.Token == token.EOF {
			p.putBack(t)
			return
		}
	}
}

func (p *Parser) expectEndOfExpr() error {
	t := p.nextNonSpace()
	switch t.Token {
	case token.Comma:
		return nil
	case token.EOL, token.EOF:
		p.putBack(t)
		return nil
	case token.Error:
		p.skipToEOL()
		return fmt.Errorf("%s:%s: %s", p.f, t.Pos.String(), t.String())
	default:
		p.skipToEOL()
		return fmt.Errorf("%s:%s: unexpected token %s at end", p.f, t.Pos.String(), t.String())
	}
}

func (p *Parser) expect(tok token.Token) (t *scanner.Token, ok bool) {
	t = p.nextNonSpace()
	if t.Token != tok {
		p.putBack(t)
		return t, false
	}
	return t, true
}

func (p *Parser) next() *scanner.Token {
	if rv := p.n; rv != nil {
		p.n = nil
		return rv
	}
	t := p.s.Scan()
	return &t
}

func (p *Parser) nextNonSpace() *scanner.Token {
	var t *scanner.Token
	// eat spaces and comments
	for t = p.next(); t.Token == token.Space || t.Token == token.Comment; t = p.next() {
	}
	return t
}

func (p *Parser) putBack(t *scanner.Token) {
	if p.n != nil {
		panic("putBack() called twice.")
	}
	p.n = t
}

func (p *Parser) parseExpr(prec int) (*Node, error) {
	n, err := p.parsePrimary()
	if err != nil {
		return n, err
	}
	for {
		t := p.nextNonSpace()
		pn, ok := precBin[t.Token]
		if !ok {
			p.putBack(t)
			break
		}
		if pn.prec < prec {
			p.putBack(t)
			break
		}
		var n1 *Node
		if pn.ltr {
			n1, err = p.parseExpr(pn.prec + 1)
		} else {
			n1, err = p.parseExpr(pn.prec)
		}
		n = &Node{t, n, n1}
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (p *Parser) parsePrimary() (n *Node, err error) {
	// TODO: imnplement special case handling of %identifier for built-ins
	t := p.nextNonSpace()
	if t.Token == token.OpMod {
		t2, ok := p.expect(token.Identifier)
		if ok {
			t.Token = token.BuiltIn
			t.Raw = append([]byte{'%'}, t2.Raw...)
		}
	}
	if prec, ok := precUnary[t.Token]; ok {
		n, err = p.parseExpr(prec.prec)
		n = &Node{t, n, nil}
		if err != nil {
			return n, err
		}
	} else {
		switch t.Token {
		case token.LeftParen:
			n, err = p.parseExpr(0)
			if err != nil {
				return n, err
			}
			_, ok := p.expect(token.RightParen)
			if !ok {
				return n, fmt.Errorf("expected ')'. Matching start '(' at %s", t.Pos.String())
			}
		case token.Immediate, token.Identifier, token.LocalLabel, token.BuiltIn:
			n = &Node{t, nil, nil}
		default:
			p.putBack(t)
			return nil, fmt.Errorf("unexpected token %s in primary", t.String())
		}
	}

	// postfix function calls.
	// LTR associative. Has higher precedence than anything else.
	for {
		t = p.nextNonSpace()
		if t.Token == token.LeftParen {
			t.Raw = []byte("·")
			n1, err := p.parseExpr(0)
			n = &Node{t, n, n1}
			if err != nil {
				return nil, err
			}
			_, ok := p.expect(token.RightParen)
			if !ok {
				return n, fmt.Errorf("missing ')' in fn call. Matching start '(' at %s", t.Pos.String())
			}
		} else {
			p.putBack(t)
			break
		}
	}
	return n, nil
}
