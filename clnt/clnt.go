/*
Package clnt implements a Client struct with methods that allows it to connect and send messages
to a gochat/svr server, as well as listen for responses from the same server.

Also records a cached state of groups on the server the client belongs to for quick
access without needing to ping the server.
*/
package clnt

import (
	"fmt"
	"github.com/zembrodt/gochat"
	"net"
	"encoding/gob"
	"errors"
	"strings"
)

type Client struct {
	Username, Address string
	MyGroups *gochat.GroupMap // cached version of Client's groups
}

// Client constructor
func NewClient(username string) *Client {
	return &Client{username, "localhost", gochat.NewGroupMap()}
}

// Connects a Client to a server and sends the 'init' message and starts a Client.Listen
// goroutine on the port the server responds with
func (client *Client) Connect(address string) (err error) {
	// Establish connection with the server
    conn, err := net.Dial("tcp", address)
	defer conn.Close()
    if err != nil {
        return
    }
	encoder := gob.NewEncoder(conn)
    // Send the cmd 'init' to let the server know this is our first time connecting
	request := &gochat.Msg{User: client.Username, Cmd: "init"}
    err = encoder.Encode(request)
    if err != nil {
        fmt.Println("Encoder error:", err)
		return
    }
	// Get response from server for the port
	var port string
    decoder := gob.NewDecoder(conn)
    err = decoder.Decode(&port)
    if err != nil {
        fmt.Println("Decoding error:",err)
		return
    }
	// Check for special case that this username already exists on the server
	if (port == "alreadyExists") {
		return errors.New(fmt.Sprintf("Error: User '%s' already exists on the server!\n", client.Username))
	}
	// Start the Listen goroutine
	errCh := make(chan error)
	go client.Listen(port, errCh)
	// Check if Listen encountered an error starting up net.Listen
	if err = <- errCh; err != nil {
		return err
	}
	//Add the global group to cache of client's groups
	client.MyGroups.Create("global", "")
	client.MyGroups.AddUser("global", client.Username)
	
	return nil
}

// Handles the input entered by the Client and creates the Msg to send to the server
func (client *Client) HandleRequest(input string) {
    // Split input on whitespace
	args := strings.Fields(input)
	if len(args) > 3 {
		// Join all strings after the first 2 arguments into a single 3rd argument
		// This will allow messages with spaces to be valid
		args = []string{args[0], args[1], strings.Join(args[2:], " ")}
	}
	// Assign the args to a Msg in the following format:
	// 0: the Cmd user wants to execute
	// 1: who the Cmd should be executed on
	// 2: The contents of the message
    msg := &gochat.Msg{}
	msg.User = client.Username
	switch len(args) {
	case 3:
		msg.Msg = args[2]
		fallthrough
	case 2:
		msg.To = args[1]
		fallthrough
	case 1:
		msg.Cmd = args[0]
	default:
		//should just be empty string
		return
	}
    // Check what Cmd the user wants and if it's valid
	// 'groups' and 'users' are commands that access the Client's local cache
	switch msg.Cmd {
	case "join", "dm", "leave", "create", "delete", "group", "kick":
		// Send the message to the server
		err := msg.Send("localhost:8080")
		if err != nil {
			fmt.Println("Error sending msg:", err)
		}
	// Local messages
	case "groups":
		// Print out all group names
		groupNames := client.MyGroups.GroupNames()
		if (len(groupNames) > 0) {
			fmt.Println("Groups:")
			for _, groupName := range groupNames {
				fmt.Printf(" * %s\n", groupName)
			}
		} else {
			fmt.Println("You belong to no groups.")
		}
	case "users":
		if msg.To == "" {
			fmt.Println("Please enter a group name to get the users of.")
			break
		}
		// Print out all users in the given group
		if group, ok := client.MyGroups.Get(msg.To); ok {
			fmt.Printf("Users in %s:\n", msg.To)
			for _, user := range group.Users.Array() {
				fmt.Printf(" * %s\n", user)
			}
		} else {
			fmt.Printf("You do not belong to the group %s.\n", msg.To)
		}
	default:
		fmt.Printf("Unknown command '%s'\n", msg.Cmd)
	}
}

// Listens on a port given by the server for messages, usually from other Clients
func (client *Client) Listen(port string, errCh chan error) {
    addr := fmt.Sprintf("%s:%s", client.Address, port)
	// Create the net.Listen
    listen, err := net.Listen("tcp", addr)
    if err != nil {
		// Send an error through the channel if one is encountered
        errCh <- err
		close(errCh)
		return
    }
	// Close the error channel so Connect can continue
	close(errCh)
    defer listen.Close()
    fmt.Println("Listening on port", port)
    for {
		// Blocks until a message is received
        conn, err := listen.Accept()
        if err != nil {
            continue
        }
		// call goroutine of HandlerResponse to handle the server message
        go client.HandleResponse(conn)
    }
}

// Determines how to process a message received as a response from the server and what to output
func (client *Client) HandleResponse(conn net.Conn) {
	defer conn.Close()
    response := &gochat.Msg{}
    response.Retrieve(conn)
	// Decisions of how to update local cache based on type of response message
	if response.User == client.Username {
		// Responses from the server from messages we sent
		switch response.Cmd {
		case "leave", "delete":
			// We left a group or deleted it, so delete our local copy of it
			client.MyGroups.Delete(response.To)
		//case "kick":
		//	client.MyGroups.RemoveUser(msg.To, msg.Msg)
		case "create", "join":
			// We created or joined a group, so create a local copy of it
			client.MyGroups.Create(response.To, "")
			client.MyGroups.AddUser(response.To, response.User)
		}
	} else {
		// Responses from the server from messages other clients sent
		switch response.Cmd {
		case "leave", "kick":
			// A user left a group or was kicked, so remove them from our local copy
			client.MyGroups.RemoveUser(response.To, response.User)
		case "delete":
			// A group was deleted, so delete our local copy
			client.MyGroups.Delete(response.To)
		case "join":
			// A user joined a group we're in, so update our local copy
			client.MyGroups.AddUser(response.To, response.User)
		}
	}
	// Only print if we have a message
	if response.Msg != "" {
		fmt.Printf("%s\n", response.Msg)
	}
}

// Sends a message to the server saying the Client is disconnecting
func (client *Client) Disconnect(server string) {
	request := &gochat.Msg{User: client.Username, Cmd: "disconnect"}
	err := request.Send(server)
	if err != nil {
		fmt.Println("Error sending disconnect:", err)
	}
}