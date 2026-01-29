package job

import "context"

// Worker is the common interface for job queue implementations (in-memory / Redis / etc).
//
// It intentionally exposes only the lifecycle and submission API that the rest of the app needs.
// Implementations may run worker pools in-process or delegate to external systems.
type Worker interface {
	Submit(ctx context.Context, job *Job) error
	Start(ctx context.Context) error
	Stop()
}


