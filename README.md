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

1. Start all services using Docker Compose:
```bash
docker-compose up -d
```

2. Run a test:
```bash
docker-compose run runner ./goload start \
  --target http://your-api.com \
  --protocol http \
  --duration 5m \
  --vusers 100
```

### Test Configuration File

Create a test configuration:
```yaml
# test.yaml
name: API Load Test
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

## Available Commands

### Master Node (Default)
```bash
./goload
```

### Worker Node
```bash
./goload worker --master master:8080 [--id worker-1] [--capacity 100] [--max-cpu 80] [--max-memory 80]
```

### Start Test
```bash
./goload start \
  --target http://example.com \
  --protocol http \
  --duration 5m \
  --vusers 100
```

### Status Check
```bash
./goload status --test-id <test-id>
```

### Stop Test
```bash
./goload stop --test-id <test-id>
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

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details.

## Tags

golang, load-testing, performance-testing, distributed-systems, stress-testing, benchmarking, testing-tools