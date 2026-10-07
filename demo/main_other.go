//go:build !(js && wasm)

package main

import (
	"log"
	"net/http"
)

// main serves web/ for local development. Build web/qr.wasm first; see the
// package documentation.
func main() {
	const addr = "localhost:8080"
	log.Printf("serving the demo on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, http.FileServer(http.Dir("web"))))
}
