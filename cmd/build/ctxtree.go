package main

import _ "embed"

// ctxTree is a tree of contexts that plays how cancellation travels
// through it: a model call's deadline passing, then Ctrl-C at the root.
// A page includes it with {{ctxtree}}; its styles are in style.css and
// its script in base.html. Without script it is a still picture with
// its caption.
//
//go:embed ctxtree.html
var ctxTree []byte
