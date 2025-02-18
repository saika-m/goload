# Goload - Distributed Load Testing Framework

Goload is a modern, distributed load testing framework that makes it easy to simulate thousands of users accessing your system from multiple geographic locations simultaneously. Built in Go for maximum performance and minimal resource usage.

## Features

- 🚀 Distributed testing across multiple worker nodes
- 📊 Real-time metrics and visualization
- 🔄 Support for multiple protocols (HTTP, gRPC, WebSocket)
- 📈 Scalable from local tests to cloud deployment
- 🛠️ Easy to configure and extend
- 📝 Detailed reporting and analysis
- 🎯 Custom assertions and validations
- ⚡ Written in Go for high performance

## Quick Start

### Prerequisites

- Go 1.22 or higher
- Docker and Docker Compose (for distributed testing)
- Make

### Installation

```bash
# Clone the repository
git clone https://github.com/saika-m/goload.git
cd goload

# Install dependencies
make deps

# Build the project
make build
```

### Running a Simple Test

1. Start the master node:
```bash
./bin/master
```

2. Start one or more worker nodes:
```bash
./bin/worker --master localhost:8080
```

3. Create a test configuration:
```yaml
# test.yaml
virtual_users: 100
duration: 5m
target: http://your-api.com
protocol: HTTP
scenarios:
  - name: basic-scenario
    weight: 1
    steps:
      - name: get-homepage
        method: GET
        path: /
        think_time: 1s
```

4. Run the test:
```bash
./bin/goload start --config test.yaml
```

### Using Docker Compose

To run a complete distributed testing environment:

```bash
# Start the environment
docker-compose up -d

# Scale workers as needed
docker-compose up -d --scale worker=5
```

## Configuration

### Master Node Configuration

```yaml
# config/master.yaml
api:
  port: 8080
  host: "0.0.0.0"

metrics:
  port: 9090
  interval: "1s"

storage:
  path: "./data"
  retention: "7d"
```

### Worker Node Configuration

```yaml
# config/worker.yaml
master:
  address: "localhost:8080"
  heartbeat_interval: "5s"

execution:
  capacity: 1000
  max_retry: 3

resources:
  cpu_limit: 80
  memory_limit: 80
```

## Test Scenarios

### HTTP Example

```yaml
scenarios:
  - name: user-flow
    weight: 1
    steps:
      - name: login
        method: POST
        path: /api/login
        headers:
          Content-Type: application/json
        body: |
          {
            "username": "test",
            "password": "test123"
          }
        assertions:
          - type: STATUS_CODE
            expected: 200
          - type: RESPONSE_TIME
            expected: 500ms

      - name: get-profile
        method: GET
        path: /api/profile
        think_time: 2s
```

### gRPC Example

```yaml
scenarios:
  - name: grpc-flow
    weight: 1
    steps:
      - name: create-user
        method: CreateUser
        path: localhost:50051/users.UserService
        body: |
          {
            "name": "John Doe",
            "email": "john@example.com"
          }
```

### WebSocket Example

```yaml
scenarios:
  - name: websocket-flow
    weight: 1
    steps:
      - name: connect
        path: ws://localhost:8080/ws
        think_time: 1s
      - name: send-message
        body: |
          {
            "type": "message",
            "content": "Hello!"
          }
```

## Metrics and Monitoring

Goload provides detailed metrics through Prometheus and visualizations through Grafana:

- Response times (min, max, avg, percentiles)
- Request rates and throughput
- Error rates
- Resource utilization
- Custom metrics

Access the dashboards:
- Grafana: http://localhost:3000 (default credentials: admin/admin)
- Prometheus: http://localhost:9091

## API Reference

### Master Node API

#### Start Test
```http
POST /api/v1/tests
Content-Type: application/json

{
  "virtual_users": 100,
  "duration": "5m",
  "target": "http://example.com",
  "protocol": "HTTP",
  "scenarios": [...]
}
```

#### Stop Test
```http
DELETE /api/v1/tests/{test_id}
```

#### Get Test Status
```http
GET /api/v1/tests/{test_id}/status
```

### Worker Node API

#### Register Worker
```http
POST /api/v1/workers
Content-Type: application/json

{
  "id": "worker-1",
  "capacity": 1000,
  "resources": {
    "cpu_cores": 4,
    "memory_mb": 8192
  }
}
```

## Development

### Running Tests

```bash
# Run all tests
make test

# Run specific package tests
go test ./pkg/worker/...
```

### Code Style

```bash
# Format code
make format

# Run linter
make lint
```

### Building for Different Platforms

```bash
# Build for all platforms
make build-all

# Build for specific platform
make build-linux
make build-darwin
make build-windows
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Tags

golang

load-testing

performance-testing

distributed-systems

stress-testing

benchmarking

testing-tools