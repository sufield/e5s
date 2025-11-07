package bg

// Sync is a Runner that executes functions synchronously in the current goroutine.
//
// This is the debug mode: each Do call blocks until the function completes,
// making execution more predictable and easier to debug.
type Sync struct{}

// Do executes the function immediately in the current goroutine.
func (Sync) Do(fn func()) {
	fn()
}
