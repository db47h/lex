package parser

import (
	"errors"
	"fmt"

	"github.com/db47h/asm/lexer"
	"github.com/db47h/asm/token"
)

type parseErrorKind int

const (
	errLexer           parseErrorKind = -1
	errUnexpectedToken parseErrorKind = iota
)

// ParseError wraps a parsing error and keeps track of
// error kind and position.
//
type ParseError struct {
	i *lexer.Item
	f *token.File
	k parseErrorKind
}

// Error implements the error interface
//
func (e *ParseError) Error() string {
	l, c := e.f.Position(e.i.Pos)
	switch e.k {
	case errLexer:
		return fmt.Sprintf("%s:%d:%d %v", e.f.Name(), l, c, e.i.Value)
	case errUnexpectedToken:
		return fmt.Sprintf("%s:%d:%d: %s %s", e.f.Name(), l, c, "unexpected token", e.i.String())
	}
}

type Node struct {
	l *lexer.Item
	c []*Node
}

func tokString(i *lexer.Item) string {
	switch v := i.Value.(type) {
	case string:
		return v
	case interface {
		String() string
	}:
		return v.String()
	case nil:
		return i.Token.String()
	default:
		panic(fmt.Errorf("unhandled token value type %T for %v", v, v))
	}
}

func (n *Node) String() string {
	if len(n.c) == 0 {
		return tokString(n.l)
	}
	s := "("
	if len(n.c) < 2 {
		s += tokString(n.l) + " " + n.c[0].String()
	} else {
		for i := range n.c[:len(n.c)-1] {
			s += n.c[i].String() + tokString(n.l)
		}
		s += n.c[len(n.c)-1].String()
	}
	return s + ")"
}

type leftOpSpec struct {
	prec int
	led  func(p *Parser, lhs *Node, i *lexer.Item, s *leftOpSpec) (*Node, error)
	ra   bool
}

var leftOp = map[token.Token]leftOpSpec{
	token.Comma:     {0, leftChain, false},
	token.OpOr:      {1, leftBinOp, false},
	token.OpXor:     {2, leftBinOp, false},
	token.OpAnd:     {3, leftBinOp, false},
	token.OpPlus:    {4, leftBinOp, false},
	token.OpMinus:   {4, leftBinOp, false},
	token.OpFactor:  {5, leftBinOp, false},
	token.OpDiv:     {5, leftBinOp, false},
	token.OpMod:     {5, leftBinOp, false},
	token.Backslash: {6, leftPostfix, false},
	token.LeftParen: {6, leftParen, false},
}

type nullOpSpec struct {
	prec int
	nud  func(p *Parser, i *lexer.Item, s *nullOpSpec) (*Node, error)
}

var nullOp = map[token.Token]nullOpSpec{
	token.OpPlus:     {6, nullUnaryOp},
	token.OpMinus:    {6, nullUnaryOp},
	token.OpXor:      {6, nullUnaryOp},
	token.LeftParen:  {0, nullParen},
	token.Identifier: {0, nullLeaf},
	token.Immediate:  {0, nullLeaf},
}

func leftBinOp(p *Parser, lhs *Node, i *lexer.Item, s *leftOpSpec) (*Node, error) {
	var prec int
	if s.ra {
		prec = s.prec
	} else {
		prec = s.prec + 1
	}
	rhs, err := p.parseExpr(prec)
	if err != nil {
		return nil, err
	}
	if lhs.l.Token == i.Token {
		lhs.c = append(lhs.c, rhs)
		return lhs, nil
	}
	return &Node{i, []*Node{lhs, rhs}}, nil
}

func leftChain(p *Parser, lhs *Node, i *lexer.Item, s *leftOpSpec) (*Node, error) {
	rhs, err := p.parseExpr(s.prec + 1)
	if err != nil {
		return nil, err
	}
	if lhs.l.Token == i.Token {
		lhs.c = append(lhs.c, rhs)
		return lhs, nil
	}
	return &Node{i, []*Node{lhs, rhs}}, nil
}

func leftPostfix(_ *Parser, lhs *Node, i *lexer.Item, _ *leftOpSpec) (*Node, error) {
	return &Node{i, []*Node{lhs}}, nil
}

func leftParen(p *Parser, lhs *Node, i *lexer.Item, s *leftOpSpec) (*Node, error) {
	inner, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	if _, ok := p.expect(token.RightParen); !ok {
		return nil, errors.New("missing )")
	}
	i.Value = string('·')
	return &Node{i, []*Node{lhs, inner}}, nil
}

func nullUnaryOp(p *Parser, i *lexer.Item, s *nullOpSpec) (*Node, error) {
	rhs, err := p.parseExpr(s.prec)
	if err != nil {
		return nil, err
	}
	return &Node{i, []*Node{rhs}}, nil
}

func nullParen(p *Parser, i *lexer.Item, s *nullOpSpec) (*Node, error) {
	inner, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	if _, ok := p.expect(token.RightParen); !ok {
		return nil, errors.New("missing )")
	}
	return inner, nil
}

func nullLeaf(p *Parser, i *lexer.Item, s *nullOpSpec) (*Node, error) {
	return &Node{i, nil}, nil
}

type Parser struct {
	f  *token.File
	l  *lexer.Lexer
	n  *lexer.Item
	lt map[token.Token]leftOpSpec
	nt map[token.Token]nullOpSpec
}

func NewParser(f *token.File) *Parser {
	return &Parser{f, lexer.New(f, nil), nil, leftOp, nullOp}
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

// expectEndOfExpr checks wether the next token marks the end of an expression.
// Expressions are terminated by a comma, EOL or EOF. The token marking the end
// of the expression is never consumed.
// On platforms with register displacement notation using brackets [], the opening
// bracket could be added as an end of expression marker.
//
func (p *Parser) expectEndOfExpr() error {
	i := p.nextNonSpace()
	switch i.Token {
	case token.Comma:
		p.putBack(i)
		return nil
	case token.EOL, token.EOF:
		p.putBack(i)
		return nil
	case token.Error:
		p.skipToEOL()
		return &ParseError{f: p.f, i: i, k: errLexer}
	default:
		p.skipToEOL()
		return &ParseError{f: p.f, i: i, k: errUnexpectedToken}
	}
}

func (p *Parser) expect(tok token.Token) (i *lexer.Item, ok bool) {
	i = p.nextNonSpace()
	if i.Token != tok {
		p.putBack(i)
		return i, false
	}
	return i, true
}

func (p *Parser) next() *lexer.Item {
	if rv := p.n; rv != nil {
		p.n = nil
		return rv
	}
	return p.l.Lex()
}

func (p *Parser) nextNonSpace() *lexer.Item {
	var i *lexer.Item
	// eat spaces and comments
	for i = p.next(); i.Token == token.Space || i.Token == token.Comment; i = p.next() {
	}
	return i
}

func (p *Parser) putBack(i *lexer.Item) {
	if p.n != nil {
		panic("putBack() called twice.")
	}
	p.n = i
}

// nextPrimary returns the next token where the token is expected to be a primary.
// Filters special cases like builtins that are composed of two other tokens:
//
//		Builtin := OpMod Identifier
//
func (p *Parser) nextPrimary() *lexer.Item {
	i := p.nextNonSpace()
	if i.Token == token.OpMod {
		t2, ok := p.expect(token.Identifier)
		if ok {
			i.Token = token.BuiltIn
			i.Value = "%" + t2.Value.(string)
		}
	}
	return i
}

// parseExpr returns the AST for an expression.
// In the AST, function calls marked by a node with token.LeftParen
// identify either a built-in function (if the lhs of the node is a token.BuiltIn)
//  or a register displacement (any other lhs, assumed to be an immediate).
//
func (p *Parser) parseExpr(pmin int) (*Node, error) {
	// primary
	t := p.nextPrimary()
	s, ok := p.nt[t.Token]
	if !ok {
		return nil, &ParseError{f: p.f, i: t, k: errUnexpectedToken}
	}
	n, err := s.nud(p, t, &s)
	if err != nil {
		return nil, err
	}
	//
	for {
		i := p.nextNonSpace()
		s, ok := p.lt[i.Token]
		if !ok || s.prec < pmin {
			p.putBack(i)
			return n, nil
		}
		n, err = s.led(p, n, i, &s)
		if err != nil {
			return nil, err
		}
	}
}
