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
	name := fmt.Sprintf("%s:%d", info.Host, info.Port)
	fmt.Printf("Name: %s\n\n", name)
	
	// Setup new raft server.
	server, err = raft.NewServer(name, path)
	server.DoHandler = DoHandler;
	server.AppendEntriesHandler = AppendEntriesHandler;
	server.RequestVoteHandler = RequestVoteHandler;
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
			fatal("Unable to write info to file")
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
		resp, err := http.Post(fmt.Sprintf("http://%s/join", peer.Name()), "application/json", &b)
		status("joining %v", b.String())
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
	status("append -> %v %s : %v", peer.Name(), b.String())
	resp, err := http.Post(fmt.Sprintf("http://%s/log/append", peer.Name()), "application/json", &b)
	if resp != nil {
		aersp = &raft.AppendEntriesResponse{}
		if err = json.NewDecoder(resp.Body).Decode(&aersp); err == nil || err == io.EOF {
			warn(">> %v", aersp)
			return aersp, nil
		}
	}
	warn("raftd: Unable to append entries [%s]: %v", peer.Name(), err)
	return aersp, fmt.Errorf("raftd: Unable to append entries: %v", err)
}

// Sends RequestVote RPCs to a peer when the server is the candidate.
func RequestVoteHandler(server *raft.Server, peer *raft.Peer, req *raft.RequestVoteRequest) (*raft.RequestVoteResponse, error) {
	status("request_vote -> %v", peer.Name())
	var rvrsp *raft.RequestVoteResponse
	var b bytes.Buffer
	json.NewEncoder(&b).Encode(req)
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(server.LogEntries())
}

func JoinHttpHandler(w http.ResponseWriter, req *http.Request) {
	command := &raft.JoinCommand{}
	if err := decodeJsonRequest(req, command); err == nil {
		status("[join] %v", command.Name)
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
	status("POST /vote")
	rvreq := &raft.RequestVoteRequest{}
	err := decodeJsonRequest(req, rvreq)
	if err == nil {
		if resp, err := server.RequestVote(rvreq); err == nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		}
	}
	warn("[vote] ERROR: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
}

func AppendEntriesHttpHandler(w http.ResponseWriter, req *http.Request) {
	status("POST /log/append")
	aereq := &raft.AppendEntriesRequest{}
	err := decodeJsonRequest(req, aereq)
	if err == nil {
		if resp, err := server.AppendEntries(aereq); err == nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		} else {
			warn("err! %v", err)
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
// Utility
//--------------------------------------

// Writes the current status to the command line.
func status(msg string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\r", v...)
}

// Writes to standard error.
func warn(msg string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", v...)
}

// Writes to standard error and dies.
func fatal(msg string, v ...interface{}) {
	warn(msg, v)
	os.Exit(1)
}
