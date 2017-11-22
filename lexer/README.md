

# lexer
`import "github.com/db47h/parsekit/lexer"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)
* [Subdirectories](#pkg-subdirectories)

## <a name="pkg-overview">Overview</a>
Package lexer provides the core of a lexer built as a DFA whose states are
implemented as functions.

Clients of the package only need to provide state functions specialized in
lexing the target language. The package provides facilities to stream input
from a RuneReader (this will be changed in the future to a simple io.Reader)
with one char look-ahead, as well as utility functions commonly used in lexers.

Implementation details:

The implementation is similar to <a href="https://golang.org/src/text/template/parse/lex.go">https://golang.org/src/text/template/parse/lex.go</a>.
Se also Rob Pike's talk about using state functions for lexing: <a href="https://talks.golang.org/2011/lex.slide">https://talks.golang.org/2011/lex.slide</a>.

Unlike the Go text template package which uses Go channels as a mean of
asynchronous token emission, this package uses a FIFO queue instead.
Benchmarks with an earlier implementation that used a channel showed that
using a FIFO is about 5 times faster. There is also no need for the parser
to take care of cancellation or draining the input in case of error.

The drawback of using a FIFO is that once a state function has called Emit,
it must return as soon as possible so that the caller of Lex (usually a
parser) can receive the lexed item.

The state sub-package provides state functions for lexing quoted strings,
quoted characters and numbers (integers in any base as well as floats) and
graceful handling of EOF.




## <a name="pkg-index">Index</a>
* [Constants](#pkg-constants)
* [type Interface](#Interface)
  * [func New(f *token.File, init StateFn) Interface](#New)
* [type Item](#Item)
  * [func (i *Item) String() string](#Item.String)
* [type Lexer](#Lexer)
  * [func (l *Lexer) Accept(r rune) bool](#Lexer.Accept)
  * [func (l *Lexer) AcceptWhile(f func(r rune) bool) int](#Lexer.AcceptWhile)
  * [func (l *Lexer) Backup()](#Lexer.Backup)
  * [func (l *Lexer) Emit(t token.Type, value interface{})](#Lexer.Emit)
  * [func (l *Lexer) Errorf(pos token.Pos, format string, args ...interface{})](#Lexer.Errorf)
  * [func (l *Lexer) Expect(r rune) bool](#Lexer.Expect)
  * [func (l *Lexer) File() *token.File](#Lexer.File)
  * [func (l *Lexer) Last() rune](#Lexer.Last)
  * [func (l *Lexer) Lex() Item](#Lexer.Lex)
  * [func (l *Lexer) Next() rune](#Lexer.Next)
  * [func (l *Lexer) Peek() rune](#Lexer.Peek)
  * [func (l *Lexer) Pos() token.Pos](#Lexer.Pos)
* [type StateFn](#StateFn)


#### <a name="pkg-files">Package files</a>
[lex.go](/src/github.com/db47h/parsekit/lexer/lex.go) 


## <a name="pkg-constants">Constants</a>
``` go
const EOF = -1
```
EOF is the return value from Next() when EOF is reached.





## <a name="Interface">type</a> [Interface](/src/target/lex.go?s=3174:3364#L103)
``` go
type Interface interface {
    Lex() Item         // Lex reads source text and returns the next item until EOF.
    File() *token.File // File returns the token.File used as input for the lexer.
}
```
Interface wraps the public methods of a lexer. This interface is intended for
parsers that call New(), then Lex() until EOF.







### <a name="New">func</a> [New](/src/target/lex.go?s=4873:4920#L147)
``` go
func New(f *token.File, init StateFn) Interface
```
New creates a new lexer associated with the given source file. A new lexer
must be created for every source file to be lexed.





## <a name="Item">type</a> [Item](/src/target/lex.go?s=2263:2617#L75)
``` go
type Item struct {
    // Token type. Can be any user-defined value, token.EOF or token.Error.
    token.Type
    // Token start position within the file (in runes).
    token.Pos
    // The value type is user-defined for token types > 0.
    // For built-in token types, this is nil except for errors where Value
    // is a string describing the error.
    Value interface{}
}
```
Item represents a token returned from the lexer.










### <a name="Item.String">func</a> (\*Item) [String](/src/target/lex.go?s=2781:2811#L89)
``` go
func (i *Item) String() string
```
String returns a string representation of the item. This should be used only
for debugging purposes as the output format is not guaranteed to be stable.




## <a name="Lexer">type</a> [Lexer](/src/target/lex.go?s=3539:4269#L112)
``` go
type Lexer struct {
    // Current initial-state function. It can be used by state functions to
    // implement context switches (e.g. switch to a JS lexer while parsing HTML, etc.)
    // by setting this value to another init function then return nil.
    I StateFn
    // Start position of current token. Used as token position by Emit.
    // calling Emit or returning nil from a StateFn resets its value to Pos() + 1.
    // State functions should normally not need to read or change this value.
    S token.Pos
    // contains filtered or unexported fields
}
```
A Lexer holds the internal state of the lexer while processing a given input.
Note that the public fields should only be accessed from custom StateFn
functions.










### <a name="Lexer.Accept">func</a> (\*Lexer) [Accept](/src/target/lex.go?s=8040:8075#L303)
``` go
func (l *Lexer) Accept(r rune) bool
```
Accept accepts input if it matches r. Returns true if successful.




### <a name="Lexer.AcceptWhile">func</a> (\*Lexer) [AcceptWhile](/src/target/lex.go?s=7830:7882#L292)
``` go
func (l *Lexer) AcceptWhile(f func(r rune) bool) int
```
AcceptWhile accepts input while the f function returns true. The return value
is the number of runes accepted.




### <a name="Lexer.Backup">func</a> (\*Lexer) [Backup](/src/target/lex.go?s=7049:7073#L252)
``` go
func (l *Lexer) Backup()
```
Backup reverts the last call to Next.




### <a name="Lexer.Emit">func</a> (\*Lexer) [Emit](/src/target/lex.go?s=5797:5850#L185)
``` go
func (l *Lexer) Emit(t token.Type, value interface{})
```
Emit emits a single token of the given type and value positioned at l.S.
Emit sets l.S to the start of the next token (i.e. l.Pos() + 1).




### <a name="Lexer.Errorf">func</a> (\*Lexer) [Errorf](/src/target/lex.go?s=6064:6137#L197)
``` go
func (l *Lexer) Errorf(pos token.Pos, format string, args ...interface{})
```
Errorf emits an error token. The Item value is set to a string representation
of the error and the position set to pos.




### <a name="Lexer.Expect">func</a> (\*Lexer) [Expect](/src/target/lex.go?s=8216:8251#L313)
``` go
func (l *Lexer) Expect(r rune) bool
```
Expect checks that the next rune matches r but does not consume it.




### <a name="Lexer.File">func</a> (\*Lexer) [File](/src/target/lex.go?s=5598:5632#L178)
``` go
func (l *Lexer) File() *token.File
```
File returns the token.File used as input for the lexer.




### <a name="Lexer.Last">func</a> (\*Lexer) [Last](/src/target/lex.go?s=7639:7666#L282)
``` go
func (l *Lexer) Last() rune
```
Last returns the last rune read. May panic if called without a previous call
to Next since the last call to Emit or transition to the initial state.




### <a name="Lexer.Lex">func</a> (\*Lexer) [Lex](/src/target/lex.go?s=5360:5386#L164)
``` go
func (l *Lexer) Lex() Item
```
Lex reads source text and returns the next item until EOF.

As a convention, once the end of file has been reached (or some
non-recoverable error condition), Lex() must return an item of type
token.EOF. Implementors of custom lexers must take care of this.




### <a name="Lexer.Next">func</a> (\*Lexer) [Next](/src/target/lex.go?s=6686:6713#L226)
``` go
func (l *Lexer) Next() rune
```
Next returns the next rune in the input stream. IF the end of the stream
has ben reached or an IO error occured, it will return EOF.




### <a name="Lexer.Peek">func</a> (\*Lexer) [Peek](/src/target/lex.go?s=6884:6911#L239)
``` go
func (l *Lexer) Peek() rune
```
Peek returns the next rune in the input stream without consuming it.




### <a name="Lexer.Pos">func</a> (\*Lexer) [Pos](/src/target/lex.go?s=7398:7429#L272)
``` go
func (l *Lexer) Pos() token.Pos
```
Pos returns the position (rune offset) of the last rune read. Returns -1
if no input has been read yet.




## <a name="StateFn">type</a> [StateFn](/src/target/lex.go?s=4701:4736#L142)
``` go
type StateFn func(l *Lexer) StateFn
```
A StateFn is a state function.

As a convention, when a StateFn is called, the input that lead to that state
has already been scanned and can be retrieved with Lexer.Last(). For example,
a state function that lexes numbers will have to call Last() to get the first
digit.

If a StateFn returns nil, the lexer resets the current token start position
then transitions back to its initial state function.














- - -
Generated by [godoc2md](http://godoc.org/github.com/davecheney/godoc2md)
