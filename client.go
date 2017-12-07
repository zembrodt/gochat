package main

import (
    "fmt"
    "os"
    "bufio"
	//"gochat/clnt"
	"github.com/zembrodt/gochat/clnt"
)

func main() {
    args := os.Args[1:]
    username := "default_user"
	server := "localhost:8080"
    switch len(args) {
		case 2:
			server = args[1]
			fallthrough
		case 1:
			username = args[0]
    }
    
	client := clnt.NewClient(username)//, "localhost")
	err := client.Connect(server)
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		os.Exit(1)
	}
	fmt.Printf("Welcome %s!\n", client.Username)
    
	defer client.Disconnect(server)
    
    scanner := bufio.NewScanner(os.Stdin)
    var input string
    for {
        scanner.Scan()
        input = scanner.Text()
		
        if input == "q" || input == "quit" || input == "exit" {
            fmt.Println("Exiting...")
            break
        } else {
            go client.HandleRequest(input)
        }
    }
}