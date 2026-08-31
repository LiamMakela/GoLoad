# GoLoad

GoLoad is a concurrent Layer-7 reverse proxy and load balancer written in Go.

It supports multiple backend services, configurable routing strategies, health checking, retries, rate limiting, metrics, graceful shutdown, WebSocket proxying, and consistent-hash session affinity.

I built GoLoad as a networking and distributed-systems project, then integrated it with a real-time multiplayer Connect Four application running across multiple FastAPI instances with Redis-backed shared state.

## Architecture

```text
                         Browser
                            |
                            v
                    React / nginx
                            |
                            v
                    +---------------+
                    |    GoLoad     |
                    |     :8080     |
                    +-------+-------+
                            |
                 +----------+----------+
                 |                     |
                 v                     v
          +-------------+       +-------------+
          |  FastAPI #1 |       |  FastAPI #2 |
          |    :8000    |       |    :8000    |
          +------+------+       +------+------+
                 |                     |
                 +----------+----------+
                            |
                            v
                      +-----------+
                      |   Redis   |
                      |   :6379   |
                      +-----------+
```

Normal HTTP traffic can be distributed using round-robin or least-connections routing.

Game-scoped requests and WebSocket connections use consistent hashing based on the game ID so that both players in the same game are routed to the same FastAPI process.

Redis provides shared game state between backend instances.

## Features

* HTTP reverse proxying
* Round-robin load balancing
* Least-connections load balancing
* Consistent-hash routing
* WebSocket proxying
* WebSocket session affinity
* Active backend health checks
* Automatic removal of unhealthy backends
* Request retries for safe HTTP methods
* Backend timeouts
* Per-backend connection tracking
* Rate limiting
* Request metrics
* Request IDs
* Forwarded client headers
* Graceful shutdown
* Docker support
* Multi-backend deployment with Docker Compose

## Why Consistent Hashing?

The Connect Four backend maintains active WebSocket connections inside each FastAPI process.

Without session affinity, two players in the same game could connect to different backend instances:

```text
Player 1 -> Backend A

Player 2 -> Backend B
```

Each backend would then only know about one player's WebSocket connection.

GoLoad extracts the game ID from routes such as:

```text
/games/ABC123/join
/games/ABC123/ws/player1
/games/ABC123/ws/player2
```

and hashes:

```text
ABC123
```

to a healthy backend.

As a result, all traffic associated with the same game is routed to the same FastAPI process:

```text
                game_id = ABC123
                       |
                       v
               consistent hash
                       |
                       v
                  Backend #1
                  /        \
                 /          \
            Player 1      Player 2
```

Redis remains the shared source of game state across all backend instances.

## Health Checking

GoLoad periodically sends health-check requests to configured backends.

A backend that stops responding is marked unhealthy and removed from normal load-balancing decisions.

When the backend becomes available again, the health checker automatically returns it to the healthy backend pool.

Example:

```text
backend1 -> healthy
backend2 -> healthy

backend1 stops

backend1 -> unhealthy
backend2 -> healthy

new requests -> backend2
```

## Configuration

GoLoad is configured using:

```text
configs/config.yaml
```

Example:

```yaml
server:
  port: 8080

load_balancer:
  strategy: least_connections

health:
  interval: 3s
  timeout: 2s

proxy:
  timeout: 5s
  retries: 2

backends:
  - url: "http://backend1:8000"
  - url: "http://backend2:8000"

rate_limit:
  enabled: false
  requests_per_second: 1000
  burst: 2000
```

Supported load-balancing strategies:

```text
round_robin
least_connections
```

## Running GoLoad Locally

Start the backend services first, then run:

```bash
go run ./cmd/goload
```

GoLoad will listen on the configured port, typically:

```text
http://localhost:8080
```

## Running the Distributed Demo

The demo architecture uses:

* GoLoad
* two FastAPI backend instances
* Redis
* React
* nginx
* Docker Compose

From the directory containing the Compose file:

```bash
docker compose up --build
```

Then open:

```text
http://localhost:5173
```

Requests from the frontend are sent through GoLoad before reaching the backend services.

## Example Routing

Creating a game is normal load-balanced traffic because no game ID exists yet:

```text
POST /games
    |
    v
least-connections
    |
    v
backend2
```

Once a game ID exists:

```text
POST /games/ABC123/join
WS   /games/ABC123/ws/player1
WS   /games/ABC123/ws/player2
```

all use:

```text
hash("ABC123")
```

and therefore prefer the same healthy backend.

Different game IDs may map to different backend instances.

## Failure Behavior

If a backend becomes unhealthy, GoLoad removes it from the healthy routing pool.

Shared game state remains available through Redis.

Active WebSocket connections hosted by a failed backend cannot survive the process failure and must reconnect.

A possible future extension would use Redis Pub/Sub or another message broker for cross-node WebSocket fanout and reduced dependence on process-local connection state.

## Testing

Run all Go tests with:

```bash
go test ./...
```

Run static analysis with:

```bash
go vet ./...
```

The test suite covers routing behavior including:

* round-robin selection
* least-connections selection
* unhealthy backend exclusion
* consistent hashing
* Connect Four game-key extraction
* WebSocket-key extraction

## Related Project

GoLoad is integrated with my real-time multiplayer Connect Four project:

```text
https://github.com/LiamMakela/Connect-4
```

The application uses:

* React
* TypeScript
* FastAPI
* WebSockets
* Redis
* Docker

GoLoad sits in front of two FastAPI instances and handles HTTP routing and WebSocket affinity.

## Technologies

* Go
* `net/http`
* `httputil.ReverseProxy`
* goroutines
* atomic counters
* consistent hashing
* Docker
* Docker Compose
* FastAPI
* Redis
* WebSockets
* React
* TypeScript

## Future Improvements

Potential extensions include:

* Redis Pub/Sub for cross-node WebSocket broadcasting
* weighted load balancing
* circuit breakers
* dynamic backend registration
* TLS termination
* structured logging
* Prometheus-compatible metrics
* additional integration and load testing
