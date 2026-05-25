# Study Topics & Recommended Books — Message Routing and Real-Time Systems

This file lists focused topics, learning objectives, and recommended books/resources to deepen understanding of message routing, WebSockets, distributed messaging, and Go concurrency. Use these as a short study plan with concrete exercises.

## Core Topics
- WebSocket protocol (RFC 6455) and lifecycle (upgrade, frames, ping/pong)
- Concurrency in Go: goroutines, channels, sync primitives, context cancellation
- Pub/Sub and routing patterns (fan-out, fan-in, topic routing)
- Message delivery semantics: at-most-once, at-least-once, exactly-once (practical tradeoffs)
- Idempotency, deduplication and message ordering
- Persistence strategies: synchronous vs asynchronous writes, worker pools, DLQ
- Backpressure and flow control (buffer sizes, worker pools)
- Connection management: heartbeats, reconnection strategies, resource cleanup
- Observability: metrics, logging, traces for real-time pipelines
- Scaling patterns: single-node hub vs cross-instance pub/sub (Redis/Redis Streams)

## Recommended Books & Resources
- Designing Data-Intensive Applications — Martin Kleppmann (core concepts: replication, consistency, pub/sub patterns)
- Concurrency in Go — Katherine Cox-Buday (deep dive into Go concurrency patterns)
- Network Programming with Go — Jan Newmarch (practical networking, TCP, WebSocket examples)
- Building Microservices — Sam Newman (patterns for message-driven systems and integration)
- The Art of Scalability — Martin L. Abbott & Michael T. Fisher (system design and scaling concerns)
- Gorilla WebSocket docs (https://github.com/gorilla/websocket) — practical WebSocket implementation in Go
- PostgreSQL docs — transactions, indexes, performance tuning
- RFC6455 — WebSocket protocol specification

## Practical Exercises (hands-on)
1. Build a minimal Go WebSocket echo server and a simple JS client. Verify ping/pong and proper close codes.
2. Implement a Hub with register/unregister and broadcast; write unit tests for concurrent register/unregister.
3. Add conversation subscription mapping and simulate 100 concurrent clients subscribing/publishing to random conversations.
4. Implement an async persistence worker pool that writes messages to Postgres; simulate transient DB errors and retries.
5. Implement a basic ACK protocol with message IDs and client ACKs; record delivered timestamps and validate retries.
6. Load test using `wrk` or `go-wrk` and capture latency/throughput; iterate on buffering and worker pool sizes.

## Learning Schedule (suggested 2-week plan)
- Week 1: WebSocket basics, Go concurrency patterns, Hub + client lifecycle (Exercises 1–3)
- Week 2: Persistence + delivery semantics, retries/DLQ, testing and load tests (Exercises 4–6)

## Notes & Tips
- Start with at-most-once delivery for MVP and design interfaces to add ACKs later.
- Keep wire format compact (avoid large nested JSON for high-throughput scenarios).
- Prefer idempotent DB writes or idempotency keys when enabling retries.
- Measure before optimizing: identify real bottlenecks with simple load tests.

Happy studying — let me know if you want a prioritized reading list or a weekly checklist.
