package main

import (
    "fmt"
    "bytes"
    "encoding/gob"
    "log"
    "sync"
    "github.com/rmahidhar/graph"
    "github.com/rmahidhar/graph/adjlist"
)

type dgraph struct {
	proposeC chan<- string // channel for proposing updates
	mutex  sync.RWMutex
	graph  graph.Graph
}

type node struct {
    Name string
}

type edge struct {
    Src string
    Dst string
}

type edgeProperty struct {
    Src string
    Dst string
    Property string
    Value interface{}
}

type nodeProperty struct {
    Name string
    Property string
    Value interface{}
}

func newDGraph(proposeC chan<- string, // write channel
               commitC <-chan *string, // read channel
               errorC <-chan  error) *dgraph { // read channel
    g := &dgraph{proposeC: proposeC, graph: adjlist.New()}
    // replay log into dgraph
    g.readCommits(commitC, errorC)
    // read commits from raft into dgraph until error
    go g.readCommits(commitC, errorC)
    return g
}

func (g *dgraph) ProposeNode(n string) {
    var buf bytes.Buffer
    if err := gob.NewEncoder(&buf).Encode(node{n}); err != nil {
        log.Fatal(err)
    }
    fmt.Printf("ProposeNode Added vertex: %s\n", n)
    g.proposeC <- string(buf.Bytes())
}

func (g *dgraph) ProposeEdge(src string, dst string) {
    var buf bytes.Buffer
    if err := gob.NewEncoder(&buf).Encode(edge{src, dst}); err != nil {
        log.Fatal(err)
    }
    fmt.Printf("ProposeEdge Added edge: %s -> %s\n", src, dst)
    g.proposeC <- string(buf.Bytes())
}

func (g *dgraph) ProposeNodeProperty(node string, property string, value interface{}) {
    var buf bytes.Buffer
    if err := gob.NewEncoder(&buf).Encode(nodeProperty{node, property, value.(string)}); err != nil {
        log.Fatal(err)
    }
    g.proposeC <- string(buf.Bytes())
}

func (g *dgraph) ProposeEdgeProperty(src string, dst string, property string, value interface{}) {
    var buf bytes.Buffer
    if err := gob.NewEncoder(&buf).Encode(edgeProperty{src, dst, property, value.(string)}); err != nil {
        log.Fatal(err)
    }
    g.proposeC <- string(buf.Bytes())
}

func (g *dgraph) readCommits(commitC <-chan *string, errorC <-chan error) {
    for data := range commitC {
        if data == nil {
            // done replaying log; new data incoming
            return;
        }

        var n node
        var e edge
        var np nodeProperty
        var ep edgeProperty

        dec := gob.NewDecoder(bytes.NewBufferString(*data))
        if err := dec.Decode(&n); err == nil {
            g.mutex.Lock()
            g.graph.AddVertex(n.Name)
            fmt.Printf("Added vertex: %s\n", n.Name)
            g.mutex.Unlock()
        }
        dec = gob.NewDecoder(bytes.NewBufferString(*data))
        if err := dec.Decode(&e); err == nil {
            g.mutex.Lock()
            g.graph.AddEdge(e.Src, e.Dst)
            fmt.Printf("Added edge: %s -> %s\n", e.Src, e.Dst)
            g.mutex.Unlock()
        }
        dec = gob.NewDecoder(bytes.NewBufferString(*data))
        if err := dec.Decode(&np); err == nil {
            //g.mutex.Lock()
            //g.graph.SetEdgeProperty(ep.src, ep.dst, ep.property, ep.value)
            //g.mutex.Unlock()
        }
        dec = gob.NewDecoder(bytes.NewBufferString(*data))
        if err := dec.Decode(&ep); err == nil {
            g.mutex.Lock()
            g.graph.SetEdgeProperty(ep.Src, ep.Dst, ep.Property, ep.Value)
            g.mutex.Unlock()
        }
//else {
//            log.Fatalf("dgraph: could not decode message (%v)", err)
//        }
    }

    if err, ok := <-errorC; ok {
        log.Fatal(err)
    }
}

