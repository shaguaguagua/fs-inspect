// fs-inspect is a CLI for inspecting multi-instance FreeSWITCH clusters.
//
// This is the bootstrap skeleton: it connects to a single FS instance via
// ESL and prints the result of `show channels as json`. Multi-instance
// aggregation lands in a follow-up commit.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/shaguaguagua/fs-inspect/internal/esl"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8021", "FreeSWITCH ESL address host:port")
	password := flag.String("password", "ClueCon", "ESL password")
	cmd := flag.String("cmd", "show channels as json", "FS API command to run")
	flag.Parse()

	client, err := esl.Dial(*addr, *password)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer client.Close()

	body, err := client.API(*cmd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Print(body)
}
