# RoutePilot Outbox Worker

The outbox worker is responsible for processing events from the outbox table and publishing them to Kafka topics. It implements the outbox pattern to ensure reliable event delivery.

## Features

- **Configurable batch processing**: Process events in configurable batch sizes
- **Retry logic**: Built-in retry mechanism for failed Kafka publishes
- **Transaction management**: Ensures events are only marked as published after successful Kafka delivery
- **Graceful shutdown**: Handles shutdown signals properly
- **Health monitoring**: Provides health status information
- **At-least-once delivery**: Guarantees events are delivered at least once to Kafka

## Configuration

The worker is configured via environment variables:

### Database Configuration
- `DATABASE_URL` (required): PostgreSQL connection string
- `DATABASE_MAX_CONNECTIONS` (default: 10): Maximum database connections
- `DATABASE_MAX_IDLE_TIME` (default: 30m): Maximum idle time for connections
- `DATABASE_MAX_LIFETIME` (default: 1h): Maximum lifetime for connections
- `DATABASE_CONNECT_TIMEOUT` (default: 10s): Connection timeout

### Kafka Configuration
- `KAFKA_BROKERS` (default: localhost:9092): Comma-separated list of Kafka brokers
- `KAFKA_RETRY_MAX` (default: 3): Maximum number of retry attempts
- `KAFKA_RETRY_BACKOFF` (default: 100ms): Backoff duration between retries
- `KAFKA_FLUSH_TIMEOUT` (default: 10s): Timeout for flushing messages

### Worker Configuration
- `OUTBOX_BATCH_SIZE` (default: 100): Number of events to process in each batch
- `OUTBOX_POLL_INTERVAL` (default: 5s): Interval between polling for new events
- `OUTBOX_PROCESS_TIMEOUT` (default: 30s): Timeout for processing each batch
- `OUTBOX_SHUTDOWN_TIMEOUT` (default: 30s): Timeout for graceful shutdown

## Usage

### Running the Worker

```bash
# Set required environment variables
export DATABASE_URL="postgres://user:password@localhost:5432/routepilot"
export KAFKA_BROKERS="localhost:9092"

# Run the worker
./bin/worker
```

### Building the Worker

```bash
go build -o bin/worker ./cmd/worker
```

### Docker Usage

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o worker ./cmd/worker

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/worker .
CMD ["./worker"]
```

## Event Processing

The worker processes events from the `outbox` table in the following order:

1. **Claim Batch**: Claims a batch of unpublished events using `FOR UPDATE SKIP LOCKED`
2. **Publish to Kafka**: Publishes events to their respective Kafka topics
3. **Mark Published**: Updates the `published_at` timestamp in the database
4. **Handle Failures**: Leaves failed events unmarked for retry on next poll

## Event Format

Events are published to Kafka with the following structure:

### Headers
- `event_id`: UUID of the outbox event
- `tenant_id`: UUID of the tenant
- `created_at`: ISO 8601 timestamp when event was created

### Message
- **Topic**: Determined by the `topic` field in the outbox event
- **Key**: Optional partition key from the `key` field
- **Value**: JSON payload from the `payload` field

## Topics

The system publishes to the following Kafka topics:

- `flag.exposures`: Flag evaluation events
- `experiment.conversions`: Experiment conversion events
- `experiment.assignments`: Experiment assignment events

## Monitoring

The worker provides structured JSON logging with the following log levels:

- **INFO**: Normal operation events (startup, shutdown, batch processing)
- **ERROR**: Error conditions (database failures, Kafka publish failures)
- **DEBUG**: Detailed debugging information (individual message publishing)

### Health Check

The worker exposes a health status via the `Health()` method:

```json
{
  "status": "running",
  "batch_size": 100,
  "poll_interval": "5s"
}
```

## Error Handling

### Database Errors
- Connection failures: Worker will attempt to reconnect on next poll
- Query failures: Logged as errors, processing continues

### Kafka Errors
- Publish failures: Events remain unmarked and will be retried
- Connection failures: Built-in retry logic with exponential backoff
- Topic not found: Events will be retried (ensure topics exist)

### Graceful Shutdown
- SIGINT/SIGTERM: Initiates graceful shutdown
- In-flight batches: Allowed to complete before shutdown
- Timeout: Configurable shutdown timeout prevents hanging

## Performance Considerations

### Batch Size
- Larger batches: Better throughput, higher memory usage
- Smaller batches: Lower latency, more database queries

### Poll Interval
- Shorter intervals: Lower latency, higher CPU usage
- Longer intervals: Higher latency, lower resource usage

### Database Connections
- Pool size should accommodate concurrent operations
- Consider read replicas for high-volume scenarios

## Troubleshooting

### Common Issues

1. **Events not being processed**
   - Check database connectivity
   - Verify outbox table has unpublished events
   - Check worker logs for errors

2. **Kafka publish failures**
   - Verify Kafka broker connectivity
   - Ensure topics exist and are accessible
   - Check Kafka broker logs

3. **High memory usage**
   - Reduce batch size
   - Check for memory leaks in Kafka client
   - Monitor garbage collection

4. **Slow processing**
   - Increase batch size
   - Reduce poll interval
   - Optimize database queries
   - Scale horizontally with multiple workers

### Debugging

Enable debug logging by setting log level to DEBUG:

```bash
export LOG_LEVEL=DEBUG
./bin/worker
```

This will show detailed information about each event being processed.
