// fs-inspect is a CLI for inspecting and operating multi-instance
// FreeSWITCH clusters.
//
// Commands:
//
//	reg <ext>    find which node a SIP extension is registered on
//	probe        ad-hoc single-node ESL debug shell
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

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
		hits := filterRegistrations(r.Body, ext)
		for _, h := range hits {
			fmt.Printf("✓ %-12s %-20s  user=%s  contact=%s:%s  (%s)\n",
				r.Node.Name, r.Node.Addr, h["reg_user"], h["network_ip"], h["network_port"], r.Elapsed.Round(1e6))
			found++
		}
	}
	if found == 0 {
		fmt.Printf("extension %s not registered on any known node\n", ext)
		os.Exit(1)
	}
}

// filterRegistrations parses the body of "show registrations as json" and
// returns rows whose reg_user equals ext. An unparseable body (empty,
// plain-text error, etc.) is treated as "no hits" rather than a hard error,
// because the caller already knows which node produced it.
func filterRegistrations(body, ext string) []map[string]string {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	var payload struct {
		RowCount int                 `json:"row_count"`
		Rows     []map[string]string `json:"rows"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil
	}
	var out []map[string]string
	for _, row := range payload.Rows {
		if row["reg_user"] == ext {
			out = append(out, row)
		}
	}
	return out
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

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
