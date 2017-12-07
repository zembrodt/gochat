# gochat

This project implements a server and client relationship through TCP connections.
Included in this project are four files that make up the actual gochat package, an example
implementation of the client, an example implementation of the server, and an example
implementation of tests.

# gochat package
 - gochat/gochat.go
 - gochat/strset/strset.go
 - gochat/svr/svr.go
 - gochat/clnt/clnt.go
 
# gochat.go
Implements structs needed by both the server and client, which are the structs for Msg,
Addr, and Group. It also implements the threadsafe versions of a map[string]Group and
map[string]Addr called GroupMap and AddrMap.

# strset.go
Implements a StringSet struct out of a map[string]bool. Also implements a thread-safe
version of this called AtomicStringSet.

# svr.go
Implements the Server struct and its corresponding methods. The server needs to be constructed
with what server it will listen on, and then can be started with its Listen() method.

# clnt.go
Implements the Client struct and its corresponding methods. Needs to be constructed with its
username and connected to a server at a given address. It can then run its HandleRequest method
on input until the user wishes to exit, at which point the Disconnect method should be called.
Supported commands:
 join <group>:
	If group exists, user joins that group.
 group <group> <msg>:
	If group exists and user is in it, sends msg to that group.
 leave <group>:
	If group exists and user is in group, they leave the group.
 create <group>:
	If group doesn't exist, creates the group and sets its owner as the user.
 delete <group>:
	If group exists and user is the owner of the group, deletes the group.
 kick <group> <target user>:
	If group exists and user is the owner of the group, removes target user from the group.
 dm <target user>:
	Sends a direct message to the target user.
 groups:
	Displays what groups the user belongs to.
 users <group>:
	Displays what users are in the group.

# Example implementation
 - client.go
 - server.go
 
# client.go
Show how the gochat/clnt might be implemented. Receives the username and address from command
line (with default values) and connects to the server. Will then call HandleRequest every
time the user enters input into the command line, and exit if the user types 'q', 'quit', or 'exit',
calling the Disconnect method.
Example usage:
 go run client.go ryan

# server.go
Shows how the gochat/svr might be implemented. Takes the port to listen on as a command line
argument (with a default value) and creates a Server with it. Will then call the Listen method.
Must be interrupted to exit.
Example usage:
 go run server.go

# Installation
Go can be found at https://golang.org/dl/
After Go is installed, simply run the followning command to install the gochat package:
 go get github.com/zembrodt/gochat
 
#Manual Installation (not using go get):
Move the gochat folder to GOPATH/src. GOPATH can be found with command 'go gopath', but is
generally located in the HOME/go directory, where HOME is your home directory. 
Your GOPATH should now include:
 GOPATH/src/gochat/clnt/clnt.go
 GOPATH/src/gochat/strset/strset.go
 GOPATH/src/gochat/svr/svr.go
 GOPATH/src/gochat/gochat.go
Once the gochat folder is in the src folder, simply install all the package files:
 go install strset
 go install gochat
 go install svr
 go install clnt
Now any Go files can import gochat into their files, such as the client.go and server.go files, which
can be located anywhere.
An easier method is:
 go get github.com/zembrodt/gochat
