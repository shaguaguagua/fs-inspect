// fs-inspect is a CLI for inspecting and operating multi-instance
// FreeSWITCH clusters.
//
// Commands:
//
//	reg <ext>    find which node a SIP extension is registered on
//	channels     list active channels across every node
//	tail         merged live ESL event stream from every node
//	shell        interactive bubbletea-powered REPL
//	probe        ad-hoc single-node ESL debug shell
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/shaguaguagua/fs-inspect/internal/cluster"
	"github.com/shaguaguagua/fs-inspect/internal/config"
	"github.com/shaguaguagua/fs-inspect/internal/display"
	"github.com/shaguaguagua/fs-inspect/internal/esl"
	"github.com/shaguaguagua/fs-inspect/internal/shell"
	"github.com/shaguaguagua/fs-inspect/internal/tail"
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
	case "tail":
		tailCmd(os.Args[2:])
	case "shell":
		shellCmd(os.Args[2:])
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
  tail         Stream merged ESL events from every node
  shell        Launch the interactive multi-node shell (bubbletea)
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
	fmt.Print(RenderReg(ext, results))

	found := false
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		for _, row := range parseRows(r.Body) {
			if row["reg_user"] == ext {
				found = true
			}
		}
	}
	if !found {
		os.Exit(1)
	}
}

// RenderReg builds the user-facing output for a reg lookup. Extracted so
// the interactive shell can reuse it without reaching into stdout.
func RenderReg(ext string, results []cluster.Result) string {
	var b strings.Builder
	found := 0
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(&b, "%s %-12s %-20s  %s  %v\n",
				display.Red("✗"), display.Cyan(r.Node.Name), r.Node.Addr, display.Red("ERROR"), r.Err)
			continue
		}
		for _, row := range parseRows(r.Body) {
			if row["reg_user"] != ext {
				continue
			}
			fmt.Fprintf(&b, "%s %s %-20s  user=%s  contact=%s  %s\n",
				display.Green("✓"),
				display.Cyan(fmt.Sprintf("%-12s", r.Node.Name)),
				r.Node.Addr,
				display.Bold(row["reg_user"]),
				display.Yellow(row["network_ip"]+":"+row["network_port"]),
				display.Gray("("+r.Elapsed.Round(time.Millisecond).String()+")"),
			)
			found++
		}
	}
	if found == 0 {
		fmt.Fprintf(&b, "extension %s not registered on any known node\n", display.Bold(ext))
	}
	return b.String()
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
	fmt.Print(RenderChannels(cfg, results))
}

// RenderChannels builds the user-facing output for a channels listing.
func RenderChannels(cfg *config.Config, results []cluster.Result) string {
	var b strings.Builder
	header := fmt.Sprintf("%-12s %-10s %-15s  %-15s %-9s %s",
		"NODE", "STATE", "CALLER", "CALLEE", "DUR", "UUID")
	fmt.Fprintln(&b, display.Bold(header))
	fmt.Fprintln(&b, display.Gray(strings.Repeat("─", 78)))

	now := time.Now().Unix()
	total := 0
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(&b, "%s %s  %s  %v\n",
				display.Red("✗"), display.Cyan(fmt.Sprintf("%-12s", r.Node.Name)), display.Red("ERROR"), r.Err)
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
			fmt.Fprintf(&b, "%s %s %s  %s %s %s\n",
				display.Cyan(fmt.Sprintf("%-12s", r.Node.Name)),
				colorizeState(truncate(row["state"], 10)),
				fmt.Sprintf("%-15s", truncate(row["cid_num"], 15)),
				fmt.Sprintf("%-15s", truncate(row["dest"], 15)),
				display.Gray(fmt.Sprintf("%-9s", dur)),
				display.Dim(uuid),
			)
			total++
		}
	}
	fmt.Fprintf(&b, "\n%s active channel(s) across %s node(s)\n",
		display.Bold(strconv.Itoa(total)), display.Bold(strconv.Itoa(len(cfg.Nodes))))
	return b.String()
}

// colorizeState maps common FreeSWITCH channel states to colors so the
// table tells you at a glance what each call is doing.
func colorizeState(state string) string {
	padded := fmt.Sprintf("%-10s", state)
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "CS_EXECUTE", "CS_ROUTING", "ACTIVE":
		return display.Green(padded)
	case "CS_NEW", "CS_INIT", "CS_CONSUME_MEDIA", "EARLY":
		return display.Yellow(padded)
	case "CS_HANGUP", "CS_DESTROY", "CS_REPORTING", "HANGUP":
		return display.Red(padded)
	default:
		return padded
	}
}

func shellCmd(args []string) {
	fs := flag.NewFlagSet("shell", flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath, "path to fs-inspect.yaml")
	_ = fs.Parse(args)

	cfg, err := config.Load(*configPath)
	if err != nil {
		fail(err)
	}

	if err := shell.Run(cfg, shell.Handlers{
		Reg:      RenderReg,
		Channels: RenderChannels,
		Probe:    probeAPI,
	}); err != nil {
		fail(err)
	}
}

func probeCmd(args []string) {
	fs := flag.NewFlagSet("probe", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8021", "FreeSWITCH ESL address host:port")
	password := fs.String("password", "ClueCon", "ESL password")
	cmd := fs.String("cmd", "show channels as json", "FS API command to run")
	_ = fs.Parse(args)

	body, err := probeAPI(*addr, *password, *cmd)
	if err != nil {
		fail(err)
	}
	fmt.Println(display.PrettyJSON(body))
}

// probeAPI is the reusable half of probeCmd: dial, run one API command,
// return the body. The shell uses it for raw-command mode.
func probeAPI(addr, password, cmd string) (string, error) {
	client, err := esl.Dial(addr, password)
	if err != nil {
		return "", err
	}
	defer client.Close()
	return client.API(cmd)
}

// parseRows unwraps the {row_count, rows:[...]} envelope that FreeSWITCH
// returns from `show <thing> as json`. An unparseable body is treated as
// "no rows" — callers already know which node produced it.
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

func tailCmd(args []string) {
	fs := flag.NewFlagSet("tail", flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath, "path to fs-inspect.yaml")
	events := fs.String("events", "CHANNEL_CREATE CHANNEL_ANSWER CHANNEL_HANGUP_COMPLETE",
		"space-separated ESL event names to subscribe to; use 'ALL' for every event")
	_ = fs.Parse(args)

	cfg, err := config.Load(*configPath)
	if err != nil {
		fail(err)
	}

	ch, stop := tail.Subscribe(cfg, *events)
	defer stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	fmt.Printf("%s tailing %s node(s) for events: %s\n",
		display.Bold(display.Cyan("›")), display.Bold(strconv.Itoa(len(cfg.Nodes))), display.Yellow(*events))
	fmt.Printf("%s press Ctrl+C to exit\n\n", display.Gray("›"))

	for {
		select {
		case <-sigCh:
			fmt.Println(display.Gray("\n(stopped)"))
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			fmt.Println(formatTailEvent(ev))
		}
	}
}

// formatTailEvent renders one tail.Event as a single terminal line.
// Shape: HH:MM:SS  node  EVENT_NAME  caller → callee  uuid
func formatTailEvent(ev tail.Event) string {
	if ev.Err != nil {
		return fmt.Sprintf("%s %s %s %s",
			display.Gray(ev.Time.Format("15:04:05")),
			display.Cyan(fmt.Sprintf("%-12s", ev.Node.Name)),
			display.Red("✗"),
			display.Red(ev.Err.Error()),
		)
	}

	uuid := ev.Get("Unique-Id")
	if len(uuid) > 8 {
		uuid = uuid[:8]
	}
	caller := ev.Get("Caller-Caller-Id-Number")
	dest := ev.Get("Caller-Destination-Number")
	legs := caller + " → " + dest
	if caller == "" && dest == "" {
		legs = ""
	}

	return fmt.Sprintf("%s %s %s  %-30s %s",
		display.Gray(ev.Time.Format("15:04:05")),
		display.Cyan(fmt.Sprintf("%-12s", ev.Node.Name)),
		colorizeEventName(ev.Name),
		legs,
		display.Dim(uuid),
	)
}

// colorizeEventName maps common call-lifecycle events to colors so you
// can eyeball call flow at a glance: green for a new channel, yellow
// when it answers, red when it hangs up.
func colorizeEventName(name string) string {
	padded := fmt.Sprintf("%-24s", name)
	switch name {
	case "CHANNEL_CREATE":
		return display.Green(padded)
	case "CHANNEL_ANSWER":
		return display.Yellow(padded)
	case "CHANNEL_HANGUP", "CHANNEL_HANGUP_COMPLETE", "CHANNEL_DESTROY":
		return display.Red(padded)
	default:
		return display.Magenta(padded)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, display.Red("error:"), err)
	os.Exit(1)
}
