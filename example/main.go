package main

import go_qr "github.com/piglig/go-qr"

func main() {

}

func doBasicDemo() {
	text := "Hello world!"
	errCorLvl := go_qr.Low

	_, _ = text, errCorLvl
}
