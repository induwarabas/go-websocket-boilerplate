package main

import (
	"go-websocket-boilerplate/internal/open_auth"
	"go-websocket-boilerplate/internal/server"
)

func main() {
	wsgw := server.NewWsGw(open_auth.NewOpenAuthenticator())
	wsgw.Start()
}
