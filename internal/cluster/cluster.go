// Package cluster runs a single ESL command across every node in the
// inventory in parallel and returns one result per node.
//
// All fan-out logic for fs-inspect lives here: command-specific parsing
// (registrations, channels, etc.) belongs in the caller, not here.
package cluster

import (
	"sync"
	"time"

	"github.com/shaguaguagua/fs-inspect/internal/config"
	"github.com/shaguaguagua/fs-inspect/internal/esl"
)

// Result is the outcome of running one API command against one node.
type Result struct {
	Node    config.Node
	Body    string
	Err     error
	Elapsed time.Duration
}

// Query fans out apiCmd to every node in cfg in parallel and returns the
// per-node results. A per-node failure does not abort the other nodes;
// inspect r.Err on each result.
func Query(cfg *config.Config, apiCmd string) []Result {
	results := make([]Result, len(cfg.Nodes))
	var wg sync.WaitGroup
	for i, node := range cfg.Nodes {
		wg.Add(1)
		go func(i int, node config.Node) {
			defer wg.Done()
			start := time.Now()
			r := Result{Node: node}
			client, err := esl.Dial(node.Addr, node.Password)
			if err != nil {
				r.Err = err
				r.Elapsed = time.Since(start)
				results[i] = r
				return
			}
			defer client.Close()
			body, err := client.API(apiCmd)
			r.Body = body
			r.Err = err
			r.Elapsed = time.Since(start)
			results[i] = r
		}(i, node)
	}
	wg.Wait()
	return results
}
