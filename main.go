package main

import (
    "flag"
    "strings"
    "github.com/coreos/etcd/raft/raftpb"
)

/*
# Start 3 node cluster
./dgraph --id 1 --cluster http://127.0.0.1:12379,http://127.0.0.1:22379,http://127.0.0.1:32379 --port 12380
./dgraph --id 2 --cluster http://127.0.0.1:12379,http://127.0.0.1:22379,http://127.0.0.1:32379 --port 22380
./dgraph --id 3 --cluster http://127.0.0.1:12379,http://127.0.0.1:22379,http://127.0.0.1:32379 --port 32380

# Add 4th node
curl -L http://127.0.0.1:12380/4 -XPOST -d http://127.0.0.1:42379
# Start 4tn node
./dgraph --id 4 --cluster http://127.0.0.1:12379,http://127.0.0.1:22379,http://127.0.0.1:32379,http://127.0.0.1:42379 --port 42380 --join

# Remove 3rd Node.
curl -L http://127.0.0.1:12380/3 -XDELETE

# Add vertices
curl -L http://127.0.0.1:12380/addvertex -XPUT -d SFO
curl -L http://127.0.0.1:12380/addvertex -XPUT -d LA
curl -L http://127.0.0.1:12380/addvertex -XPUT -d NY
curl -L http://127.0.0.1:12380/addvertex -XPUT -d FL

# Add edges
curl -L http://127.0.0.1:12380/addedge -XPUT -d SFO,LA
curl -L http://127.0.0.1:12380/addedge -XPUT -d LA,NY
curl -L http://127.0.0.1:12380/addedge -XPUT -d NY,FL
curl -L http://127.0.0.1:12380/addedge -XPUT -d FL,SFO
*/

func main() {
    cluster := flag.String("cluster", "http://127.0.0.1:9021", "comma separated cluster peers")
    id := flag.Int("id", 1, "node ID")
    port := flag.Int("port", 9121, "dgraph server port")
    join := flag.Bool("join", false, "join an existing cluster")
    flag.Parse()

    proposeC := make(chan string)
    defer close(proposeC)
    confChangeC := make(chan raftpb.ConfChange)
    defer close(confChangeC)

    // raft provides a commit stream for the proposals from the http api
    commitC, errorC := newRaftNode(*id, strings.Split(*cluster, ","), *join, proposeC, confChangeC)

    // the key-value http handler will propose updates to raft
    serveHttpGraphAPI(*port, proposeC, confChangeC, commitC, errorC)
}
