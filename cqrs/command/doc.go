// Package command defines the Command interface, CommandHandler[T,R] interface,
// and CommandBus interface for CQRS write operations.
//
// Use Dispatch[T,R](ctx, bus, cmd) to execute a command through the bus with
// generic type safety. Commands are handled by exactly one handler.
package command
