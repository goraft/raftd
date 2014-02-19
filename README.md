raftd
=====

## Overview

The raftd server is a reference implementation for using the [goraft](https://github.com/goraft/raft) library.
This library provides a distributed consensus protocol based on the Raft protocol as described by Diego Ongaro and John Ousterhout in their paper, [In Search of an Understandable Consensus Algorithm](https://ramcloud.stanford.edu/wiki/download/attachments/11370504/raft.pdf).
This protocol is based on Paxos but is architected to be more understandable.
It is similar to other log-based distributed consensus systems such as [Google's Chubby](https://www.google.com/url?sa=t&rct=j&q=&esrc=s&source=web&cd=1&ved=0CDAQFjAA&url=http%3A%2F%2Fresearch.google.com%2Farchive%2Fchubby.html&ei=i9OGUerTJKbtiwLkiICoCQ&usg=AFQjCNEmFWlaB_iXQfEjMcMwPaYTphO6bA&sig2=u1vefM2ZOZu_ZVIZGynt1A&bvm=bv.45960087,d.cGE) or [Heroku's doozerd](https://github.com/ha/doozerd).

This reference implementation is very simple.
It is a key/value database with the following HTTP API:

```
# Set the value of a key.
$ curl -X POST http://localhost:4001/db/my_key -d 'FOO'
```

```
# Retrieve the value for a given key.
$ curl http://localhost:4001/db/my_key
FOO
```

All the values sent to the leader will be propagated to the other servers in the cluster.
This reference implementation does not support command forwarding.
If you try to send a change to a follower then it will simply be denied.


## Running

First, install raftd:

```sh
$ go get github.com/goraft/raftd
```

To start the first node in your cluster, simply specify a port and a directory where the data will be stored:

```sh
$ raftd -p 4001 ~/node.1
```

To add nodes to the cluster, you'll need to start on a different port and use a different data directory.
You'll also need to specify the host/port of the leader of the cluster to join:

```sh
$ raftd -p 4002 -join localhost:4001 ~/node.2
```

When you restart the node, it's already been joined to the cluster so you can remove the `-join` argument.

Finally, you can add one more node:

```sh
$ raftd -p 4003 -join localhost:4001 ~/node.3
```

Now when you set values to the leader:

```sh
$ curl -XPOST localhost:4001/db/foo -d 'bar'
```

The values will be propagated to the followers:

```sh
$ curl localhost:4001/db/foo
bar
$ curl localhost:4002/db/foo
bar
$ curl localhost:4003/db/foo
bar
```

Killing the leader will automatically elect a new leader.
If you kill and restart the first node and try to set a value you'll receive:

```sh
$ curl -XPOST localhost:4001/db/foo -d 'bar'
raft.Server: Not current leader
```

Leader forwarding is not implemented in the reference implementation.


## FAQ

### Why is command forwarding not implemented?

Command forwarding is a nice feature to have because it allows a client to send a command to any server and have it pushed to the current leader. This lets your client code stay simple. However, now you have an additional point of failure in your remote call. If the intermediate server crashes while delivering the command then your client will still need to know how to retry its command. Since this retry logic needs to be in your client code, adding command forwarding doesn't provide any benefit.

### Why isn't feature X implemented?

Raftd is meant to be a basic reference implementation. As such, it's aim is to provide the smallest, simplest implementation required to get someone off the ground and using go-raft in their project. If you have questions on how to implement a given feature, please add a Github Issue and we can provide instructions in this README.


## Debugging

If you want to see more detail then you can specify several options for logging:

```
-v       Enables verbose raftd logging.
-debug   Enables debug-level raft logging.
-trace   Enables trace-level raft logging.
```

If you're having an issue getting `raftd` running, the `-debug` and `-trace` options can be really useful.


## Caveats

One issue with running a 2-node distributed consensus protocol is that we need both servers operational to make a quorum and to perform an actions on the server.
So if we kill one of the servers at this point, we will will not be able to update the system (since we can't replicate to a majority).
You will need to add additional nodes to allow failures to not affect the system.
For example, with 3 nodes you can have 1 node fail.
With 5 nodes you can have 2 nodes fail.

