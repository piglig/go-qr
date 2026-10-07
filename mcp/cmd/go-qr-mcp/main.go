// Command go-qr-mcp is a Model Context Protocol server that gives AI
// assistants exact QR Code decoding, generation and content inspection.
//
// By default it speaks MCP over stdin and stdout, which is how desktop
// clients launch local servers:
//
//	go-qr-mcp                  # stdio
//	go-qr-mcp -root ~/Pictures # confine file access to a directory
//	go-qr-mcp -http localhost:8090
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/piglig/go-qr/mcp/server"
)

func main() {
	root := flag.String("root", "", "confine files the tools read and write to this directory")
	addr := flag.String("http", "", "serve MCP over streamable HTTP on this address instead of stdio, for example localhost:8090")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "go-qr-mcp: an MCP server for QR Codes (decode_qr, generate_qr, inspect_qr).\n\nUsage:\n  go-qr-mcp [flags]\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	version := "(devel)"
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		version = bi.Main.Version
	}
	if *showVersion {
		fmt.Println(version)
		return
	}

	s, err := server.New(server.Options{Root: *root, Version: version})
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if *addr != "" {
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, nil)
		log.Printf("go-qr-mcp %s listening on http://%s", version, *addr)
		srv := &http.Server{Addr: *addr, Handler: handler}
		go func() {
			<-ctx.Done()
			srv.Close()
		}()
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
		return
	}
	if err := s.Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
