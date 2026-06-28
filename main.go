package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/spawn-flowise/spawn-flowise/cmd"
	"github.com/spawn-flowise/spawn-flowise/internal/config"
	"github.com/spawn-flowise/spawn-flowise/internal/container"
)

func usage() {
	fmt.Fprintf(os.Stderr, `spawn-flowise — orchestrate multiple isolated FlowiseAI instances.

Usage:
  %s [flags] <command> [args]

Commands:
  check                 Validate engine reachability and host resources.
  spawn <N>             Start N isolated Flowise instances.
  stop all              Stop all flowise-instance-NN containers.
  stop <N>              Stop flowise-instance-<N-1> only.
  hold                  Stop instances and move data dirs to ~/.bkpflowiseNN.
  unhold                Restore held data dirs from ~/.bkpflowiseNN.
  cleanup               Stop containers, archive data, and remove state.

Flags:
`, os.Args[0])
	flag.PrintDefaults()
}

func main() {
	engineFlag := flag.String("engine", config.DefaultEngine, "Container engine: docker or podman")
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}

	engine, err := container.New(*engineFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	command := args[0]
	switch command {
	case "check":
		err = cmd.RunCheck(engine)
	case "spawn":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: %s spawn <N>\n", os.Args[0])
			os.Exit(1)
		}
		n, err2 := config.ParseInstanceCount(args[1])
		if err2 != nil {
			err = err2
			break
		}
		err = cmd.RunSpawn(engine, n)
	case "stop":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: %s stop all | %s stop <N> (stops flowise-instance-<N-1>)\n", os.Args[0], os.Args[0])
			os.Exit(1)
		}
		err = cmd.RunStop(engine, args[1])
	case "hold":
		err = cmd.RunHold(engine)
	case "unhold":
		err = cmd.RunUnhold(engine)
	case "cleanup":
		err = cmd.RunCleanup(engine)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
