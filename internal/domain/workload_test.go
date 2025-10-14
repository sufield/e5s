package domain_test

import (
	"errors"
	"testing"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkload_NewWorkloadValidated tests validated constructor
func TestWorkload_NewWorkloadValidated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pid       int
		uid       int
		gid       int
		path      string
		wantErr   bool
		wantPath  string // expected normalized path
	}{
		{
			name:     "valid workload",
			pid:      1234,
			uid:      1000,
			gid:      1000,
			path:     "/usr/bin/app",
			wantErr:  false,
			wantPath: "/usr/bin/app",
		},
		{
			name:     "path with redundant slashes",
			pid:      1234,
			uid:      1000,
			gid:      1000,
			path:     "/usr//bin///app",
			wantErr:  false,
			wantPath: "/usr/bin/app",
		},
		{
			name:     "path with dot segments",
			pid:      1234,
			uid:      1000,
			gid:      1000,
			path:     "/usr/./bin/../bin/app",
			wantErr:  false,
			wantPath: "/usr/bin/app",
		},
		{
			name:    "negative pid",
			pid:     -1,
			uid:     1000,
			gid:     1000,
			path:    "/usr/bin/app",
			wantErr: true,
		},
		{
			name:    "negative uid",
			pid:     1234,
			uid:     -1,
			gid:     1000,
			path:    "/usr/bin/app",
			wantErr: true,
		},
		{
			name:    "negative gid",
			pid:     1234,
			uid:     1000,
			gid:     -1,
			path:    "/usr/bin/app",
			wantErr: true,
		},
		{
			name:    "empty path",
			pid:     1234,
			uid:     1000,
			gid:     1000,
			path:    "",
			wantErr: true,
		},
		{
			name:     "zero values but valid path",
			pid:      0,
			uid:      0,
			gid:      0,
			path:     "/sbin/init",
			wantErr:  false,
			wantPath: "/sbin/init",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w, err := domain.NewWorkloadValidated(tt.pid, tt.uid, tt.gid, tt.path)

			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, domain.ErrWorkloadInvalid))
				assert.Nil(t, w)
			} else {
				require.NoError(t, err)
				require.NotNil(t, w)
				assert.Equal(t, tt.pid, w.PID())
				assert.Equal(t, tt.uid, w.UID())
				assert.Equal(t, tt.gid, w.GID())
				assert.Equal(t, tt.wantPath, w.Path())
			}
		})
	}
}

// TestWorkload_NewWorkload tests lenient constructor
func TestWorkload_NewWorkload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pid      int
		uid      int
		gid      int
		path     string
		wantPath string
	}{
		{
			name:     "valid workload",
			pid:      1234,
			uid:      1000,
			gid:      1000,
			path:     "/usr/bin/app",
			wantPath: "/usr/bin/app",
		},
		{
			name:     "path normalization",
			pid:      1234,
			uid:      1000,
			gid:      1000,
			path:     "/usr//bin///app",
			wantPath: "/usr/bin/app",
		},
		{
			name:     "empty path preserved",
			pid:      1234,
			uid:      1000,
			gid:      1000,
			path:     "",
			wantPath: "",
		},
		{
			name:     "negative values allowed",
			pid:      -1,
			uid:      -1,
			gid:      -1,
			path:     "/usr/bin/app",
			wantPath: "/usr/bin/app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := domain.NewWorkload(tt.pid, tt.uid, tt.gid, tt.path)

			require.NotNil(t, w)
			assert.Equal(t, tt.pid, w.PID())
			assert.Equal(t, tt.uid, w.UID())
			assert.Equal(t, tt.gid, w.GID())
			assert.Equal(t, tt.wantPath, w.Path())
		})
	}
}

// TestWorkload_MustNewWorkload tests panic behavior
func TestWorkload_MustNewWorkload(t *testing.T) {
	t.Parallel()

	// Valid input should not panic
	assert.NotPanics(t, func() {
		w := domain.MustNewWorkload(1234, 1000, 1000, "/usr/bin/app")
		assert.NotNil(t, w)
	})

	// Invalid input should panic
	assert.Panics(t, func() {
		domain.MustNewWorkload(-1, 1000, 1000, "/usr/bin/app")
	})

	assert.Panics(t, func() {
		domain.MustNewWorkload(1234, 1000, 1000, "")
	})
}

// TestWorkload_Validate tests validation method
func TestWorkload_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		workload *domain.Workload
		wantErr bool
	}{
		{
			name:     "valid workload",
			workload: domain.NewWorkload(1234, 1000, 1000, "/usr/bin/app"),
			wantErr:  false,
		},
		{
			name:     "nil workload",
			workload: nil,
			wantErr:  true,
		},
		{
			name:     "negative pid",
			workload: domain.NewWorkload(-1, 1000, 1000, "/usr/bin/app"),
			wantErr:  true,
		},
		{
			name:     "negative uid",
			workload: domain.NewWorkload(1234, -1, 1000, "/usr/bin/app"),
			wantErr:  true,
		},
		{
			name:     "negative gid",
			workload: domain.NewWorkload(1234, 1000, -1, "/usr/bin/app"),
			wantErr:  true,
		},
		{
			name:     "empty path",
			workload: domain.NewWorkload(1234, 1000, 1000, ""),
			wantErr:  true,
		},
		{
			name:     "zero values with valid path",
			workload: domain.NewWorkload(0, 0, 0, "/sbin/init"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.workload.Validate()

			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, domain.ErrWorkloadInvalid))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestWorkload_IsZero tests zero-value detection
func TestWorkload_IsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		workload *domain.Workload
		wantZero bool
	}{
		{
			name:     "nil workload",
			workload: nil,
			wantZero: true,
		},
		{
			name:     "all zeros",
			workload: domain.NewWorkload(0, 0, 0, ""),
			wantZero: true,
		},
		{
			name:     "valid workload",
			workload: domain.NewWorkload(1234, 1000, 1000, "/usr/bin/app"),
			wantZero: false,
		},
		{
			name:     "zero ids but has path",
			workload: domain.NewWorkload(0, 0, 0, "/sbin/init"),
			wantZero: false,
		},
		{
			name:     "non-zero pid only",
			workload: domain.NewWorkload(1, 0, 0, ""),
			wantZero: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.workload.IsZero()
			assert.Equal(t, tt.wantZero, result)
		})
	}
}

// TestWorkload_String tests string representation
func TestWorkload_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		workload *domain.Workload
		want     string
	}{
		{
			name:     "nil workload",
			workload: nil,
			want:     "workload<nil>",
		},
		{
			name:     "valid workload",
			workload: domain.NewWorkload(1234, 1000, 1000, "/usr/bin/app"),
			want:     `workload{pid=1234,uid=1000,gid=1000,path="/usr/bin/app"}`,
		},
		{
			name:     "zero values",
			workload: domain.NewWorkload(0, 0, 0, ""),
			want:     `workload{pid=0,uid=0,gid=0,path=""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.workload.String()
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestWorkload_Accessors_NilSafe tests nil-safe accessors
func TestWorkload_Accessors_NilSafe(t *testing.T) {
	t.Parallel()

	var nilWorkload *domain.Workload

	// All accessors should be safe on nil
	assert.NotPanics(t, func() {
		assert.Equal(t, 0, nilWorkload.PID())
		assert.Equal(t, 0, nilWorkload.UID())
		assert.Equal(t, 0, nilWorkload.GID())
		assert.Equal(t, "", nilWorkload.Path())
	})
}

// TestWorkload_Immutability tests that workload is immutable
func TestWorkload_Immutability(t *testing.T) {
	t.Parallel()

	w := domain.NewWorkload(1234, 1000, 1000, "/usr/bin/app")

	// Get values
	pid1 := w.PID()
	uid1 := w.UID()
	gid1 := w.GID()
	path1 := w.Path()

	// Call accessors again
	pid2 := w.PID()
	uid2 := w.UID()
	gid2 := w.GID()
	path2 := w.Path()

	// Values should be identical
	assert.Equal(t, pid1, pid2)
	assert.Equal(t, uid1, uid2)
	assert.Equal(t, gid1, gid2)
	assert.Equal(t, path1, path2)

	// No way to mutate the workload (all fields unexported)
	// This is a compile-time guarantee, but we document it here
}

// TestWorkload_PathNormalization tests path cleaning
func TestWorkload_PathNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean path unchanged",
			input:    "/usr/bin/app",
			expected: "/usr/bin/app",
		},
		{
			name:     "redundant slashes",
			input:    "/usr//bin///app",
			expected: "/usr/bin/app",
		},
		{
			name:     "dot segments",
			input:    "/usr/./bin/./app",
			expected: "/usr/bin/app",
		},
		{
			name:     "dot-dot segments",
			input:    "/usr/local/../bin/app",
			expected: "/usr/bin/app",
		},
		{
			name:     "trailing slash removed",
			input:    "/usr/bin/app/",
			expected: "/usr/bin/app",
		},
		{
			name:     "complex normalization",
			input:    "/usr/./local/../bin//./app/",
			expected: "/usr/bin/app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := domain.NewWorkload(1234, 1000, 1000, tt.input)
			assert.Equal(t, tt.expected, w.Path())

			// Also test validated constructor
			w2, err := domain.NewWorkloadValidated(1234, 1000, 1000, tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, w2.Path())
		})
	}
}
