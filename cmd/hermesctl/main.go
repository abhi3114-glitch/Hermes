package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

var (
	version   = "1.0.0"
	adminAddr = "http://localhost:8081"
)

func main() {
	// Global flags
	flag.StringVar(&adminAddr, "admin", adminAddr, "Admin API address")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	command := args[0]

	switch command {
	case "status":
		doStatus()
	case "backends":
		doBackends()
	case "stats":
		doStats()
	case "circuits":
		doCircuits()
	case "version":
		fmt.Printf("hermesctl v%s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`hermesctl - Hermes Admin CLI

Usage:
  hermesctl [flags] <command>

Commands:
  status    Show proxy health status
  backends  List all backends and their status
  stats     Show request statistics
  circuits  Show circuit breaker states
  version   Show version

Flags:
  -admin string   Admin API address (default "http://localhost:8081")`)
}

func doStatus() {
	resp, err := http.Get(adminAddr + "/health")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	status := result["status"].(string)
	healthy := int(result["healthy_backends"].(float64))
	total := int(result["total_backends"].(float64))

	statusSymbol := "✓"
	if status == "unhealthy" {
		statusSymbol = "✗"
	} else if status == "degraded" {
		statusSymbol = "!"
	}

	fmt.Printf("%s Hermes Status: %s\n", statusSymbol, status)
	fmt.Printf("  Healthy backends: %d/%d\n", healthy, total)
}

func doBackends() {
	resp, err := http.Get(adminAddr + "/backends")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var backends []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&backends)

	fmt.Println("BACKEND              HEALTH    CONNECTIONS  WEIGHT")
	fmt.Println("---------------------------------------------------")
	for _, b := range backends {
		health := "healthy"
		if !b["healthy"].(bool) {
			health = "unhealthy"
		}
		fmt.Printf("%-20s %-9s %-12.0f %v\n",
			b["address"],
			health,
			b["connections"],
			b["weight"],
		)
	}
}

func doStats() {
	resp, err := http.Get(adminAddr + "/stats")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var stats map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&stats)

	fmt.Println("Request Statistics")
	fmt.Println("------------------")
	fmt.Printf("Total Requests:  %.0f\n", stats["total_requests"])
	fmt.Printf("Active Requests: %.0f\n", stats["active_requests"])
	fmt.Printf("Failed Requests: %.0f\n", stats["failed_requests"])
}

func doCircuits() {
	resp, err := http.Get(adminAddr + "/circuits")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var circuits map[string]string
	json.Unmarshal(body, &circuits)

	if len(circuits) == 0 {
		fmt.Println("No circuit breakers initialized yet")
		return
	}

	fmt.Println("BACKEND              CIRCUIT STATE")
	fmt.Println("-----------------------------------")
	for addr, state := range circuits {
		fmt.Printf("%-20s %s\n", addr, state)
	}
}
