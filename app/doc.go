// Package app provides the App bootstrap: lifecycle management, automatic
// backend configuration, command/query/event handler registration, and
// graceful shutdown.
//
// Use app.NewApp(opts...) with functional options like WithAutoBackend(),
// WithCommandHandlers(), and WithDefaultAspects() to configure your app.
package app
