package main

import (
	"context"
	"flick/internal/server"
)

func main() {
	father, cancel := context.WithCancel(context.Background())
	defer cancel()
	simpleHttpServer := &server.SimpleHTTPServer{}
	err := simpleHttpServer.RunSimpleHTTPServer(father, cancel, ":3000", server.LoggingMiddleware)
	if err != nil {
		panic(err)
	}
}
