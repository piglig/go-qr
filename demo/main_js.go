//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"
)

// main exposes two functions to the page, announces them with a
// "goqr-ready" event, and blocks:
//
//	goqrEncode(requestJSON string) string
//	goqrDecode(rgba Uint8Array | Uint8ClampedArray, width, height int) string
func main() {
	js.Global().Set("goqrEncode", js.FuncOf(func(_ js.Value, args []js.Value) any {
		var req EncodeRequest
		if err := json.Unmarshal([]byte(args[0].String()), &req); err != nil {
			return marshal(EncodeResponse{Error: err.Error()})
		}
		return marshal(encode(req))
	}))
	js.Global().Set("goqrDecode", js.FuncOf(func(_ js.Value, args []js.Value) any {
		src := args[0]
		pix := make([]byte, src.Get("length").Int())
		// CopyBytesToGo takes a Uint8Array; canvas data is a Uint8ClampedArray.
		view := js.Global().Get("Uint8Array").New(src.Get("buffer"), src.Get("byteOffset"), len(pix))
		js.CopyBytesToGo(pix, view)
		return marshal(decodeRGBA(pix, args[1].Int(), args[2].Int()))
	}))
	js.Global().Call("dispatchEvent", js.Global().Get("Event").New("goqr-ready"))
	select {}
}

func marshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		b, _ = json.Marshal(map[string]string{"error": err.Error()})
	}
	return string(b)
}
