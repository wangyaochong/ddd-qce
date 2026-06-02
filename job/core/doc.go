// Package job/core defines the async job system interfaces: Job, JobStore,
// JobManager, TypeRegistry, and StateMachine.
//
// Jobs represent long-running operations that need timeout, retry, and
// cancellation support. Submit a job via JobManager.Submit(), track status
// with GetStatus(), and wait for completion with Wait().
package core
