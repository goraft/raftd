package main

import (
	"flag"
	"github.com/benbjohnson/raftd/command"
	"github.com/benbjohnson/raftd/server"
	"github.com/goraft/raft"
	"log"
	"math/rand"
	"os"
	"time"
)

var verbose bool
var debug bool
var host string
var port int
var join string

func init() {
	flag.BoolVar(&verbose, "v", false, "verbose logging")
	flag.BoolVar(&debug, "debug", false, "Raft debugging")
	flag.StringVar(&host, "h", "localhost", "hostname")
	flag.IntVar(&port, "p", 4001, "port")
	flag.StringVar(&join, "join", "", "host:port of leader to join")
}

func main() {
	log.SetFlags(0)
	flag.Parse()
	if verbose {
		log.Print("Verbose logging enabled.")
	}
	if debug {
		raft.SetLogLevel(raft.Debug)
		log.Print("Raft debugging enabled.")
	}

    rand.Seed(time.Now().UnixNano())

	// Setup commands.
	raft.RegisterCommand(&command.WriteCommand{})

	// Set the data directory.
	if flag.NArg() == 0 {
		log.Fatal("Data path argument required")
	}
	path := flag.Arg(0)
	if err := os.MkdirAll(path, 0744); err != nil {
		log.Fatalf("Unable to create path: %v", err)
	}

	log.SetFlags(log.LstdFlags)
	s := server.New(path, host, port)
	log.Fatal(s.ListenAndServe(join))
}
