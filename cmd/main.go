package main

import (
	"broadcas-server/internal/client"
	"broadcas-server/internal/server"
	"flag"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: broadcast-server <start|connect> [options]")
		os.Exit(1)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet("broadcast-server", flag.ExitOnError)
	port := fs.String("port", "8080", "server port")
	username := fs.String("username", "", "username")
	fs.Parse(os.Args[2:])

	switch cmd {
	case "start":
		server.StartServer(*port)
	case "connect":
		client.StartClient(*port, *username)
	}
}
