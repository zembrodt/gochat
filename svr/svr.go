/*
Package svr implements a server for the client/server interactions.

It will listen on an address given to it on creation and accept incoming messages.
It contains a map of users to addresses to know where to send response messages.
It contains a map of groups to users to know who to send a message to if the request is
for a group message.
*/
package svr

import (
    "fmt"
	"net"
	"strings"
	"github.com/zembrodt/gochat"
	"errors"
	"encoding/gob"
)

// A server is constructed out of an address to listen on and a pointer to maps of
// users to addresses and groups to users, which are shared among threads.
type Server struct {
	address string
	Addrs *gochat.AddrMap
	Groups *gochat.GroupMap
}

// Constructor function for Server
func NewServer(address string) *Server {
	return &Server{address, gochat.NewAddrMap(), gochat.NewGroupMap()}
}

// Tells a server to start listening on its port
func (server *Server) Listen() (err error) {
	listen, err := net.Listen("tcp", server.address)
	if err != nil {
		fmt.Println("Error creating listener:", err)
		return err //or put through chan?
	}
	defer listen.Close()
	// main loop
	for {
		conn, err := listen.Accept()
		if err != nil {
			fmt.Println("Error on accept:", err)
			continue
		}
		// Create goroutine to handle the connection
		go server.HandleRequest(conn)
	}
}

// Parses a message sent by the client and decides what message(s) to send out
func (server *Server) HandleRequest(conn net.Conn) {
	defer conn.Close()
	msg := &gochat.Msg{}
	// Decode the message
	err := msg.Retrieve(conn)
	if err != nil {
		fmt.Println("Error retrieving msg:",err)
		return
	}
	fmt.Printf("Received : %+v\n", msg)
	
	addrs := server.Addrs
	groups := server.Groups
	
	// Parse the message data
	switch msg.Cmd {
	case "init":
		// User has just connected
		encoder := gob.NewEncoder(conn)
		// if user is not in addrs
		if _, ok := addrs.Get(msg.User); !ok {
			// build Addr
			addrStr := strings.Split(conn.RemoteAddr().String(), ":")
			addr := gochat.Addr{addrStr[0], addrStr[1]}
			
			// add addr to map
			addrs.Add(msg.User, addr)
			
			// send the port back to client so they know what to listen on
			fmt.Println("Sending user port",addr.Port)
			err = encoder.Encode(addr.Port)
			if err != nil {
				fmt.Println("Encoding error:",err)
			}
			
			// Add client to global channel
			if ok = groups.AddUser("global", msg.User); !ok {
				groups.Create("global", "")
				groups.AddUser("global", msg.User)
			}
			
			// Update client's global group cache
			if addr, ok := addrs.Get(msg.User); ok {
				group, _ := groups.Get("global")
				for _, groupMember := range group.Users.Array() {
					if groupMember != msg.User {
						cacheUpdate := &gochat.Msg{}
						cacheUpdate.User = groupMember
						cacheUpdate.To = "global"
						cacheUpdate.Cmd = "join"
						err = cacheUpdate.Send(addr.String())
					}
				}
			}
			// Create message to send out to all other users
			msg.Msg = fmt.Sprintf("%s is online.", msg.User)
			msg.Cmd = "join" // so the other users know to update their cache
			msg.To = "global"
			errCh := make(chan error)
			go server.SendGroupMsg(msg, errCh)
			// wait to see if SendGroupMsg encounters any errors
			for {
				if err, ok = <- errCh; ok {
					fmt.Println("Group message error:", err)
				} else {
					break
				}
			}
			
		} else {
			// User already exists, send the 'alreadyExists' response so they exit
			err = encoder.Encode("alreadyExists")
			if err != nil {
				fmt.Println("Encoding error:", err)
			}
		}
		
	case "join":
		// User wants to join a group
		response := &gochat.Msg{}
		*response = *msg // shallow copy
		response.Cmd = ""
		// Check if we were able to add the user to the group
		if ok := groups.AddUser(msg.To, msg.User); ok {
			response.Msg = fmt.Sprintf("You have joined the group %s.", msg.To)
			response.Cmd = "join"
			// Notify all users in the group that this user joined
			msg.Msg = fmt.Sprintf("%s has joined the group.", msg.User)
			errCh := make(chan error)
			go server.SendGroupMsg(msg, errCh)
			// Check for errors
			for {
				if err, ok = <- errCh; ok {
					fmt.Println("Group message error:", err)
				} else {
					break
				}
			}
			// Notify the user they joined
			err = server.SendMsg(response, response.User)
			// Now send the user messages containing all groups currently in that group
			// so they can update their local cache
			group, _ := groups.Get(msg.To)
			for _, groupMember := range group.Users.Array() {
				if groupMember != msg.User {
					cacheUpdate := &gochat.Msg{}
					cacheUpdate.User = groupMember
					cacheUpdate.To = msg.To
					cacheUpdate.Cmd = "join"
					server.SendMsg(cacheUpdate, msg.User)
				}
			}
		} else {
			// The group doesn't exist
			response.Msg = fmt.Sprintf("Group %s doesn't exist.", msg.To)
			err = server.SendMsg(response, response.User)
		}
		
	case "dm":
		// User wants to send a direct message to another user
		// Create the message
		dmMsg := &gochat.Msg{}
		dmMsg.Msg = fmt.Sprintf("%s whispers %s", msg.User, msg.Msg)
		// Send the message
		server.SendMsg(dmMsg, msg.To)
		
	case "group":
		// User wants to send a message to a group
		response := &gochat.Msg{}
		*response = *msg
		response.Cmd = ""
		// Check if the user belongs to the group
		if contains, ok := groups.ContainsUser(msg.To, msg.User); contains {
			// Build the response message for the user
			response.Msg = fmt.Sprintf("[%s] %s: %s", msg.To, msg.User, msg.Msg)
			// Send the message to all other users in the group
			msg.Msg = fmt.Sprintf("%s: %s", msg.User, msg.Msg)
			errCh := make(chan error)
			go server.SendGroupMsg(msg, errCh)
			// Check for errors
			for {
				if err, ok = <- errCh; ok {
					fmt.Println("Group message error:", err)
				} else {
					break
				}
			}
		} else {
			// User is either not in the group or the group doesn't exist
			if !ok {
				response.Msg = fmt.Sprintf("Group %s doesn't exist.", msg.To)
			} else {
				response.Msg = fmt.Sprintf("You don't have access to group %s!", msg.To)
			}
		}
		// Send the response back to the user
		err = server.SendMsg(response, response.User)
		
	case "leave":
		// User wants to leave a group
		response := &gochat.Msg{}
		*response = *msg
		response.Cmd = ""
		// Check if we are able to remove the user from the group
		if ok := groups.RemoveUser(msg.To, msg.User); ok {
			// User was in the group, build their response message
			response.Msg = fmt.Sprintf("You have left the group %s.", msg.To)
			response.Cmd = "leave"
			// Notify all other users in the group the user has left
			msg.Msg = fmt.Sprintf("%s has left the group.", msg.User)
			errCh := make(chan error)
			go server.SendGroupMsg(msg, errCh)
			// Check for errors
			for {
				if err, ok = <- errCh; ok {
					fmt.Println("Group message error:", err)
				} else {
					break
				}
			}
		} else {
			// Group doesn't exist
			response.Msg = fmt.Sprintf("Group %s doesn't exist.", msg.To)
		}
		// Send the response message
		err = server.SendMsg(response, response.User)
		
	case "create":
		// User wants to create a group
		response := &gochat.Msg{}
		*response = *msg
		response.Cmd = ""
		// Check if they were able to create the group, with themselves as owner
		if ok := groups.Create(msg.To, msg.User); ok {
			// Group was created, add the user to the group and build their response message
			groups.AddUser(msg.To, msg.User)
			response.Msg = fmt.Sprintf("You created the group %s!", msg.To)
			response.Cmd = "create"
		} else {
			// Group already exists on the server
			response.Msg = fmt.Sprintf("Group %s already exists!", msg.To)
		}
		// Send the response message
		err = server.SendMsg(response, response.User)
		
	case "delete":
		// User wants to delete a group
		response := &gochat.Msg{}
		*response = *msg
		response.Cmd = ""
		// Check if the group exists
		if group, ok := groups.Get(msg.To); ok {
			// Check if the user is the owner of the group
			if group.Owner == msg.User {
				response.Msg = fmt.Sprintf("You deleted the group %s!", msg.To)
				response.Cmd = "delete"
				// Notify all other users in the group
				msg.Msg = "has been deleted."
				errCh := make(chan error)
				go server.SendGroupMsg(msg, errCh)
				// Check for errors
				for {
					if err, ok = <- errCh; ok {
						fmt.Println("Group message error:", err)
					} else {
						break
					}
				}
				// delete the group
				groups.Delete(msg.To)
			} else {
				// User is not the owner of the group
				response.Msg = fmt.Sprintf("You don't have permission to delete the group %s!", msg.To)
			}
		} else {
			// Group user wants to delete doesn't exist
			response.Msg = fmt.Sprintf("Group %s doesn't exist!", msg.To)
		}
		// Send the response message
		err = server.SendMsg(response, response.User)
		
	case "disconnect":
		// User has disconnected from the server
		fmt.Printf("Received a d/c from user %s!\n", msg.User)
		// Remove the user from the AddrMap
		if ok := addrs.Remove(msg.User); ok {
			// Remove user from all groups they're in
			for _, groupName := range groups.GroupNames() {
				if _, contains := groups.ContainsUser(groupName, msg.User); contains {
					// Remove the user from the group
					groups.RemoveUser(groupName, msg.User)
					// Notify all users in the group that the user has left
					msg.Msg = fmt.Sprintf("%s has left the group.", msg.User)
					msg.To = groupName
					msg.Cmd = "leave"
					errCh := make(chan error)
					go server.SendGroupMsg(msg, errCh)
					// Check for errors
					for {
						if err, ok = <- errCh; ok {
							fmt.Println("Group message error:", err)
						} else {
							break
						}
					}
				}
			}
		} else {
			fmt.Printf("User %s doesn't exist!\n", msg.User)
		}
	case "kick":
		// User wants to kick someone from a group
		// NOTE: The user to remove will be in msg.Msg
		response := &gochat.Msg{}
		*response = *msg
		response.Cmd = ""
		// Check if the group exists
		if group, ok := groups.Get(msg.To); ok {
			// Check if the user is the owner of the group
			if group.Owner == msg.User {
				// Remove the target user from the group (given by msg.Msg)
				if ok = groups.RemoveUser(msg.To, msg.Msg); ok {
					response.Msg = "" // to denote we don't want to send a response
					// Notify all other users in the group who was kicked (kicked user is no longer in group)
					kickedMsg := &gochat.Msg{}
					*kickedMsg = *msg //shallow copy msg
					kickedMsg.User = msg.Msg
					kickedMsg.Msg = fmt.Sprintf("%s has been kicked from the group.", msg.Msg)
					errCh := make(chan error)
					go server.SendGroupMsg(kickedMsg, errCh)
					// Check for errors
					for {
						if err, ok = <- errCh; ok {
							fmt.Println("Group message error:", err)
						} else {
							break
						}
					}
					// Notify the kicked user with a separate message
					kickedUserMsg := &gochat.Msg{}
					kickedUserMsg.User = msg.Msg
					kickedUserMsg.To = msg.To
					kickedUserMsg.Msg = fmt.Sprintf("[%s] You've been removed from the group.", kickedUserMsg.To)
					kickedUserMsg.Cmd = "leave"
					server.SendMsg(kickedUserMsg, msg.Msg)
				} else {
					// Target user is not in the group
					response.Msg = fmt.Sprintf("User %s isn't in the group %s.", msg.Msg, msg.To)
				}
				
			} else {
				// User is not the owner of the group
				response.Msg = fmt.Sprintf("You don't have permission to remove users from group %s!", msg.To)
			}
		} else {
			// The group doesn't exist on the server
			response.Msg = fmt.Sprintf("Group %s doesn't exist!", msg.To)
		}
		// Send the response message if there was an error
		if response.Msg != "" {
			err = server.SendMsg(response, response.User)
		}
	} // end switch
}

// Wrapper to send a message. Checks if the user has an address
func (server *Server) SendMsg(msg *gochat.Msg, user string)  (err error) {
	if addr, ok := server.Addrs.Get(user); ok {
		return msg.Send(addr.String())
	} else {
		return errors.New(fmt.Sprintf("Address for user %s not found.", user))
	}
}

// Wrapper to send a message to all users of a group
func (server *Server) SendGroupMsg(msg *gochat.Msg, c chan error)  {
	if group, ok := server.Groups.Get(msg.To); ok {
		for _, user := range group.Users.Array() {
			// Don't send the message to the user who wanted it sent
			if user != msg.User {
				// Check if we have an address for the user
				if addr, ok := server.Addrs.Get(user); ok {
					//shallow copy
					response := *msg
					response.Msg = fmt.Sprintf("[%s] %s", msg.To, msg.Msg)
					// send the message
					err := response.Send(addr.String())
					if err != nil {
						// send the error to the channel if we encounter one
						c <- err
					}
				} else {
					// send the error to the channel if we encounter one
					c <- errors.New(fmt.Sprintf("Could not find address for user %s.", user))
					continue
				}
			}
		}
	} else {
		// send the error to the channel if we encounter one
		c <- errors.New(fmt.Sprintf("Group %s doesn't exist.", msg.To))
	}
	// close the channel so the HandleRequest goroutine can continue
	close(c)
}