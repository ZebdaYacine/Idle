package main

import (
	"log"
	"os"

	"github.com/rqlite/gorqlite"
)

func OpenRqliteFromEnv() *gorqlite.Connection {
	url := os.Getenv("RQLITE_URL")
	if url == "" {
		url = "http://192.168.1.15:4001"
	}
	conn, err := gorqlite.Open(url)
	if err != nil {
		log.Fatal(err)
	}
	return conn
}
