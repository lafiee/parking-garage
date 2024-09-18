# Design Overview

This workspace is designed to simulate an automated parking garage system using a combination of Go, Python, Redis, RabbitMQ, and Docker. The architecture consists of several services, each with a specific role, and is orchestrated using Docker Compose.

## Components

### RabbitMQ
- **Role**: Message broker for handling vehicle entry and exit events.
- **Configuration**: Defined in `docker-compose.yaml` with management UI exposed on port `15672`.

### Redis
- **Role**: In-memory data store for storing vehicle entry and exit records.
- **Configuration**: Defined in `docker-compose.yaml` and configured via `redis.conf` in `services/redis/`.

### Prometheus
- **Role**: Monitoring and metrics collection.
- **Configuration**: Defined in `docker-compose.yaml` with configuration file `prometheus.yml`.

### Simulator Service
- **Role**: Generates vehicle entry and exit events.
- **Implementation**: Written in Go, located in `services/simulator/`.
- **Configuration**: Uses `config.json` for settings.

### Backend Service
- **Role**: Consumes events from RabbitMQ, maintains vehicle records, and invokes REST API for summary.
- **Implementation**: Written in Go, located in `services/backend/`.
- **Configuration**: Environment variables set in `docker-compose.yaml`.

### Writer Service
- **Role**: Receives REST API calls from the backend and writes vehicle summaries to a local file.
- **Implementation**: Written in Python, located in `services/writer/`.

## Workflow

1. **Event Generation**:
   - The simulator service generates vehicle entry and exit events and publishes them to RabbitMQ queues.

2. **Event Consumption**:
   - The backend service consumes these events, updates Redis with entry and exit times, and calculates the duration of parking.

3. **Summary Writing**:
   - Upon processing an exit event, the backend service calls the writer service's REST API to log the summary of the vehicle's parking duration to `logs/vehicle_summary.log`.

4. **Monitoring**:
   - Prometheus collects metrics from the backend service to monitor event processing latency and other statistics. Graph visualizer exposed on port `9090`.

## Deployment

- **Docker Compose**: All services are defined in `docker-compose.yaml` and can be deployed together using  `docker compose up`.
- **Configuration Management**: Environment variables and configuration files are used to manage service settings.

## Directory Structure

- **Root**:
  - `docker-compose.yaml`: Docker Compose configuration.
  - `prometheus.yml`: Prometheus configuration.
  - `README.md`: Project overview and instructions.

- **Services**:
  - `backend/`: Go-based backend service.
  - `simulator/`: Go-based event generator.
  - `writer/`: Python-based summary writer.
  - `redis/`: Redis configuration.

- **Logs**:
  - `logs/`: Directory for volume sharing logs with containers.

## Key Metrics

- **Prometheus Queries**:
  - Backend post latencies: `post_request_latency_seconds`
  - Writer process latency: `rate(request_latency_seconds_sum[5m]) / rate(request_latency_seconds_count[5m])`
