// Package bg provides an abstraction for running functions in the background.
//
// This package allows e5s to control its own concurrency behavior, making it possible
// to switch between asynchronous (production) and synchronous (debug) execution modes
// without changing application code.
package bg

// Runner is an interface for executing functions, either synchronously or asynchronously.
//
// This abstraction allows e5s to abstract away the "go func()" decision, making it
// possible to remove all e5s-owned goroutines for debugging purposes while keeping
// the same code paths.
type Runner interface {
	// Do executes the given function.
	// The implementation determines whether this happens synchronously or asynchronously.
	Do(fn func())
}
