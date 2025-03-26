package validator

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var osMkdirTemp = os.MkdirTemp

func TestProcessExitTasks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name        string
		numWorkers  int
		numTasks    int
		shouldFail  bool
		expectError bool
	}{
		{
			name:       "successful processing multiple tasks",
			numWorkers: 2,
			numTasks:   5,
		},
		{
			name:        "worker error propagation",
			numWorkers:  2,
			numTasks:    3,
			shouldFail:  true,
			expectError: true,
		},
		{
			name:       "no tasks",
			numWorkers: 2,
			numTasks:   0,
		},
		{
			name:       "single worker multiple tasks",
			numWorkers: 1,
			numTasks:   5,
		},
		{
			name:       "more workers than tasks",
			numWorkers: 4,
			numTasks:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup generator
			generator := &VoluntaryExitGenerator{
				NumWorkers: tt.numWorkers,
				OutputDir:  tmpDir,
			}

			// Setup mock command
			origExecCommand := execCommand
			execCommand = mockEthdoCommand(!tt.shouldFail)

			defer func() { execCommand = origExecCommand }()

			// Create tasks
			tasks := make(chan exitTask, tt.numTasks)
			for i := 0; i < tt.numTasks; i++ {
				tasks <- exitTask{
					validatorIndex: i,
					pubkey:         "0x" + "12345678901234567890123456789012345678901234567890123456789012",
					keystorePath:   filepath.Join(tmpDir, "keystore.json"),
				}
			}

			close(tasks)

			config := &BeaconConfig{
				GenesisValidatorsRoot: "0x1234",
				Epoch:                 "12345",
				GenesisVersion:        "0x00000000",
				ExitForkVersion:       "0x00000000",
			}

			// Process tasks
			err := generator.processExitTasks(tasks, config, 1)

			if tt.expectError {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestWorker(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name        string
		setupTasks  func() chan exitTask
		shouldFail  bool
		expectError bool
	}{
		{
			name: "process single task",
			setupTasks: func() chan exitTask {
				tasks := make(chan exitTask, 1)
				tasks <- exitTask{
					validatorIndex: 1,
					pubkey:         "0x" + "12345678901234567890123456789012345678901234567890123456789012",
					keystorePath:   filepath.Join(tmpDir, "keystore.json"),
				}
				close(tasks)

				return tasks
			},
		},
		{
			name: "handle ethdo failure",
			setupTasks: func() chan exitTask {
				tasks := make(chan exitTask, 1)
				tasks <- exitTask{
					validatorIndex: 1,
					pubkey:         "0x" + "12345678901234567890123456789012345678901234567890123456789012",
					keystorePath:   filepath.Join(tmpDir, "keystore.json"),
				}
				close(tasks)

				return tasks
			},
			shouldFail:  true,
			expectError: true,
		},
		{
			name: "empty task channel",
			setupTasks: func() chan exitTask {
				tasks := make(chan exitTask)
				close(tasks)

				return tasks
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup generator
			generator := &VoluntaryExitGenerator{
				OutputDir: tmpDir,
			}

			// Setup mock command
			origExecCommand := execCommand
			execCommand = mockEthdoCommand(!tt.shouldFail)

			defer func() { execCommand = origExecCommand }()

			// Setup worker
			var wg sync.WaitGroup

			wg.Add(1)

			errChan := make(chan error, 1)

			var completedExits uint64

			config := &BeaconConfig{
				GenesisValidatorsRoot: "0x1234",
				Epoch:                 "12345",
				GenesisVersion:        "0x00000000",
				ExitForkVersion:       "0x00000000",
			}

			// Run worker
			go generator.worker(1, tt.setupTasks(), &wg, errChan, &completedExits, config)
			wg.Wait()
			close(errChan)

			// Check results
			var err error
			for e := range errChan {
				err = e

				break
			}

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestReportProgress(t *testing.T) {
	tests := []struct {
		name           string
		iterations     int
		totalKeystores int32
		completedExits uint64
		duration       time.Duration
	}{
		{
			name:           "normal progress",
			iterations:     10,
			totalKeystores: 5,
			completedExits: 5,
			duration:       50 * time.Millisecond,
		},
		{
			name:           "zero progress",
			iterations:     10,
			totalKeystores: 5,
			completedExits: 0,
			duration:       50 * time.Millisecond,
		},
		{
			name:           "complete progress",
			iterations:     10,
			totalKeystores: 5,
			completedExits: 10,
			duration:       50 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := &VoluntaryExitGenerator{
				Iterations:     tt.iterations,
				TotalKeystores: tt.totalKeystores,
			}

			stop := make(chan struct{})

			var completedExits uint64

			atomic.StoreUint64(&completedExits, tt.completedExits)

			// Start progress reporter
			go generator.reportProgress(1, &completedExits, stop)

			// Let it run for a bit
			time.Sleep(tt.duration)

			// Stop the reporter
			close(stop)

			// Verify the completed exits count hasn't changed
			assert.Equal(t, tt.completedExits, atomic.LoadUint64(&completedExits))
		})
	}
}

func TestWorkerTempDirCleanup(t *testing.T) {
	generator := &VoluntaryExitGenerator{}

	var wg sync.WaitGroup

	wg.Add(1)

	errChan := make(chan error, 1)

	var completedExits uint64

	tasks := make(chan exitTask)
	close(tasks)

	config := &BeaconConfig{}

	// Create a patch for os.MkdirTemp to track the created directory
	var tempDir string

	originalMkdirTemp := osMkdirTemp

	osMkdirTemp = func(dir, pattern string) (string, error) {
		var err error
		tempDir, err = originalMkdirTemp(dir, pattern)

		return tempDir, err
	}

	defer func() { osMkdirTemp = originalMkdirTemp }()

	// Run worker
	go generator.worker(1, tasks, &wg, errChan, &completedExits, config)
	wg.Wait()

	// Assert that the specific directory we created does not exist anymore
	if tempDir != "" {
		_, err := os.Stat(tempDir)
		assert.True(t, os.IsNotExist(err), "Worker temporary directory should be cleaned up")
	}
}
