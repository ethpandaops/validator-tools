package validator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock ethdo command for testing
func mockEthdoCommand(success bool) func(string, ...string) commander {
	return func(name string, args ...string) commander {
		if success {
			return &mockCmd{
				output: []byte(`{"message": "Exit generated successfully"}`),
			}
		}

		return &mockCmd{
			shouldFail: true,
			output:     []byte("mock command failed"),
		}
	}
}

func TestNewVoluntaryExitGenerator(t *testing.T) {
	generator := NewVoluntaryExitGenerator(
		"/tmp/exits",
		"0x123",
		"password",
		"http://localhost:5052",
		100,
		10,
		4,
	)

	assert.Equal(t, "/tmp/exits", generator.OutputDir)
	assert.Equal(t, "0x123", generator.WithdrawalCredentials)
	assert.Equal(t, "password", generator.Passphrase)
	assert.Equal(t, "http://localhost:5052", generator.BeaconURL)
	assert.Equal(t, 100, generator.Iterations)
	assert.Equal(t, 10, generator.StartIndex)
	assert.Equal(t, 4, generator.NumWorkers)
	assert.Equal(t, int32(0), generator.CurrentKeystore)
	assert.Equal(t, int32(0), generator.TotalKeystores)
}

func TestGetValidatorStartIndex(t *testing.T) {
	tests := []struct {
		name           string
		providedIndex  int
		serverResponse string
		expectedIndex  int
		expectError    bool
	}{
		{
			name:          "use provided index",
			providedIndex: 42,
			expectedIndex: 42,
		},
		{
			name:           "fetch from beacon node",
			providedIndex:  -1,
			serverResponse: `{"data":[{"index":"5"},{"index":"10"},{"index":"3"}]}`,
			expectedIndex:  10,
		},
		{
			name:           "handle invalid indices",
			providedIndex:  -1,
			serverResponse: `{"data":[{"index":"invalid"},{"index":"10"},{"index":"3"}]}`,
			expectedIndex:  10,
		},
		{
			name:           "no valid indices",
			providedIndex:  -1,
			serverResponse: `{"data":[{"index":"invalid"}]}`,
			expectError:    true,
		},
		{
			name:           "invalid json response",
			providedIndex:  -1,
			serverResponse: `invalid json`,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/eth/v1/beacon/states/head/validators", r.URL.Path)

				_, err := w.Write([]byte(tt.serverResponse))
				require.NoError(t, err)
			}))
			defer server.Close()

			generator := &VoluntaryExitGenerator{
				BeaconURL:  server.URL,
				StartIndex: tt.providedIndex,
			}

			index, err := generator.GetValidatorStartIndex()
			if tt.expectError {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedIndex, index)
		})
	}
}

func TestGenerateExits(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "generator-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test keystore file with proper length pubkey
	keystoreContent := map[string]interface{}{
		"pubkey": "0x" + "12345678901234567890123456789012345678901234567890123456789012",
	}
	keystoreData, err := json.Marshal(keystoreContent)
	require.NoError(t, err)

	keystorePath := filepath.Join(tmpDir, "keystore.json")
	err = os.WriteFile(keystorePath, keystoreData, 0o600)
	require.NoError(t, err)

	tests := []struct {
		name        string
		iterations  int
		startIndex  int
		expectError bool
		setup       func(*VoluntaryExitGenerator)
	}{
		{
			name:       "successful generation",
			iterations: 3,
			startIndex: 1000,
			setup: func(g *VoluntaryExitGenerator) {
				g.OutputDir = tmpDir
				g.NumWorkers = 2
			},
		},
		{
			name:        "invalid keystore path",
			iterations:  3,
			startIndex:  1000,
			expectError: true,
			setup: func(g *VoluntaryExitGenerator) {
				g.OutputDir = tmpDir
				g.NumWorkers = 2
			},
		},
		{
			name:        "invalid output directory",
			iterations:  3,
			startIndex:  1000,
			expectError: true,
			setup: func(g *VoluntaryExitGenerator) {
				g.OutputDir = "/nonexistent/directory"
				g.NumWorkers = 2
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace exec.Command with our mock
			origExecCommand := execCommand
			execCommand = mockEthdoCommand(!tt.expectError)

			defer func() { execCommand = origExecCommand }()

			generator := &VoluntaryExitGenerator{
				Iterations: tt.iterations,
			}

			if tt.setup != nil {
				tt.setup(generator)
			}

			config := &BeaconConfig{
				GenesisValidatorsRoot: "0x1234",
				Epoch:                 "12345",
				GenesisVersion:        "0x00000000",
				ExitForkVersion:       "0x00000000",
			}

			testPath := keystorePath
			if tt.expectError {
				testPath = "nonexistent/keystore.json"
			}

			err := generator.GenerateExits(testPath, config, tt.startIndex)
			if tt.expectError {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)

			// Verify output files were created
			for i := 1; i <= tt.iterations; i++ {
				expectedFile := filepath.Join(generator.OutputDir,
					fmt.Sprintf("%d-%s.json", tt.startIndex+i, keystoreContent["pubkey"]))
				_, err := os.Stat(expectedFile)
				assert.NoError(t, err, "Expected output file not found: %s", expectedFile)
			}
		})
	}
}

func TestSetTotalKeystores(t *testing.T) {
	generator := &VoluntaryExitGenerator{}
	generator.SetTotalKeystores(42)
	assert.Equal(t, int32(42), generator.TotalKeystores)
}

func TestGenerateExitsAtomicCounter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "atomic-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test keystore with proper length pubkey
	keystoreContent := map[string]interface{}{
		"pubkey": "0x" + "12345678901234567890123456789012345678901234567890123456789012",
	}
	keystoreData, err := json.Marshal(keystoreContent)
	require.NoError(t, err)

	keystorePath := filepath.Join(tmpDir, "keystore.json")
	err = os.WriteFile(keystorePath, keystoreData, 0o600)
	require.NoError(t, err)

	// Replace exec.Command with our mock
	origExecCommand := execCommand
	execCommand = mockEthdoCommand(true)

	defer func() { execCommand = origExecCommand }()

	generator := &VoluntaryExitGenerator{
		OutputDir:      tmpDir,
		NumWorkers:     2,
		Iterations:     1,
		TotalKeystores: 3,
	}

	config := &BeaconConfig{
		GenesisValidatorsRoot: "0x1234",
		Epoch:                 "12345",
		GenesisVersion:        "0x00000000",
		ExitForkVersion:       "0x00000000",
	}

	// Generate exits multiple times and check counter
	for i := int32(0); i < 3; i++ {
		err := generator.GenerateExits(keystorePath, config, 1000)
		require.NoError(t, err)

		assert.Equal(t, i+1, generator.CurrentKeystore)
	}
}

func TestGenerator_FetchJSON(t *testing.T) {
	tests := []struct {
		name           string
		serverStatus   int
		serverResponse string
		expectError    bool
	}{
		{
			name:           "successful fetch",
			serverStatus:   http.StatusOK,
			serverResponse: `{"data": "test"}`,
		},
		{
			name:           "server error",
			serverStatus:   http.StatusInternalServerError,
			serverResponse: "internal error",
			expectError:    true,
		},
		{
			name:         "connection error",
			serverStatus: -1, // Server will be closed
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.serverStatus != -1 {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.serverStatus)

					_, err := w.Write([]byte(tt.serverResponse))
					require.NoError(t, err)
				}))
				defer server.Close()
			} else {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
				server.Close() // Close immediately to simulate connection error
			}

			generator := &VoluntaryExitGenerator{}
			resp, err := generator.FetchJSON(server.URL)

			if tt.expectError {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.serverResponse, string(resp))
		})
	}
}
