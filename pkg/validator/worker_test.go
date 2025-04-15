package validator

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorker(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")
	require.NoError(t, os.Mkdir(outputDir, 0o755))

	tests := []struct {
		name        string
		setup       func() (*VoluntaryExitGenerator, chan exitTask, chan error, *uint64)
		expectError bool
	}{
		{
			name: "successful worker execution",
			setup: func() (*VoluntaryExitGenerator, chan exitTask, chan error, *uint64) {
				tasks := make(chan exitTask, 1)
				errChan := make(chan error, 1)
				var completed uint64

				generator := &VoluntaryExitGenerator{
					OutputDir:             outputDir,
					WithdrawalCredentials: "0x123",
					Passphrase:            "testpass",
				}

				tasks <- exitTask{
					validatorIndex: 1,
					pubkey:         "0xabc",
					keystorePath:   "test/keystore.json",
				}
				close(tasks)

				return generator, tasks, errChan, &completed
			},
			expectError: false,
		},
		{
			name: "worker with invalid temp dir",
			setup: func() (*VoluntaryExitGenerator, chan exitTask, chan error, *uint64) {
				tasks := make(chan exitTask, 1)
				errChan := make(chan error, 1)
				var completed uint64

				generator := &VoluntaryExitGenerator{
					OutputDir:             "/invalid/path",
					WithdrawalCredentials: "0x123",
					Passphrase:            "testpass",
				}

				tasks <- exitTask{
					validatorIndex: 1,
					pubkey:         "0xabc",
					keystorePath:   "test/keystore.json",
				}
				close(tasks)

				return generator, tasks, errChan, &completed
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator, tasks, errChan, completed := tt.setup()

			var wg sync.WaitGroup

			wg.Add(1)

			config := &BeaconConfig{
				GenesisValidatorsRoot:      "0x123",
				Epoch:                      "100",
				GenesisVersion:             "0x1",
				ExitForkVersion:            "0x2",
				CurrentForkVersion:         "0x3",
				BlsToExecutionChangeDomain: "0x4",
				VoluntaryExitDomain:        "0x5",
			}

			// Mock execCommand
			origExecCommand := execCommand
			execCommand = func(name string, args ...string) commander {
				return &mockCmd{
					t:      t,
					output: []byte(`{"test": "success"}`),
				}
			}

			defer func() { execCommand = origExecCommand }()

			go generator.worker(0, tasks, &wg, errChan, completed, config)
			wg.Wait()

			if tt.expectError {
				assert.Error(t, <-errChan)
			} else {
				select {
				case err := <-errChan:
					assert.NoError(t, err)
				default:
				}
				assert.Equal(t, uint64(1), *completed)
			}
		})
	}
}

func TestReportProgress(t *testing.T) {
	generator := &VoluntaryExitGenerator{
		TotalKeystores: 2,
		Iterations:     10,
	}

	var completed uint64

	stop := make(chan struct{})

	origLog := log
	log = logrus.New()

	defer func() { log = origLog }()

	go generator.reportProgress(1, &completed, stop)

	// Simulate some progress
	atomic.AddUint64(&completed, 5)
	time.Sleep(100 * time.Millisecond) // Give time for progress to be reported
	close(stop)
}
