package main

import (
	"ddb-explorer/aws"
	"ddb-explorer/tui"
	"flag"
	"fmt"
	"os"
)

var profile = flag.String("profile", "dev", "AWS profile to use (dev or prod)")
var showHelp = flag.Bool("help", false, "Show help and usage information")

func printHelp() {
	fmt.Println(`DynamoDB TUI Explorer

USAGE:
    ddb-explorer [--profile PROFILE]

OPTIONS:
    --profile    AWS profile to use (default: dev)
    --help       Show this help message`)
}

func main() {
	flag.Parse()

	if *showHelp {
		printHelp()
		return
	}

	if *profile != "dev" && *profile != "prod" {
		fmt.Printf("invalid profile %q: expected dev or prod\n", *profile)
		os.Exit(1)
	}

	client, err := aws.NewClient(*profile)
	if err != nil {
		fmt.Printf("failed to create AWS client: %v\n", err)
		os.Exit(1)
	}

	model := tui.NewModel(*profile, client)
	program := tui.NewProgram(model)

	if _, err := program.Run(); err != nil {
		fmt.Printf("error running TUI: %v\n", err)
		os.Exit(1)
	}
}
