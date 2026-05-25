// Package queue provides Laravel-style job queues with swappable backends.
//
// Light drivers (memory) auto-register when the queue package is imported.
// Heavy drivers (sqs, kafka, nats) require a blank import:
//
//	_ "github.com/godx-jp/godx-platform-framework/queue/drivers/sqs"
//
// Job lifecycle hooks integrate with the events module:
// job.processing, job.processed, job.failed.
package queue
