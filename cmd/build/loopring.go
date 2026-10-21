package main

import _ "embed"

// loopRing is the agent loop drawn as a ring of its four stages beside a
// list of them, with a link to the guide for each. A page includes it
// with {{loop}}; its styles are in style.css and its script in
// base.html, which also link it to listings marked with stages=.
//
//go:embed loopring.html
var loopRing []byte
