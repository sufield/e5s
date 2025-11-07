package bg

// Async is a Runner that executes functions asynchronously in a new goroutine.
//
// This is the production mode: each Do call spawns a goroutine, allowing
// concurrent execution.
type Async struct{}

// Do executes the function in a new goroutine.
func (Async) Do(fn func()) {
	go fn()
}
