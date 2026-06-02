// Package aspect provides AOP (Aspect-Oriented Programming) support via
// AspectChain — an onion-model execution pipeline with Before/After hooks
// for Command, Query, and Event processing.
//
// Register aspects (logging, tracing, metrics, transaction) via RegisterCommandAspect,
// RegisterQueryAspect, and RegisterEventAspect. Built-in aspects are in the builtin subpackage.
package aspect
