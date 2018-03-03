package main

import (
	"sync"
	"fmt"
	"time"
)

type DB struct {
	mu sync.RWMutex
	data map[string]string
}

func Create() (*DB) {
	db := &DB{
		data : make(map[string]string),
	}
	return db
}

type Tx struct {
	db *DB
	writable bool
}

func (tx *Tx) lock() {
	if tx.writable {
		tx.db.mu.Lock()
	} else {
		tx.db.mu.RLock()
	}
}

func (tx *Tx) unlock() {
	if tx.writable {
		tx.db.mu.Unlock()
	} else {
		tx.db.mu.RUnlock()
	}
}

func (tx *Tx) Set(key, value string) {
	fmt.Println("Setting value... " , key , value)
	tx.db.data[key] = value
}

func (tx *Tx) Get(key string) string {
	return tx.db.data[key]
}

func (db * DB) View(fn func (tx *Tx) error) error {
	return db.managed(false, fn)
}

func (db * DB) Update(fn func (tx *Tx) error) error {
	return db.managed(true, fn)
}

func (db *DB) Begin(writable bool) (*Tx,error) {
	tx := &Tx {
		db : db,
		writable: writable,
	}
	tx.lock()

	return tx,nil
}

func (db *DB) managed(writable bool, fn func(tx *Tx) error) (err error) {
	var tx *Tx
	tx, err = db.Begin(writable)
	if err != nil {
		return
	}

	defer func() {
		if writable {
			fmt.Println("Write Unlocking...")
			tx.unlock()
		} else {
			fmt.Println("Read Unlocking...")
			tx.unlock()
		}
	}()

	err = fn(tx)
	return
}

func main() {

	db := Create()

	go db.Update(func(tx *Tx) error {
		tx.Set("mykey", "go")
		tx.Set("mykey2", "is")
		tx.Set("mykey3", "awesome")
		return nil
	})

	go db.View(func(tx *Tx) error {
		fmt.Println("value is")
		fmt.Println(tx.Get("mykey3"))
		return nil
	})

	time.Sleep(20000000000)
}
