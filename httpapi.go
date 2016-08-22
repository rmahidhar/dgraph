package main

import (
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "strconv"
    "strings"
    "github.com/coreos/etcd/raft/raftpb"
)

// Handler for a http based key-value store backed by raft
type httpGraphAPI struct {
    graph       *dgraph
    confChangeC chan<- raftpb.ConfChange
}

func (h *httpGraphAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    oper := r.RequestURI
    switch {
    case r.Method == "PUT":
        v, err := ioutil.ReadAll(r.Body)
        if err != nil {
            log.Printf("Failed to read on PUT (%v)\n", err)
            http.Error(w, "Failed on PUT", http.StatusBadRequest)
            return
        }
        fmt.Printf("PUT  %s : %s\n", oper, string(v))
        s := strings.Split(string(v), ",")
        if oper == "/addvertex" {
            h.graph.ProposeNode(s[0])
        } else if oper == "/addedge" {
            h.graph.ProposeEdge(s[0], s[1])
        } else if oper == "/setedgeproperty" {
            //h.graph.ProposeEdgeProperty(r.Body)
        } else if oper == "/setnodeproperty" {
            //h.graph.ProposeNodeProperty(r.Body)
        } else {

        }

        // Optimistic-- no waiting for ack from raft. Value is not yet
        // committed so a subsequent GET on the key may return old value
        w.WriteHeader(http.StatusNoContent)
    case r.Method == "GET":
        if oper == "getnodes" {
            h.graph.graph.GetNumVertices()
        } else if oper == "getedges" {
            h.graph.graph.GetNumEdges()
        } else {
            http.Error(w, "Failed to GET", http.StatusNotFound)
        }
    case r.Method == "POST":
        url, err := ioutil.ReadAll(r.Body)
        if err != nil {
            log.Printf("Failed to read on POST (%v)\n", err)
            http.Error(w, "Failed on POST", http.StatusBadRequest)
            return
        }

        nodeId, err := strconv.ParseUint(oper[1:], 0, 64)
        if err != nil {
            log.Printf("Failed to convert ID for conf change (%v)\n", err)
            http.Error(w, "Failed on POST", http.StatusBadRequest)
            return
        }

        cc := raftpb.ConfChange{
            Type:    raftpb.ConfChangeAddNode,
            NodeID:  nodeId,
            Context: url,
        }
        h.confChangeC <- cc

        // As above, optimistic that raft will apply the conf change
        w.WriteHeader(http.StatusNoContent)
    case r.Method == "DELETE":
        nodeId, err := strconv.ParseUint(oper[1:], 0, 64)
        if err != nil {
            log.Printf("Failed to convert ID for conf change (%v)\n", err)
            http.Error(w, "Failed on DELETE", http.StatusBadRequest)
            return
        }
        cc := raftpb.ConfChange{
            Type:   raftpb.ConfChangeRemoveNode,
            NodeID: nodeId,
        }
        h.confChangeC <- cc

        // As above, optimistic that raft will apply the conf change
        w.WriteHeader(http.StatusNoContent)
    default:
        w.Header().Set("Allow", "PUT")
        w.Header().Add("Allow", "GET")
        w.Header().Add("Allow", "POST")
        w.Header().Add("Allow", "DELETE")
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

// serveHttpGraphAPI starts a graph with a GET/PUT API and listen
func serveHttpGraphAPI(port int, proposeC chan<- string, confChangeC chan<- raftpb.ConfChange,
                       commitC <-chan *string, errorC <-chan error) {

    // exit when raft goes down
    go func() {
        if err, ok := <-errorC; ok {
            log.Fatal(err)
        }
        os.Exit(0)
    }()

    srv := http.Server{
        Addr: ":" + strconv.Itoa(port),
        Handler: &httpGraphAPI{
            graph:       newDGraph(proposeC, commitC, errorC),
            confChangeC: confChangeC,
        },
    }
    if err := srv.ListenAndServe(); err != nil {
        log.Fatal(err)
    }
}