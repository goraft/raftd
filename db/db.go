package db

import (
    "sync"
)

// The key-value database.
type DB struct {
    data       map[string]string
    mutex      sync.RWMutex
}

// Creates a new database.
func New() *DB {
    return &DB{
        data: make(map[string]string),
    }
}

// Retrieves the value for a given key.
func (db *DB) Get(key string) string {
    db.mutex.RLock()
    defer db.mutex.RUnlock()
    return db.data[key]
}

// Sets the value for a given key.
func (db *DB) Put(key string, value string) {
    db.mutex.Lock()
    defer db.mutex.Unlock()
    db.data[key] = value
}
