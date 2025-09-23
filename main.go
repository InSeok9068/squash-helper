package main

import (
	"flag"
	"fmt"
	"squash-helper/client"
	"squash-helper/server"
)

func main() {
	flag.Parse()
	args := flag.Args()

	fmt.Println(args)

	// mod := "server"
	mod := "client"

	if len(args) > 0 {
		mod = args[0]
	}

	if mod == "client" {
		client.Run()
	} else {
		server.Run()
	}
}
