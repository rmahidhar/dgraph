package main

import (
    "fmt"
    "bytes"
    "encoding/gob"
    "log"
    "sync"
    "github.com/rmahidhar/graph"
    "github.com/rmahidhar/graph/adjlist"
    "github.com/rmahidhar/dgraph/graphpb"
    "github.com/golang/protobuf/proto"
)

type dgraph struct {
	proposeC chan<- string // channel for proposing updates
	mutex  sync.RWMutex
	graph  graph.Graph
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
    node := &graphpb.Node{Name: n}
    if bytes, err := proto.Marshal(node); err != nil {
        log.Fatalln("Failed to encode node:", err)
    } else {
        msg := &graphpb.GraphMsg{Oper: graphpb.Operation_ADD_NODE, Msg: bytes}
        if bytes, err = proto.Marshal(msg); err == nil {
            fmt.Printf("ProposeNode Added vertex: %s\n", n)
            g.proposeC <- string(bytes)
        } else {
            log.Fatalln("Failed to encode graph message:", err)
        }
    }
}

func (g *dgraph) ProposeEdge(src string, dst string) {
    edge := &graphpb.Edge{SrcNode: src, DstNode: dst}
    if bytes, err := proto.Marshal(edge); err != nil {
        log.Fatalln("Failed to encode edge:", err)
    } else {
        msg := &graphpb.GraphMsg{Oper: graphpb.Operation_ADD_EDGE, Msg: bytes}
        if bytes, err = proto.Marshal(msg); err == nil {
            fmt.Printf("ProposeEdge Added edge: %s -> %s\n", src, dst)
            g.proposeC <- string(bytes)
        } else {
            log.Fatalln("Failed to encode graph message:", err)
        }
    }
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

        msg := &graphpb.GraphMsg{}

        if err := proto.Unmarshal([]byte(*data), msg); err == nil {
            g.mutex.Lock()
            switch msg.Oper {
            case graphpb.Operation_ADD_NODE:
                n := &graphpb.Node{}
                if err = proto.Unmarshal(msg.Msg, n); err == nil {
                    g.graph.AddVertex(n.Name)
                    fmt.Printf("Added vertex: %s\n", n.Name)
                } else {
                    log.Fatalf("dgraph: could not decode node message (%v)", err)
                }
            case graphpb.Operation_ADD_EDGE:
                e := &graphpb.Edge{}
                if err = proto.Unmarshal(msg.Msg, e); err == nil {
                    g.graph.AddEdge(e.SrcNode, e.DstNode)
                    fmt.Printf("Added edge: %s -> %s\n", e.SrcNode, e.DstNode)
                } else {
                    log.Fatalf("dgraph: could not decode edge message (%v)", err)
                }
            default:
                log.Fatalf("dgraph: invalid message type (%d)", msg.Oper)
            }
            g.mutex.Unlock()
        } else {
            log.Fatalf("dgraph: could not decode graph message (%v)", err)
        }
    }

    if err, ok := <-errorC; ok {
        log.Fatal(err)
    }
}

