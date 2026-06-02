// Package memory provides in-memory implementations of JobStore and JobManager.
//
// InMemoryJobStore stores jobs in a map with RWMutex protection. JobManager
// executes jobs with timeout, retry, and recovery support. Recovery mode
// automatically resumes jobs left in Running state after a crash.
package memory
