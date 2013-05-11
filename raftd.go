package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/benbjohnson/go-raft"
	"github.com/gorilla/mux"
	"log"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"os"
	"time"
)

//------------------------------------------------------------------------------
//
// Initialization
//
//------------------------------------------------------------------------------

var verbose bool

func init() {
	flag.BoolVar(&verbose, "v", false, "verbose logging")
	flag.BoolVar(&verbose, "verbose", false, "verbose logging")
}

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
	if verbose {
		fmt.Println("Verbose logging enabled.\n")
	}

	// Use the present working directory if a directory was not passed in.
	var path string
	if flag.NArg() == 0 {
		path, _ = os.Getwd()
	} else {
		path = flag.Arg(0)
		if err := os.MkdirAll(path, 0744); err != nil {
			fatal("Unable to create path: %v", err)
		}
	}

	// Read server info from file or grab it from user.
	var info *Info = getInfo(path)
	name := fmt.Sprintf("%s:%d", info.Host, info.Port)
	fmt.Printf("Name: %s\n\n", name)
	
	// Setup new raft server.
	server, err = raft.NewServer(name, path)
	server.DoHandler = DoHandler;
	server.AppendEntriesHandler = AppendEntriesHandler;
	server.RequestVoteHandler = RequestVoteHandler;
	server.SetElectionTimeout(2 * time.Second)
	server.SetHeartbeatTimeout(1 * time.Second)
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

	// Create HTTP interface.
    r := mux.NewRouter()
    r.HandleFunc("/join", JoinHttpHandler).Methods("POST")
    r.HandleFunc("/vote", VoteHttpHandler).Methods("POST")
    r.HandleFunc("/log", GetLogHttpHandler).Methods("GET")
    r.HandleFunc("/log/append", AppendEntriesHttpHandler).Methods("POST")
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
		info.Host = strings.TrimSpace(info.Host)
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
			fatal("Unable to write info to file: %v", err)
		}
	}
	
	return info
}


//--------------------------------------
// Handlers
//--------------------------------------

// Forwards requests to the leader.
func DoHandler(server *raft.Server, peer *raft.Peer, _command raft.Command) error {
	if command, ok := _command.(*raft.JoinCommand); ok {
		var b bytes.Buffer
		json.NewEncoder(&b).Encode(command)
		debug("[send] POST http://%v/join", peer.Name())
		resp, err := http.Post(fmt.Sprintf("http://%s/join", peer.Name()), "application/json", &b)
		if resp != nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		return fmt.Errorf("raftd: Unable to join: %v", err)
	}
	return fmt.Errorf("raftd: Unsupported command: %v", _command)
}

// Sends AppendEntries RPCs to a peer when the server is the leader.
func AppendEntriesHandler(server *raft.Server, peer *raft.Peer, req *raft.AppendEntriesRequest) (*raft.AppendEntriesResponse, error) {
	var aersp *raft.AppendEntriesResponse
	var b bytes.Buffer
	json.NewEncoder(&b).Encode(req)
	debug("[send] POST http://%s/log/append [%d]", peer.Name(), len(req.Entries))
	resp, err := http.Post(fmt.Sprintf("http://%s/log/append", peer.Name()), "application/json", &b)
	if resp != nil {
		aersp = &raft.AppendEntriesResponse{}
		if err = json.NewDecoder(resp.Body).Decode(&aersp); err == nil || err == io.EOF {
			return aersp, nil
		}
	}
	return aersp, fmt.Errorf("raftd: Unable to append entries: %v", err)
}

// Sends RequestVote RPCs to a peer when the server is the candidate.
func RequestVoteHandler(server *raft.Server, peer *raft.Peer, req *raft.RequestVoteRequest) (*raft.RequestVoteResponse, error) {
	var rvrsp *raft.RequestVoteResponse
	var b bytes.Buffer
	json.NewEncoder(&b).Encode(req)
	debug("[send] POST http://%s/vote", peer.Name())
	resp, err := http.Post(fmt.Sprintf("http://%s/vote", peer.Name()), "application/json", &b)
	if resp != nil {
		rvrsp := &raft.RequestVoteResponse{}
		if err = json.NewDecoder(resp.Body).Decode(&rvrsp); err == nil || err == io.EOF {
			return rvrsp, nil
		}
	}
	return rvrsp, fmt.Errorf("raftd: Unable to request vote: %v", err)
}

//--------------------------------------
// HTTP Handlers
//--------------------------------------

func GetLogHttpHandler(w http.ResponseWriter, req *http.Request) {
	debug("[recv] GET http://%v/log", server.Name())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(server.LogEntries())
}

func JoinHttpHandler(w http.ResponseWriter, req *http.Request) {
	debug("[recv] POST http://%v/join", server.Name())
	command := &raft.JoinCommand{}
	if err := decodeJsonRequest(req, command); err == nil {
		if err = server.Do(command); err != nil {
			warn("raftd: Unable to join: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	} else {
		warn("[join] ERROR: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func VoteHttpHandler(w http.ResponseWriter, req *http.Request) {
	rvreq := &raft.RequestVoteRequest{}
	err := decodeJsonRequest(req, rvreq)
	if err == nil {
		debug("[recv] POST http://%v/vote [%s]", server.Name(), rvreq.CandidateName)
		if resp, _ := server.RequestVote(rvreq); resp != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}
	w.WriteHeader(http.StatusInternalServerError)
}

func AppendEntriesHttpHandler(w http.ResponseWriter, req *http.Request) {
	aereq := &raft.AppendEntriesRequest{}
	err := decodeJsonRequest(req, aereq)
	if err == nil {
		debug("[recv] POST http://%s/log/append [%d]", server.Name(), len(aereq.Entries))
		if resp, _ := server.AppendEntries(aereq); resp != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}
	warn("[append] ERROR: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
}


//--------------------------------------
// HTTP Utilities
//--------------------------------------

func decodeJsonRequest(req *http.Request, data interface{}) error {
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&data); err != nil && err != io.EOF {
		logger.Println("Malformed json request: %v", err)
		return fmt.Errorf("Malformed json request: %v", err)
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
// Log
//--------------------------------------

func debug(msg string, v ...interface{}) {
	if verbose {
		logger.Printf("DEBUG " + msg + "\n", v...)
	}
}

func info(msg string, v ...interface{}) {
	logger.Printf("INFO  " + msg + "\n", v...)
}

func warn(msg string, v ...interface{}) {
	logger.Printf("WARN  " + msg + "\n", v...)
}

func fatal(msg string, v ...interface{}) {
	logger.Printf("FATAL " + msg + "\n", v...)
	os.Exit(1)
}
