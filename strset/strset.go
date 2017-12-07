/*
Package strset implements a set of strings out of Go's map type.

A thread-safe, atomic version of the sets is also provided.
Both implementations must be accessed by the defined functions as the structs' fields
are not exported.
*/
package strset

import "sync"

// We just care about the key's value, so have the value we're mapping to be something
// simple, such as bool
type StringSet struct {
	set map[string]bool
}

// Thread-safe wrapper for StringSet
type AtomicStringSet struct {
	set *StringSet
	lock sync.RWMutex // RWMutex allows multiple readers but only one writer
}

// Constructor for StringSet
func NewStringSet() *StringSet {
	return &StringSet{set: make(map[string]bool)}
}

// Adds a new key to the map. Returns true if the value already existed
func (set *StringSet) Add(s string) (found bool) {
	_, found = set.set[s]
	set.set[s] = true
	return !found
}

// Returns a bool whether or not the string exists as a key in the map
func (set *StringSet) Contains(s string) (found bool) {
	_, found = set.set[s]
	return
}

// Deletes the key from the map. Does not check if the key exists, as it is not required
// for delete.
func (set *StringSet) Remove(s string) {
	delete(set.set, s)
}

// Converts the map's keys into a string slice
func (set *StringSet) Array() (s []string) {
	for k, _ := range set.set {
		s = append(s, k)
	}
	return
}

// Constructor fo AtomicStringSet
func NewAtomicStringSet() *AtomicStringSet {
	return &AtomicStringSet{set: NewStringSet()}
}

func (set *AtomicStringSet) Add(s string) (found bool) {
	set.lock.Lock()
	found = set.set.Add(s)
	set.lock.Unlock()
	return found
}

func (set *AtomicStringSet) Contains(s string) (found bool) {
	set.lock.RLock()
	found = set.set.Contains(s)
	set.lock.RUnlock()
	return
}

func (set *AtomicStringSet) Remove(s string) (found bool) {
	// This version checks if the StringSet contains the key so as to not wait for a write
	// lock if it doesn't
	set.lock.RLock()
	found = set.set.Contains(s)
	set.lock.RUnlock()
	if found {
		set.lock.Lock()
		set.set.Remove(s)
		set.lock.Unlock()
	}
	return
}

func (set *AtomicStringSet) Array() (s []string) {
	set.lock.RLock()
	s = set.set.Array()
	set.lock.RUnlock()
	return
}