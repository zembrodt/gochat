package main

import (
	"fmt"
	"os"
	//"gochat/svr"
	"github.com/zembrodt/gochat/svr"
)


func main() {
	args := os.Args[1:]
	address := "localhost"
	port := "8080"
	if len(args) > 0 {
		port = args[1]
	}
	fmt.Println("Listening on port", port)
	server := svr.NewServer(fmt.Sprintf("%s:%s", address, port))
	err := server.Listen()
	if err != nil {
		fmt.Println("Server returned with error:", err)
	}
}