// fs-inspect is a CLI for inspecting and operating multi-instance
// FreeSWITCH clusters.
//
// Commands:
//
//	reg <ext>    find which node a SIP extension is registered on
//	channels     list active channels across every node
//	probe        ad-hoc single-node ESL debug shell
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shaguaguagua/fs-inspect/internal/cluster"
	"github.com/shaguaguagua/fs-inspect/internal/config"
	"github.com/shaguaguagua/fs-inspect/internal/esl"
)

const defaultConfigPath = "fs-inspect.yaml"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "reg":
		regCmd(os.Args[2:])
	case "channels":
		channelsCmd(os.Args[2:])
	case "probe":
		probeCmd(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`fs-inspect — multi-instance FreeSWITCH operations CLI

Usage:
  fs-inspect <command> [flags]

Commands:
  reg <ext>    Find which node a SIP extension is registered on
  channels     List active channels across every node in the cluster
  probe        Ad-hoc single-node ESL debug shell

Run 'fs-inspect <command> -h' for command-specific flags.`)
}

func regCmd(args []string) {
	fs := flag.NewFlagSet("reg", flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath, "path to fs-inspect.yaml")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: fs-inspect reg [-config path] <ext>")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)
	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(2)
	}
	ext := fs.Arg(0)

	cfg, err := config.Load(*configPath)
	if err != nil {
		fail(err)
	}

	results := cluster.Query(cfg, "show registrations as json")

	found := 0
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(os.Stderr, "✗ %-12s %-20s  ERROR  %v\n", r.Node.Name, r.Node.Addr, r.Err)
			continue
		}
		for _, row := range parseRows(r.Body) {
			if row["reg_user"] != ext {
				continue
			}
			fmt.Printf("✓ %-12s %-20s  user=%s  contact=%s:%s  (%s)\n",
				r.Node.Name, r.Node.Addr, row["reg_user"], row["network_ip"], row["network_port"], r.Elapsed.Round(time.Millisecond))
			found++
		}
	}
	if found == 0 {
		fmt.Printf("extension %s not registered on any known node\n", ext)
		os.Exit(1)
	}
}

func channelsCmd(args []string) {
	fs := flag.NewFlagSet("channels", flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath, "path to fs-inspect.yaml")
	_ = fs.Parse(args)

	cfg, err := config.Load(*configPath)
	if err != nil {
		fail(err)
	}

	results := cluster.Query(cfg, "show channels as json")

	fmt.Printf("%-12s %-10s %-15s  %-15s %-9s %s\n", "NODE", "STATE", "CALLER", "CALLEE", "DUR", "UUID")
	fmt.Println(strings.Repeat("─", 78))

	now := time.Now().Unix()
	total := 0
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(os.Stderr, "✗ %-12s ERROR  %v\n", r.Node.Name, r.Err)
			continue
		}
		for _, row := range parseRows(r.Body) {
			created, _ := strconv.ParseInt(row["created_epoch"], 10, 64)
			dur := time.Duration(0)
			if created > 0 {
				dur = time.Duration(now-created) * time.Second
			}
			uuid := row["uuid"]
			if len(uuid) > 8 {
				uuid = uuid[:8]
			}
			fmt.Printf("%-12s %-10s %-15s  %-15s %-9s %s\n",
				r.Node.Name,
				truncate(row["state"], 10),
				truncate(row["cid_num"], 15),
				truncate(row["dest"], 15),
				dur,
				uuid,
			)
			total++
		}
	}
	fmt.Printf("\n%d active channel(s) across %d node(s)\n", total, len(cfg.Nodes))
}

func probeCmd(args []string) {
	fs := flag.NewFlagSet("probe", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8021", "FreeSWITCH ESL address host:port")
	password := fs.String("password", "ClueCon", "ESL password")
	cmd := fs.String("cmd", "show channels as json", "FS API command to run")
	_ = fs.Parse(args)

	client, err := esl.Dial(*addr, *password)
	if err != nil {
		fail(err)
	}
	defer client.Close()

	body, err := client.API(*cmd)
	if err != nil {
		fail(err)
	}
	fmt.Print(body)
}

// parseRows unwraps the {row_count, rows:[...]} envelope that FreeSWITCH
// returns from `show <thing> as json`. An unparseable body is treated as
// "no rows" — callers already know which node produced it and can decide
// whether to surface the parse failure as an error.
func parseRows(body string) []map[string]string {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	var payload struct {
		Rows []map[string]string `json:"rows"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil
	}
	return payload.Rows
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
