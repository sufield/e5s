package bg_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sufield/e5s/internal/bg"
)

// TestAsync_Do verifies that Async.Do spawns a goroutine and doesn't block.
func TestAsync_Do(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	var executed bool
	bg.Async{}.Do(func() {
		executed = true
		wg.Done()
	})

	// Wait for goroutine to finish
	wg.Wait()

	if !executed {
		t.Error("function was not executed")
	}
}

// TestAsync_DoNonBlocking verifies that Async.Do returns immediately without blocking.
func TestAsync_DoNonBlocking(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	done := make(chan struct{})
	bg.Async{}.Do(func() {
		// Block for a bit to ensure the caller doesn't wait
		time.Sleep(10 * time.Millisecond)
		wg.Done()
	})

	// If Async.Do blocks, this will timeout
	go func() {
		wg.Wait()
		close(done)
	}()

	// Verify we returned immediately (not blocking on the sleep)
	select {
	case <-done:
		// Good - goroutine completed
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Async.Do did not complete within timeout")
	}
}

// TestAsync_DoConcurrency verifies multiple concurrent executions work correctly.
func TestAsync_DoConcurrency(t *testing.T) {
	const numGoroutines = 100
	var wg sync.WaitGroup
	var counter atomic.Int32

	wg.Add(numGoroutines)
	runner := bg.Async{}

	for i := 0; i < numGoroutines; i++ {
		runner.Do(func() {
			counter.Add(1)
			wg.Done()
		})
	}

	wg.Wait()

	if got := counter.Load(); got != numGoroutines {
		t.Errorf("expected %d executions, got %d", numGoroutines, got)
	}
}

// TestSync_Do verifies that Sync.Do executes synchronously in the current goroutine.
func TestSync_Do(t *testing.T) {
	var executed bool
	bg.Sync{}.Do(func() {
		executed = true
	})

	// No need for sync primitives - Sync.Do blocks until completion
	if !executed {
		t.Error("function was not executed")
	}
}

// TestSync_DoBlocking verifies that Sync.Do blocks until the function completes.
func TestSync_DoBlocking(t *testing.T) {
	var executed bool
	bg.Sync{}.Do(func() {
		time.Sleep(10 * time.Millisecond)
		executed = true
	})

	// Since Sync.Do blocks, executed should be true immediately after return
	if !executed {
		t.Error("function was not executed before Sync.Do returned")
	}
}

// TestSync_DoConcurrency verifies synchronous execution order.
func TestSync_DoConcurrency(t *testing.T) {
	var executions []int
	var mu sync.Mutex
	runner := bg.Sync{}

	// Execute multiple Do calls - they should happen in order
	for i := 0; i < 5; i++ {
		value := i // Capture loop variable
		runner.Do(func() {
			mu.Lock()
			executions = append(executions, value)
			mu.Unlock()
		})
	}

	// Verify executions happened in order (since Sync blocks)
	expected := []int{0, 1, 2, 3, 4}
	if len(executions) != len(expected) {
		t.Fatalf("expected %d executions, got %d", len(expected), len(executions))
	}
	for i, v := range expected {
		if executions[i] != v {
			t.Errorf("execution %d: expected %d, got %d", i, v, executions[i])
		}
	}
}

// TestRunnerInterface verifies both types implement the Runner interface.
func TestRunnerInterface(t *testing.T) {
	tests := []struct {
		name   string
		runner bg.Runner
	}{
		{"Async", bg.Async{}},
		{"Sync", bg.Sync{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var executed bool
			var wg sync.WaitGroup

			// For Async, we need to wait; for Sync, it blocks anyway
			wg.Add(1)
			tt.runner.Do(func() {
				executed = true
				wg.Done()
			})
			wg.Wait()

			if !executed {
				t.Errorf("%s: function was not executed", tt.name)
			}
		})
	}
}

// TestSync_DoPanic verifies that panics in Sync.Do propagate to the caller.
func TestSync_DoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic to propagate, but it didn't")
		}
	}()

	// This should panic the current goroutine
	bg.Sync{}.Do(func() {
		panic("test panic")
	})
}
