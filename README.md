raftd
=====

## Overview

The raftd server is a reference implementation for using the [go-raft](https://github.com/benbjohnson/go-raft) library.
This library provides a distributed consensus protocol based on the Raft protocol as described by Diego Ongaro and John Ousterhout in their paper, [In Search of an Understandable Consensus Algorithm](https://ramcloud.stanford.edu/wiki/download/attachments/11370504/raft.pdf).
This protocol is based on Paxos but is architected to be more understandable.
It is similar to other log-based distributed consensus systems such as [Google's Chubby](https://www.google.com/url?sa=t&rct=j&q=&esrc=s&source=web&cd=1&ved=0CDAQFjAA&url=http%3A%2F%2Fresearch.google.com%2Farchive%2Fchubby.html&ei=i9OGUerTJKbtiwLkiICoCQ&usg=AFQjCNEmFWlaB_iXQfEjMcMwPaYTphO6bA&sig2=u1vefM2ZOZu_ZVIZGynt1A&bvm=bv.45960087,d.cGE) or [Heroku's doozerd](https://github.com/ha/doozerd).

This reference implementation is very simple.
It allows servers to join the cluster, fail over and will replicate values in local files.
The server runs out of a directory that you provide.
The directory will store the Raft log, some basic info about the server name and port, and any files that are being replicated.

## Running

First, install raftd:

```sh
$ go get github.com/benbjohnson/raftd
```

Then run `raftd` and provide a directory to store the data.
The directory will be created if it doesn't already exist.
The server will ask for a hostname and a port which will identify the node in the cluster.
This will be replicated to other nodes and this will be how they contact this server.
Since our example is local we'll use local host and a random port:

```sh
$ raftd ~/node.1
Enter hostname: [localhost] 
Enter port: [4001] 
Name: localhost:4001

This server has no log. Please enter a server in the cluster to join
to or hit enter to initialize a cluster.
Join to (host:port)> 
```

We'll go with the default `localhost:4001` and we won't specify a server to join since this is the first node.
Next we'll add a second node in the same way but a different data directory.
Make sure you use a different port and you specify the name of the first server when joining:

```sh
$ raftd ~/node.2
Enter hostname: [localhost] 
Enter port: [4001] 4002
Name: localhost:4002

This server has no log. Please enter a server in the cluster to join
to or hit enter to initialize a cluster.
Join to (host:port)> localhost:4001
```

You can also enable verbose logging to see the communication between nodes by using the `-v` or `--verbose` flags:

```sh
$ raftd -v ~/node.2
```

The `raftd` implementation uses logger election timeouts (2 seconds) and heartbeats (1 second) than the defaults so you can see what's going on.


## API

To write a value to a file, use the `POST /files/:filename` endpoint and pass the value as the POST body:

```sh
$ curl -XPOST http://localhost:4001/files/foo -d "BAR"
```

To read the value back, use the `GET /files/:filename` endpoint:

```sh
$ curl http://localhost:4001/files/foo
BAR
```

Currently you need to send write to the leader to be able to update the value.


## Internals

If you want to get a peek at the internals, check out the code!
Everything was intentionally put into a single file so it's easy to find things.
In a production system, please don't put all your code in one file.

The `go-raft` library serializes to a log using JSON so it's easy to see what's in the log.
To check out the log, use the `cat` command:

```sh
$ cat /tmp/node.1/log
76e1500f 0000000000000001 0000000000000002 raft:join {"name":"localhost:4001"}
02502ad1 0000000000000002 0000000000000002 raft:join {"name":"localhost:4002"}
28b5397b 0000000000000003 0000000000000002 file:write {"filename":"foo","content":"BAR"}
```

The log has multiple fixed width columns and then the command name and the command data.
The first column is a checksum to protect against a corrupted log file.
The second column is the monotonically increasing index of the log.
The third column is the election term that the log entry was created in.
There is no specific format for the command names except that they can't include a space.
The internal raft commands (e.g. `raft:join`) are prefixed with `raft:`.


## Caveats

One issue with running a 2-node distributed consensus protocol is that we need both servers operational to make a quorum and to perform an actions on the server.
So if we kill one of the servers at this point, we will will not be able to update the system (since we can't replicate to a majority).
You will need to add additional nodes to allow failures to not affect the system.
For example, with 3 nodes you can have 1 node fail.
With 5 nodes you can have 2 nodes fail.

