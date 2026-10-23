package main

import _ "embed"

// errChain is the chain of errors a run returns when every model call
// got a 429, as chapter 12's code builds it: wrapped layer by layer,
// then searched by errors.AsType twice, once for the *APIError and once
// for anything retryable. A page includes it with {{errchain}}; its
// styles are in style.css and its script in base.html. Without script
// it is the whole chain, still, with its caption.
//
//go:embed errchain.html
var errChain []byte
