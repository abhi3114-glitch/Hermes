# Hermes - High-Performance HTTP Reverse Proxy

Hermes is a lightweight, high-performance HTTP reverse proxy and load balancer written in Go. It allows you to distribute incoming HTTP traffic across multiple backend servers, ensuring high availability and reliability through active health checks and circuit breaking.

## Features

- **Load Balancing**: Supports Round-Robin and Least-Connections algorithms to efficiently distribute traffic.
- **Health Checks**:
  - **Active**: Periodically probes backend servers to monitor their availability.
  - **Passive**: Detects failures during request proxying and automatically takes unhealthy backends out of rotation.
- **Circuit Breaking**: Implements the circuit breaker pattern to prevent cascading failures by isolating faulting backends.
- **Request Buffering**: Buffers request bodies to handle slow clients effectively and retry connection errors.
- **Admin API**: Provides a REST API for real-time monitoring of backend status, circuit breaker states, and traffic statistics.
- **CLI Management**: Includes `hermesctl`, a command-line tool for interacting with the admin API.

## Installation

### Prerequisites

- Go 1.21 or higher

### Building from Source

1. Clone the repository:
   ```bash
   git clone https://github.com/abhi3114-glitch/Hermes.git
   cd Hermes
   ```

2. Build the server and CLI tool:
   ```bash
   go build -o hermes ./cmd/hermes
   go build -o hermesctl ./cmd/hermesctl
   ```

## Usage

### Configuration

Create a `config.yaml` file in the working directory. An example configuration is provided below:

```yaml
server:
  listen: ":8080"
  admin_listen: ":8081"

backends:
  - address: "localhost:9001"
    weight: 1
  - address: "localhost:9002"
    weight: 1

load_balancing:
  algorithm: "round-robin"  # Options: "round-robin", "least-connections"

health_check:
  enabled: true
  interval: 10s
  timeout: 2s
  path: "/health"
  unhealthy_threshold: 3
  healthy_threshold: 2

circuit_breaker:
  enabled: true
  failure_threshold: 5
  success_threshold: 3
  timeout: 30s

buffer:
  max_request_body: 10485760  # 10MB
```

### Running the Server

Start the proxy server with your configuration:

```bash
./hermes -config config.yaml
```

### Using the CLI

Use `hermesctl` to monitor the proxy status:

```bash
# Check overall health status
./hermesctl status

# List all backends and their current state
./hermesctl backends

# View request statistics
./hermesctl stats

# Inspect circuit breaker states
./hermesctl circuits
```

## Architecture

Hermes is composed of several modular components:

- **Core**: Handles configuration loading and server lifecycle management.
- **Proxy**: The main request handler that manages buffering and forwarding requests.
- **Balancer**: Manages the pool of backends and executes the load balancing strategy.
- **Health**: Runs background routines for active health checking and monitors passive signals.
- **Circuit**: Maintains the state of circuit breakers for each backend to manage fault tolerance.

## License

This project is licensed under the MIT License.
