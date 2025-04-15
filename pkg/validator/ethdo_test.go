package validator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCmd struct {
	t          *testing.T
	mu         sync.Mutex
	execCalled bool
	shouldFail bool
	output     []byte
}

func (m *mockCmd) CombinedOutput() ([]byte, error) {
	m.mu.Lock()
	m.execCalled = true
	m.mu.Unlock()

	if m.shouldFail {
		return []byte("mock command failed"), &exec.ExitError{ProcessState: new(os.ProcessState)}
	}

	return m.output, nil
}

const testKeystorePath = "test/keystore.json"

func TestRunEthdoCommand(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ethdo-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name        string
		setup       func() (*VoluntaryExitGenerator, string, string, string)
		shouldFail  bool
		mockOutput  []byte
		checkOutput func(*testing.T, string)
	}{
		{
			name: "successful execution",
			setup: func() (*VoluntaryExitGenerator, string, string, string) {
				outFile := filepath.Join(tmpDir, "success.json")

				return &VoluntaryExitGenerator{
					Passphrase: "testpass",
				}, testKeystorePath, outFile, tmpDir
			},
			mockOutput: []byte(`{"test": "success"}`),
			checkOutput: func(t *testing.T, outFile string) {
				t.Helper()
				content, err := os.ReadFile(outFile)
				require.NoError(t, err)
				assert.JSONEq(t, `{"test": "success"}`, string(content))
			},
		},
		{
			name: "command execution failure",
			setup: func() (*VoluntaryExitGenerator, string, string, string) {
				outFile := filepath.Join(tmpDir, "fail.json")

				return &VoluntaryExitGenerator{
					Passphrase: "testpass",
				}, testKeystorePath, outFile, tmpDir
			},
			shouldFail: true,
			checkOutput: func(t *testing.T, outFile string) {
				t.Helper()
				_, err := os.Stat(outFile)
				assert.True(t, os.IsNotExist(err))
			},
		},
		{
			name: "output file write failure",
			setup: func() (*VoluntaryExitGenerator, string, string, string) {
				outFile := filepath.Join(tmpDir, "invalid-dir")
				require.NoError(t, os.Mkdir(outFile, 0o755))

				return &VoluntaryExitGenerator{
					Passphrase: "testpass",
				}, testKeystorePath, outFile, tmpDir
			},
			mockOutput: []byte(`{"test": "fail"}`),
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator, keystorePath, outFile, workDir := tt.setup()

			mock := &mockCmd{
				t:          t,
				shouldFail: tt.shouldFail,
				output:     tt.mockOutput,
			}

			origExecCommand := execCommand
			execCommand = func(name string, args ...string) commander {
				assert.Equal(t, "ethdo", name)
				assert.Contains(t, args, "--validator="+keystorePath)
				assert.Contains(t, args, "--passphrase="+generator.Passphrase)
				assert.Contains(t, args, "--json")
				assert.Contains(t, args, "--offline")

				return mock
			}

			defer func() { execCommand = origExecCommand }()

			err := generator.runEthdoCommand(keystorePath, outFile, workDir, logrus.NewEntry(logrus.New()))

			if tt.shouldFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.True(t, mock.execCalled, "ethdo command was not executed")

			if tt.checkOutput != nil {
				tt.checkOutput(t, outFile)
			}
		})
	}
}

func TestEthdoCommandRedaction(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ethdo-redaction-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	generator := &VoluntaryExitGenerator{
		Passphrase: "super-secret-password",
	}

	var logBuffer strings.Builder

	logger := logrus.New()
	logger.SetOutput(&logBuffer)
	logger.SetLevel(logrus.DebugLevel)

	mock := &mockCmd{
		t:      t,
		output: []byte(`{"test": "redaction"}`),
	}

	origExecCommand := execCommand

	execCommand = func(name string, args ...string) commander {
		return mock
	}

	defer func() { execCommand = origExecCommand }()

	err = generator.runEthdoCommand(
		testKeystorePath,
		filepath.Join(tmpDir, "out.json"),
		tmpDir,
		logrus.NewEntry(logger),
	)
	require.NoError(t, err)

	logOutput := logBuffer.String()
	assert.NotContains(t, logOutput, "super-secret-password")
	assert.Contains(t, logOutput, "--passphrase=********")
}
