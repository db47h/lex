// +build ignore

//
package parser

import (
	"errors"
	"fmt"

	"github.com/db47h/parsekit/lexer"
	"github.com/db47h/parsekit/token"
)

const maxPrec int = int(^uint(0) >> 1)
const minPrec int = -maxPrec - 1

var (
	errLexer           = errors.New("lexer error")
	errUnexpectedToken = errors.New("unexpected token")
	errUnbalancedParen = errors.New("missing )")
)

// ParseError wraps a parsing error and keeps track of
// error kind and position.
//
type ParseError struct {
	i *lexer.Item
	f *token.File
	e error
}

// Error implements the error interface
//
func (e *ParseError) Error() string {
	p := e.f.Position(e.i.Pos)
	switch e.e {
	case errLexer:
		return fmt.Sprintf("%s: %v", p.String(), e.i.Value)
	case errUnexpectedToken:
		return fmt.Sprintf("%s: %s %s", p.String(), "unexpected token", e.i.String())
	default:
		return fmt.Sprintf("%s: %s", p.String(), e.e)
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
		return fmt.Sprintf("%v", i.Type)
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

// A Null is any language entity that preceedes its argument(s) or that has
// no arguments.
//
type Null interface {
	NuD(p *Parser, i *lexer.Item) (*Node, error) // Null Denotation
}

// A NullFunc is a function implementing the Null interface.
//
type NullFunc func(p *Parser, i *lexer.Item) (*Node, error)

// NuD implements the Null interface.
//
func (f NullFunc) NuD(p *Parser, i *lexer.Item) (*Node, error) {
	return f(p, i)
}

// A Left is any language entity that takes arguments on its left.
// It can have 0 or more arguments to its right.
//
type Left interface {
	LeD(p *Parser, i *lexer.Item, lhs *Node) (*Node, error) // Left Denotation
	LBP() int                                               // left binding power (or precedence).
}

// PostfixFunc is the prototoype of the Parse function for Postfix.
//
type PostfixFunc func(p *Parser, i *lexer.Item, lhs *Node) (*Node, error)

// LeD implements the LeD method for the Postfix interface.
//
func (f PostfixFunc) LeD(p *Parser, i *lexer.Item, lhs *Node) (*Node, error) {
	return f(p, i, lhs)
}

type postfixWrapper struct {
	PostfixFunc
	p int
}

func (w *postfixWrapper) LBP() int { return w.p }

// WrapPostfixFunc returns a Postfix wrapper around the provided function
// with the given precedence.
//
func WrapPostfixFunc(prec int, f PostfixFunc) Left {
	return &postfixWrapper{f, prec}
}

// func (w *postfixWrapper) Parse(p *Parser, i *lexer.Item, lhs *Node) (*Node, error) {
// 	return w.f(p, i, lhs)
// }

var postfix = map[token.Type]Left{
// token.Comma:     {0, leftChain},
// token.OpOr:      {1, leftBinOp},
// token.OpXor:     {2, leftBinOp},
// token.OpAnd:     {3, leftBinOp},
// token.OpPlus:    {4, leftBinOp},
// token.OpMinus:   {4, leftBinOp},
// token.OpFactor:  {5, leftBinOp},
// token.OpDiv:     {5, leftBinOp},
// token.OpMod:     {5, leftBinOp},
// token.LeftParen: {6, leftParen},
// token.Error:     {100, leftError},
}

var prefix = map[token.Type]Null{
// token.Error:      tokenError{},
// token.Identifier: NullFunc(Leaf),
// token.Immediate:  NullFunc(Leaf),
// token.LeftParen:  SubExpression(token.RightParen),
// token.OpPlus:     UnaryOperator(6),
// token.OpMinus:    UnaryOperator(6),
// token.OpXor:      UnaryOperator(6),
// token.OpMod: NullFunc(func(p *Parser, i *lexer.Item) (*Node, error) {
// 	n := p.next()
// 	if n.Token != token.Identifier {
// 		p.putBack(n)
// 		// TODO: the error here is mode complex.
// 		// should be something like "malformed built-in".
// 		return nil, &ParseError{f: p.f, i: i, e: errUnexpectedToken}
// 	}
// 	i.Value = "%_" + n.Value.(string)
// 	return &Node{l: i, c: nil}, nil
// }),
}

type tokenError struct{}

func (tokenError) NuD(p *Parser, i *lexer.Item) (*Node, error) {
	return nil, &ParseError{f: p.f, i: i, e: errLexer}
}

func (tokenError) LeD(p *Parser, i *lexer.Item, _ *Node) (*Node, error) {
	return nil, &ParseError{f: p.f, i: i, e: errLexer}
}

func (tokenError) Precedence() int {
	//
	return maxPrec
}

// Leaf returns a leaf node.
//
func Leaf(p *Parser, i *lexer.Item) (*Node, error) {
	return &Node{i, nil}, nil
}

// SubExpression parses a sub expression ending with the given token.
//
func SubExpression(end token.Type) Null {
	return NullFunc(func(p *Parser, i *lexer.Item) (*Node, error) {
		inner, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		if _, ok := p.expect(end); !ok {
			return nil, &ParseError{f: p.f, i: i, e: errUnbalancedParen}
		}
		return inner, nil
	})
}

// UnaryOperator returns a unary operator with the given precedence.
//
func UnaryOperator(prec int) Null {
	return NullFunc(func(p *Parser, i *lexer.Item) (*Node, error) {
		rhs, err := p.parseExpr(prec)
		if err != nil {
			return nil, err
		}
		return &Node{i, []*Node{rhs}}, nil
	})
}

// func leftError(p *Parser, _ *Node, i *lexer.Item) (*Node, error) {
// 	return nil, &ParseError{f: p.f, i: i, e: errLexer}
// }

// func nullError(p *Parser, i *lexer.Item) (*Node, error) {
// 	return nil, &ParseError{f: p.f, i: i, e: errLexer}
// }

// func leftBinOp(p *Parser, lhs *Node, i *lexer.Item, s *leftOpSpec) (*Node, error) {
// 	rhs, err := p.parseExpr(s.prec + 1)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if lhs.l.Token == i.Token {
// 		lhs.c = append(lhs.c, rhs)
// 		return lhs, nil
// 	}
// 	return &Node{i, []*Node{lhs, rhs}}, nil
// }

// func leftChain(p *Parser, lhs *Node, i *lexer.Item, s *leftOpSpec) (*Node, error) {
// 	rhs, err := p.parseExpr(s.prec + 1)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if lhs.l.Token == i.Token {
// 		lhs.c = append(lhs.c, rhs)
// 		return lhs, nil
// 	}
// 	return &Node{i, []*Node{lhs, rhs}}, nil
// }

// func leftParen(p *Parser, lhs *Node, i *lexer.Item, _ *leftOpSpec) (*Node, error) {
// 	inner, err := p.parseExpr(0)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if _, ok := p.expect(token.RightParen); !ok {
// 		return nil, &ParseError{f: p.f, i: i, e: errUnbalancedParen}
// 	}
// 	i.Value = string('·')
// 	return &Node{i, []*Node{lhs, inner}}, nil
// }

type Parser struct {
	f    *token.File
	l    *lexer.Lexer
	n    *lexer.Item
	post map[token.Type]Left
	pre  map[token.Type]Null
}

func NewParser(f *token.File) *Parser {
	return &Parser{f, lexer.New(f, nil), nil, postfix, prefix}
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

// func (p *Parser) skipToEOL() {
// 	for {
// 		t := p.next()
// 		if t.Token == token.EOL || t.Token == token.EOF {
// 			p.putBack(t)
// 			return
// 		}
// 	}
// }

// expectEndOfExpr checks wether the next token marks the end of an expression.
// Expressions are terminated by a comma, EOL or EOF. The token marking the end
// of the expression is never consumed.
//
func (p *Parser) expectEndOfExpr() error {
	i := p.nextNonSpace()
	switch i.Type {
	// case token.Comma:
	// 	p.putBack(i)
	// 	return nil
	// case token.EOL, token.EOF:
	// 	p.putBack(i)
	// 	return nil
	case token.Error:
		// p.skipToEOL()
		return &ParseError{f: p.f, i: i, e: errLexer}
	default:
		//p.skipToEOL()
		return &ParseError{f: p.f, i: i, e: errUnexpectedToken}
	}
}

func (p *Parser) expect(tok token.Type) (i *lexer.Item, ok bool) {
	i = p.nextNonSpace()
	if i.Type != tok {
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
	i := p.l.Lex()
	return &i
}

func (p *Parser) nextNonSpace() *lexer.Item {
	var i *lexer.Item
	// eat spaces and comments
	// for i = p.next(); i.Token == token.Space || i.Token == token.Comment; i = p.next() {
	// }
	return i
}

func (p *Parser) putBack(i *lexer.Item) {
	if p.n != nil {
		panic("putBack() called twice.")
	}
	p.n = i
}

func (p *Parser) lookupPrefix(t token.Type) Null {
	pf := p.pre[t]
	if pf != nil {
		return pf
	}
	return NullFunc(func(p *Parser, i *lexer.Item) (*Node, error) {
		return nil, &ParseError{f: p.f, i: i, e: errUnexpectedToken}
	})
}

// pfNotFound is rteturned when no matching postfix operator is found.
// since it's not necessarily an error, we set its precedence to the minimum
// possible value.
//
var pfNotFound = WrapPostfixFunc(minPrec, func(p *Parser, i *lexer.Item, _ *Node) (*Node, error) {
	return nil, &ParseError{f: p.f, i: i, e: errUnexpectedToken}
})

func (p *Parser) lookupPostfix(t token.Type) Left {
	pf := p.post[t]
	if pf != nil {
		return pf
	}
	return pfNotFound
}

// parseExpr returns the AST for an expression.
// In the AST, function calls marked by a node with token.LeftParen
// identify either a built-in function (if the lhs of the node is a token.BuiltIn)
//  or a register displacement (any other lhs, assumed to be an immediate).
//
func (p *Parser) parseExpr(rbp int) (*Node, error) {
	// primary
	i := p.nextNonSpace()
	pr := p.lookupPrefix(i.Type)
	n, err := pr.NuD(p, i)
	if err != nil {
		return nil, err
	}
	//
	for {
		i = p.nextNonSpace()
		po := p.lookupPostfix(i.Type)
		if po.LBP() < rbp {
			p.putBack(i)
			return n, nil
		}
		n, err = po.LeD(p, i, n)
		if err != nil {
			return nil, err
		}
	}
}
