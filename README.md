raftd
=====

## Overview

The raftd server is a reference implementation for using the [go-raft](https://github.com/benbjohnson/go-raft) library.
This library provides a distributed consensus protocol based on the Raft protocol as described by Diego Ongaro and John Ousterhout in their paper, [In Search of an Understandable Consensus Algorithm](https://ramcloud.stanford.edu/wiki/download/attachments/11370504/raft.pdf).
This protocol is based on Paxos but is architected to be more understandable.
It is similar to other log-based distributed consensus systems such as [Google's Chubby](https://www.google.com/url?sa=t&rct=j&q=&esrc=s&source=web&cd=1&ved=0CDAQFjAA&url=http%3A%2F%2Fresearch.google.com%2Farchive%2Fchubby.html&ei=i9OGUerTJKbtiwLkiICoCQ&usg=AFQjCNEmFWlaB_iXQfEjMcMwPaYTphO6bA&sig2=u1vefM2ZOZu_ZVIZGynt1A&bvm=bv.45960087,d.cGE) or [Heroku's doozerd](https://github.com/ha/doozerd).


## Running

To run raftd, create a new directory and start the server with the server's name (host + port).
For example, we can start our first raftd node like this:

```sh
$ mkdir ~/raftd.1
$ cd ~/raftd.1
$ raftd
Enter host:port> localhost:4001
Initialize or join? [ij] i
New raftd cluster created.
```

The server uses the present working directory as its storage so make sure you start with a directory specifically for your server instance.
Here we're making a directory for our first node and then starting the node from that directory.
If the node dies then you can restart it from this same directory but make sure you start it with the same name.

This node will be operational by itself but will not replicate to any other nodes (because they don't exist yet).
To add more nodes, simply start the server from new directories.

```sh
$ mkdir ~/raftd.2
$ cd ~/raftd.2
$ raftd
Enter host:port> localhost:4002
Initialize or join? [ij] j
Server to join: localhost:4001
Joining cluster... joined.
```

This will start a node named `localhost:4002` that will attempt to connect to the first node we started.
Now when we make changes to either one of these nodes, the other node will replicate the changes.

IMPORTANT NOTE: One caveat with running a 2-node distributed consensus protocol is that we need both servers operational to make a quorum and to perform an actions on the server.
So if we kill one of the servers at this point, we will will not be able to update the system (since we can't replicate to a majority).
You will need to add additional nodes to allow failures to not affect the system.
For example, with 3 nodes you can have 1 node fail.
With 5 nodes you can have 2 nodes fail.

