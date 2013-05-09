package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/benbjohnson/go-raft"
	"github.com/gorilla/mux"
	"log"
	"io"
	"io/ioutil"
	"net/http"
	"os"
)

//------------------------------------------------------------------------------
//
// Typedefs
//
//------------------------------------------------------------------------------

type Info struct {
	Host string `json:"host"`
	Port int `json:"port"`
}

//------------------------------------------------------------------------------
//
// Variables
//
//------------------------------------------------------------------------------

var server *raft.Server
var logger *log.Logger

//------------------------------------------------------------------------------
//
// Functions
//
//------------------------------------------------------------------------------

//--------------------------------------
// Main
//--------------------------------------

func main() {
	var err error
	logger = log.New(os.Stdout, "", log.LstdFlags)
	flag.Parse()

	// Use the present working directory if a directory was not passed in.
	var path string
	if flag.NArg() == 0 {
		path, _ = os.Getwd()
	} else {
		path = flag.Arg(0)
	}

	// Read server info from file or grab it from user.
	var info *Info = getInfo(path)
	name := fmt.Sprintf("%s:%d\n", info.Host, info.Port)
	fmt.Printf("Name: %s\n", name)
	
	// Setup new raft server.
	server, err = raft.NewServer(name, path)
	if err != nil {
		fatal("%v", err)
	}
	server.Start()

	// Join to another server if we don't have a log.
	if server.IsLogEmpty() {
		var leaderHost string
		fmt.Println("This server has no log. Please enter a server in the cluster to join\nto or hit enter to initialize a cluster.");
		fmt.Printf("Join to (host:port)> ");
		fmt.Scanf("%s", &leaderHost)
		if leaderHost == "" {
			server.Join(server.Name())
		} else {
			server.Join(leaderHost)
		}
	}

	// TODO: Setup handler functions.

	// Create HTTP interface.
    r := mux.NewRouter()
    r.HandleFunc("/join", JoinHandler).Methods("POST")
    r.HandleFunc("/log", GetLogHandler).Methods("GET")
    http.Handle("/", r)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", info.Port), nil))
}

func usage() {
	fatal("usage: raftd [PATH]")
}

//--------------------------------------
// Config
//--------------------------------------

func getInfo(path string) *Info {
	info := &Info{}

	// Read in the server info if available.
	infoPath := fmt.Sprintf("%s/info", path)
	if file, err := os.Open(infoPath); err == nil {
		if content, err := ioutil.ReadAll(file); err != nil {
			fatal("Unable to read info: %v", err)
		} else {
			if err = json.Unmarshal(content, &info); err != nil {
				fatal("Unable to parse info: %v", err)
			}
		}
		file.Close()
	
	// Otherwise ask user for info and write it to file.
	} else {
		fmt.Printf("Enter hostname: [localhost] ");
		fmt.Scanf("%s", &info.Host)
		if info.Host == "" {
			info.Host = "localhost"
		}

		fmt.Printf("Enter port: [4001] ");
		fmt.Scanf("%d", &info.Port)
		if info.Port == 0 {
			info.Port = 4001
		}

		// Write to file.
		content, _ := json.Marshal(info)
		content = []byte(string(content) + "\n")
		if err := ioutil.WriteFile(infoPath, content, 0644); err != nil {
			fatal("Unable to write info to file")
		}
	}
	
	return info
}


//--------------------------------------
// HTTP Handlers
//--------------------------------------

func GetLogHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(server.LogEntries())
}

func JoinHandler(w http.ResponseWriter, req *http.Request) {
	command := &raft.JoinCommand{}
	if err := decodeJsonRequest(req, command); err != nil {
		server.Do(command)
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}


//--------------------------------------
// HTTP Utilities
//--------------------------------------

func decodeJsonRequest(req *http.Request, data interface{}) error {
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&data); err != nil && err != io.EOF {
		logger.Println("Malformed json request.")
		return errors.New("Malformed json request.")
	}
	return nil
}

func encodeJsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data != nil {
		encoder := json.NewEncoder(w)
		encoder.Encode(data)
	}
}

//--------------------------------------
// Utility
//--------------------------------------

// Writes to standard error.
func warn(msg string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", v...)
}

// Writes to standard error and dies.
func fatal(msg string, v ...interface{}) {
	warn(msg, v)
	os.Exit(1)
}
