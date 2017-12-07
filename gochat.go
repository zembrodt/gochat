/*
Package gochat has several useful implementations in the overall gochat application.

The Msg struct contains all information that is neccessary for the exchangement between
clients and a server.

*/
package gochat

import (
	"fmt"
	"net"
	"sync"
	"encoding/gob"
	"github.com/zembrodt/gochat/strset"
)

// A message is broken into 4 parts
// User: The user sending the message
// To:   Who we're sending that message to
// Msg:  The contents of the message
// Cmd:  The command we'll execute on the server
type Msg struct {
	User, To, Msg, Cmd string
}

type Addr struct {
	Address, Port string
}

// Defined who owns a group and what users are in the group. Needed for GroupMap
type Group struct {
	Owner string
	Users *strset.AtomicStringSet
}

// Keeps track of an Addr for each user. Thread-safe
type AddrMap struct {
    v map[string]Addr
    lock sync.RWMutex // can be held by an arbitrary amount of readers and one writer
}

// Keeps track of each group's owner and users. Thread-safe
type GroupMap struct {
	v map[string]Group
    lock sync.RWMutex
}

// Sends a message to the given address
func (msg *Msg) Send(addr string) (err error) {
	// Dial a connect to remote client
	conn, err := net.Dial("tcp", addr)
	defer conn.Close()
	if err != nil {
		return err
	}
	// Set up a new encoder to send the msg as a gob
	encoder := gob.NewEncoder(conn)
	err = encoder.Encode(msg) // actually sends the message
	if err != nil {
		return err
	}
	return nil
}

// Decodes a message from the given connection
func (msg *Msg) Retrieve(conn net.Conn) (err error) {
	// Set up a decoder to get the message from the connection
	// The decoder will block until it has received the full gob
	decoder := gob.NewDecoder(conn)
    err = decoder.Decode(msg) // decodes the message into msg
    if err != nil {
        return err
    }
	return nil
}

// Converts an Addr to a string
func (addr *Addr) String() (string) {
	return fmt.Sprintf("%s:%s", addr.Address, addr.Port)
}

// Constructor function for AddrMap
func NewAddrMap() *AddrMap {
	return &AddrMap{v: make(map[string]Addr)}
}

// Returns the Addr associated with the given user, and a boolean if that user exists
func (addrMap *AddrMap) Get(user string) (addr Addr, ok bool) {
	addrMap.lock.RLock()
	addr, ok = addrMap.v[user]
	addrMap.lock.RUnlock()
	return
}

// Adds an entry into the AddrMap unless the user already exists, which will return false
func (addrMap *AddrMap) Add(user string, addr Addr) (ok bool) {
	addrMap.lock.RLock()
	_, ok = addrMap.v[user]
	addrMap.lock.RUnlock()
	if !ok {
		addrMap.lock.Lock()
		addrMap.v[user] = addr
		addrMap.lock.Unlock()
	}
	return !ok
}

// Removes the given user from the AddrMap if they exist
func (addrMap *AddrMap) Remove(user string) (ok bool) {
	// Check that the map contains the user, so if it doesn't we're only having to use
	// a read lock and not a write lock.
	addrMap.lock.RLock()
	_, ok = addrMap.v[user]
	addrMap.lock.RUnlock()
	if ok {
		addrMap.lock.Lock()
		delete(addrMap.v, user)
		addrMap.lock.Unlock()
	}
	return
}

// Constructor function for GroupMap
func NewGroupMap() *GroupMap {
	return &GroupMap{v: make(map[string]Group)}
}

// Returns the Group associated with the given group name, and a boolean if that group exists
func (groupMap *GroupMap) Get(groupId string) (group Group, ok bool) {
	groupMap.lock.RLock()
	group, ok = groupMap.v[groupId]
	groupMap.lock.RUnlock()
	return
}

// Adds a user to the given group. Returns false if group doesn't exist
func (groupMap *GroupMap) AddUser(group, user string) (ok bool) {
	groupMap.lock.RLock()
	if _, ok = groupMap.v[group]; ok {
		ok = groupMap.v[group].Users.Contains(user)
		ok = !ok
	}
	groupMap.lock.RUnlock()
	if ok {
		groupMap.lock.Lock() //RW lock
		groupMap.v[group].Users.Add(user)
		groupMap.lock.Unlock()
	}
	return
}

// Removes the user from the given group. Returns false if the group doesn't exist
func (groupMap *GroupMap) RemoveUser(group, user string) (ok bool) {
	groupMap.lock.RLock()
	if _, ok = groupMap.v[group]; ok {
		ok = groupMap.v[group].Users.Contains(user)
	}
	groupMap.lock.RUnlock()
	if ok {
		groupMap.lock.Lock()
		groupMap.v[group].Users.Remove(user)
		groupMap.lock.Unlock()
	}
	return
}

// Returns two booleans, first is if the given group contains the user.
// Second boolean is if the group exists.
func (groupMap *GroupMap) ContainsUser(group, user string) (contains, ok bool) {
	groupMap.lock.RLock()
	if _, ok = groupMap.v[group]; ok {
		contains = groupMap.v[group].Users.Contains(user)
	}
	groupMap.lock.RUnlock()
	return
}

// Creates a group with the given name and owner. Returns false if group exists.
func (groupMap *GroupMap) Create(group, owner string) (ok bool) {
	groupMap.lock.RLock()
	_, ok = groupMap.v[group]
	groupMap.lock.RUnlock()
	if !ok {
		groupMap.lock.Lock()
		groupMap.v[group] = Group{owner, strset.NewAtomicStringSet()}
		//groupMap.v[group].Users.Add(owner)
		groupMap.lock.Unlock()
	}
	return !ok
}

// Removes the given group from the GroupMap
// Returns false if group doesn't exist
func (groupMap *GroupMap) Delete(group string) (ok bool) {
	groupMap.lock.RLock()
	_, ok = groupMap.v[group]
	groupMap.lock.RUnlock()
	if ok {
		groupMap.lock.Lock()
		delete(groupMap.v, group)
		groupMap.lock.Unlock()
	}
	return
}

// Converts the keys of the map into a string slice.
func (groupMap *GroupMap) GroupNames() (groupNames []string) {
	groupMap.lock.RLock()
	for groupName, _ := range groupMap.v {
		groupNames = append(groupNames, groupName)
	}
	groupMap.lock.RUnlock()
	return
}