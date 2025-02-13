package deposit_test

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethpandaops/validator-tools/pkg/deposit"
	state_native "github.com/prysmaticlabs/prysm/v5/beacon-chain/state/state-native"
	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/primitives"
	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"github.com/stretchr/testify/require"
)

func setupTestFiles(t *testing.T) (dir string, cleanup func()) {
	t.Helper()
	tmpDir := t.TempDir()

	// Create test files
	pubkeys := []string{
		"83e8519e3c69669c1141ef7a5e66c710c67ab52cc6f57c4ee35200c35154daa3cdc18bc52a47ef6c5900df29aedcf302",
		"a63bffb2b9be4830811150bdaefd904d32aad8a09998aa9bc836cda7cbab97c594293b995199afe659560ee7a930149d",
	}

	exits := []string{
		`{"message":{"epoch":"29696","validator_index":"1912395"},"signature":"0x953a4db199508b3fa070de4111a9f61afcb6987dc82d257c51b564116a87aafd78bbb33ea33df6510203ca27607b689a065a8e830d8cebcb633af01c105a77db0d5ade46395a4aacafc2c79bcd16f38abb576774f611990d2eb70fdb27c7ca4c"}`,
		`{"message":{"epoch":"29696","validator_index":"1912391"},"signature":"0xa1ad506b4cabfd28c681b1c6dac9c89e8f3314404330653e3beec98dab07627210936a96393caa9953bc99491eb1c22f17736ebe02816b6e9b5023b62f6bc5e58b39db36e4c2b61dd053e4b8c1cb98abb873343a88efc2579a170b1d9e21fdd4"}`,
	}

	for i, pubkey := range pubkeys {
		filename := filepath.Join(tmpDir, "exit-"+pubkey+".json")
		err := os.WriteFile(filename, []byte(exits[i]), 0o600)
		require.NoError(t, err)
	}

	return tmpDir, func() {
		os.RemoveAll(tmpDir)
	}
}

func TestNewVoluntaryExits(t *testing.T) {
	tmpDir, cleanup := setupTestFiles(t)
	defer cleanup()

	tests := []struct {
		name            string
		network         string
		withdrawalCreds string
		numExits        int
		expectError     bool
	}{
		{
			name:            "valid holesky exits",
			network:         "holesky",
			withdrawalCreds: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
			numExits:        1,
			expectError:     false,
		},
		{
			name:            "invalid num exits",
			network:         "holesky",
			withdrawalCreds: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
			numExits:        3,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exits, err := deposit.NewVoluntaryExits(tmpDir, tt.network, tt.withdrawalCreds, tt.numExits)
			if tt.expectError {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, exits)

			// Verify withdrawal credentials
			creds, err := hex.DecodeString(tt.withdrawalCreds)
			require.NoError(t, err)
			require.Equal(t, creds, exits.WithdrawalCreds)

			// Verify exits by pubkey
			require.Len(t, exits.ExitsByPubkey, 2)

			for pubkey, validatorExits := range exits.ExitsByPubkey {
				require.Len(t, validatorExits.Exits, tt.numExits)
				require.NotNil(t, validatorExits.State)

				// Verify pubkey format
				_, err := hex.DecodeString(pubkey)
				require.NoError(t, err)
			}
		})
	}
}

func TestNewVoluntaryExits_EdgeCases(t *testing.T) {
	tmpDir, cleanup := setupTestFiles(t)
	defer cleanup()

	tests := []struct {
		name            string
		network         string
		withdrawalCreds string
		numExits        int
		expectError     bool
		errorContains   string
	}{
		{
			name:            "empty directory",
			network:         "holesky",
			withdrawalCreds: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
			numExits:        1,
			expectError:     true,
			errorContains:   "no voluntary exits found",
		},
		{
			name:            "invalid network",
			network:         "invalid",
			withdrawalCreds: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
			numExits:        1,
			expectError:     true,
			errorContains:   "unknown network",
		},
		{
			name:            "invalid withdrawal creds",
			network:         "holesky",
			withdrawalCreds: "invalid",
			numExits:        1,
			expectError:     true,
			errorContains:   "encoding/hex",
		},
		{
			name:            "zero exits",
			network:         "holesky",
			withdrawalCreds: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
			numExits:        0,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := tmpDir
			if tt.name == "empty directory" {
				testDir = t.TempDir() // Use empty directory
			}

			exits, err := deposit.NewVoluntaryExits(testDir, tt.network, tt.withdrawalCreds, tt.numExits)
			if tt.expectError {
				require.Error(t, err)

				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, exits)
		})
	}
}

func TestNewVoluntaryExits_FileContents(t *testing.T) {
	tmpDir := t.TempDir()
	pubkey := "83e8519e3c69669c1141ef7a5e66c710c67ab52cc6f57c4ee35200c35154daa3cdc18bc52a47ef6c5900df29aedcf302"

	// Create a valid exit file first
	validExit := `{"message":{"epoch":"29696","validator_index":"1912391"},"signature":"0xa1ad506b4cabfd28c681b1c6dac9c89e8f3314404330653e3beec98dab07627210936a96393caa9953bc99491eb1c22f17736ebe02816b6e9b5023b62f6bc5e58b39db36e4c2b61dd053e4b8c1cb98abb873343a88efc2579a170b1d9e21fdd4"}`
	validFilename := filepath.Join(tmpDir, "exit-"+pubkey+".json")

	tests := []struct {
		name          string
		fileContents  string
		setup         func(t *testing.T)
		expectError   bool
		errorContains string
	}{
		{
			name:         "valid file",
			fileContents: validExit,
			setup:        nil,
			expectError:  false,
		},
		{
			name:         "invalid json",
			fileContents: "{invalid json",
			setup: func(t *testing.T) {
				t.Helper()

				err := os.Remove(validFilename)
				require.NoError(t, err)
			},
			expectError:   true,
			errorContains: "no voluntary exits found",
		},
		{
			name:         "missing epoch",
			fileContents: `{"message":{"validator_index":"1912395"},"signature":"0x953a"}`,
			setup: func(t *testing.T) {
				t.Helper()

				err := os.Remove(validFilename)
				require.NoError(t, err)
			},
			expectError:   true,
			errorContains: "no voluntary exits found",
		},
		{
			name:         "missing validator index",
			fileContents: `{"message":{"epoch":"29696"},"signature":"0x953a"}`,
			setup: func(t *testing.T) {
				t.Helper()

				err := os.Remove(validFilename)
				require.NoError(t, err)
			},
			expectError:   true,
			errorContains: "no voluntary exits found",
		},
		{
			name:         "invalid signature",
			fileContents: `{"message":{"epoch":"29696","validator_index":"1912395"},"signature":"invalid"}`,
			setup: func(t *testing.T) {
				t.Helper()

				err := os.Remove(validFilename)
				require.NoError(t, err)
			},
			expectError:   true,
			errorContains: "no voluntary exits found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write valid file first
			err := os.WriteFile(validFilename, []byte(validExit), 0o600)

			require.NoError(t, err)

			if tt.setup != nil {
				tt.setup(t)
			}

			// Write test file
			testFilename := filepath.Join(tmpDir, "exit-test-"+pubkey+".json")

			err = os.WriteFile(testFilename, []byte(tt.fileContents), 0o600)

			require.NoError(t, err)

			_, err = deposit.NewVoluntaryExits(tmpDir, "holesky", "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41", 1)
			if tt.expectError {
				require.Error(t, err)

				if tt.errorContains != "" {
					t.Logf("Got error: %v", err)
					require.Contains(t, err.Error(), tt.errorContains)
				}

				return
			}

			require.NoError(t, err)

			// Cleanup test file
			err = os.Remove(testFilename)
			require.NoError(t, err)
		})
	}
}

func TestExitsVerify(t *testing.T) {
	tmpDir, cleanup := setupTestFiles(t)
	defer cleanup()

	exits, err := deposit.NewVoluntaryExits(tmpDir, "holesky", "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41", 1)
	require.NoError(t, err)

	err = exits.Verify()
	require.NoError(t, err)
}

func TestExitsVerify_EdgeCases(t *testing.T) {
	tmpDir, cleanup := setupTestFiles(t)
	defer cleanup()

	tests := []struct {
		name          string
		modifyExits   func(*deposit.Exits)
		expectError   bool
		errorContains string
	}{
		{
			name: "invalid state",
			modifyExits: func(e *deposit.Exits) {
				// Create an empty state that will fail verification
				for _, validatorExits := range e.ExitsByPubkey {
					slot := primitives.Slot(uint64(params.BeaconConfig().SlotsPerEpoch) * uint64(validatorExits.Exits[0].PBExit.Exit.Epoch))
					state, err := state_native.InitializeFromProtoDeneb(&ethpb.BeaconStateDeneb{
						Slot:                  slot,
						GenesisValidatorsRoot: params.BeaconConfig().GenesisValidatorsRoot[:],
					})
					if err != nil {
						panic(err)
					}

					validatorExits.State = state
				}
			},
			expectError:   true,
			errorContains: "index out of bounds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exits, err := deposit.NewVoluntaryExits(tmpDir, "holesky", "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41", 1)
			require.NoError(t, err)

			if tt.modifyExits != nil {
				tt.modifyExits(exits)
			}

			err = exits.Verify()
			if tt.expectError {
				require.Error(t, err)

				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}

				return
			}

			require.NoError(t, err)
		})
	}
}
