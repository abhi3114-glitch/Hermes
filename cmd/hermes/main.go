package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/hermes-proxy/hermes/internal/core"
)

var (
	version = "1.0.0"
)

func main() {
	// Command line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Hermes v%s\n", version)
		os.Exit(0)
	}

	// Setup logging
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetPrefix("")

	// ASCII banner
	banner := `
  _    _                               
 | |  | |                              
 | |__| | ___ _ __ _ __ ___   ___  ___ 
 |  __  |/ _ \ '__| '_ ' _ \ / _ \/ __|
 | |  | |  __/ |  | | | | | |  __/\__ \
 |_|  |_|\___|_|  |_| |_| |_|\___||___/
                                       
 High-Performance HTTP Reverse Proxy
`
	fmt.Print(banner)

	// Load configuration
	config, err := core.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("[HERMES] Failed to load config: %v", err)
	}

	// Create and run the server
	server, err := core.NewServer(config)
	if err != nil {
		log.Fatalf("[HERMES] Failed to create server: %v", err)
	}

	if err := server.Run(); err != nil {
		log.Fatalf("[HERMES] Server error: %v", err)
	}
}
