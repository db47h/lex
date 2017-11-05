# Lexer examples

There's only a single example showing how to implement embedded languages. It's a clone of the Go text template language.

Taking only into account the state functions, the resulting code is roughly half the size of the [original](https://golang.org/src/text/template/parse/lex.go), and performance is slightly better (150µs vs 170µs).

All the interesting things happen when lexing between the `{{` and `}}` delimiters, so we could have written our lexer states like the original:

```go
var lang = lexer.NewLang(func (l *lexer.Lexer) lexer.ScanState {
    l.AcceptUpTo([]rune("{{"))
    // ... discard delimiter
    // ... trim
    // ... emit text preceding delimiter
    return lexInsideAction
})

func lexInsideAction(l *lexer.Lexer) lexer.ScanState {
    r := l.Next()
    switch {
        case atRightDelim():
            // ... discard delimiter
            // return to initial state
            return nil
        case r == '.' || r == '$':
            // ...
        case unicode.IsLetter(r) || r == '_':
            // ...
        // case isSpace...
            // ...
    }
}
```

But we'd loose the ability to use automatic prefix matching since lexInsideAction is not the initial state of the language.

So instead, we define two languages: one for plain text wehere we only check for "{{", and one for "actions", the switch between them on the fly. Additionally, the Go text template lexer checks that parenthesis are properly balanced, so we add some state data with a simple structure and pass closures on that structure as state functions to the lexer.