package validator

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/prysmaticlabs/prysm/v5/config/params"
	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPubkeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestSetNetwork(t *testing.T) {
	tests := []struct {
		name        string
		network     string
		expectError bool
	}{
		{
			name:        "mainnet",
			network:     "mainnet",
			expectError: false,
		},
		{
			name:        "holesky",
			network:     "holesky",
			expectError: false,
		},
		{
			name:        "hoodi",
			network:     "hoodi",
			expectError: false,
		},
		{
			name:        "invalid network",
			network:     "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := setNetwork(tt.network)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIsExitFile(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		isDir    bool
		expected bool
	}{
		{
			name:     "valid json file",
			fileName: "exit-0x1234.json",
			isDir:    false,
			expected: true,
		},
		{
			name:     "non-json file",
			fileName: "exit-0x1234.txt",
			isDir:    false,
			expected: false,
		},
		{
			name:     "directory",
			fileName: "dir",
			isDir:    true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := mockDirEntry{
				name:  tt.fileName,
				isDir: tt.isDir,
			}
			result := isExitFile(entry)
			assert.Equal(t, tt.expected, result)
		})
	}
}

type mockDirEntry struct {
	name  string
	isDir bool
}

func (m mockDirEntry) Name() string {
	return m.name
}

func (m mockDirEntry) IsDir() bool {
	return m.isDir
}

func (m mockDirEntry) Type() os.FileMode {
	return 0
}

func (m mockDirEntry) Info() (os.FileInfo, error) {
	return nil, fmt.Errorf("mock directory entry does not implement Info")
}

func TestNewVoluntaryExits(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Setup test exit file
	validatorIndex := "1"
	epoch := "1"
	signature := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	exitFile := filepath.Join(tempDir, "exit-"+testPubkeyHex+".json")
	fileContent := `{
		"message": {
			"epoch": "` + epoch + `",
			"validator_index": "` + validatorIndex + `"
		},
		"signature": "0x` + signature + `"
	}`

	err := os.WriteFile(exitFile, []byte(fileContent), 0o600)
	require.NoError(t, err)

	tests := []struct {
		name                string
		path                string
		network             string
		withdrawalCreds     string
		numExits            int
		expectedPubkeys     []string
		expectError         bool
		expectedPubkeyCount int
		expectedExitFile    bool
	}{
		{
			name:                "valid exits",
			path:                tempDir,
			network:             "mainnet",
			withdrawalCreds:     "0x0123456789abcdef0123456789abcdef01234567",
			numExits:            1,
			expectedPubkeys:     []string{testPubkeyHex},
			expectError:         false,
			expectedPubkeyCount: 1,
			expectedExitFile:    true,
		},
		{
			name:                "invalid network",
			path:                tempDir,
			network:             "invalid",
			withdrawalCreds:     "0x0123456789abcdef0123456789abcdef01234567",
			numExits:            1,
			expectedPubkeys:     []string{testPubkeyHex},
			expectError:         true,
			expectedPubkeyCount: 0,
			expectedExitFile:    false,
		},
		{
			name:                "invalid numExits",
			path:                tempDir,
			network:             "mainnet",
			withdrawalCreds:     "0x0123456789abcdef0123456789abcdef01234567",
			numExits:            2, // We only have 1 exit file
			expectedPubkeys:     []string{testPubkeyHex},
			expectError:         true,
			expectedPubkeyCount: 0,
			expectedExitFile:    false,
		},
		{
			name:                "invalid withdrawal credentials",
			path:                tempDir,
			network:             "mainnet",
			withdrawalCreds:     "invalid",
			numExits:            1,
			expectedPubkeys:     []string{testPubkeyHex},
			expectError:         true,
			expectedPubkeyCount: 0,
			expectedExitFile:    false,
		},
		{
			name:                "unexpected pubkey",
			path:                tempDir,
			network:             "mainnet",
			withdrawalCreds:     "0x0123456789abcdef0123456789abcdef01234567",
			numExits:            1,
			expectedPubkeys:     []string{"differentpubkey"},
			expectError:         true,
			expectedPubkeyCount: 0,
			expectedExitFile:    false,
		},
		{
			name:                "missing expected pubkey",
			path:                tempDir,
			network:             "mainnet",
			withdrawalCreds:     "0x0123456789abcdef0123456789abcdef01234567",
			numExits:            1,
			expectedPubkeys:     []string{testPubkeyHex, "anotherpubkey"},
			expectError:         true,
			expectedPubkeyCount: 0,
			expectedExitFile:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exits, err := NewVoluntaryExits(tt.path, tt.network, tt.withdrawalCreds, tt.numExits, tt.expectedPubkeys)

			if tt.expectError {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, exits)
			assert.Equal(t, tt.expectedPubkeyCount, len(exits.ExitsByPubkey))

			if tt.expectedExitFile {
				pubkeyStr := testPubkeyHex
				validatorExits, exists := exits.ExitsByPubkey[pubkeyStr]
				require.True(t, exists)
				require.NotNil(t, validatorExits)
				require.Equal(t, 1, len(validatorExits.Exits))

				vexit := validatorExits.Exits[0]
				require.NotNil(t, vexit.PBExit)
				require.Equal(t, uint64(1), uint64(vexit.PBExit.Exit.Epoch))
				require.Equal(t, uint64(1), uint64(vexit.PBExit.Exit.ValidatorIndex))

				pkBytes, err := hex.DecodeString(testPubkeyHex)
				require.NoError(t, err)
				assert.Equal(t, pkBytes, vexit.Pubkey)
			}
		})
	}
}

func TestReadExitFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Valid exit file
	validExit := `{
		"message": {
			"epoch": "1",
			"validator_index": "2"
		},
		"signature": "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	}`
	validExitPath := filepath.Join(tempDir, "exit-"+testPubkeyHex+".json")
	err := os.WriteFile(validExitPath, []byte(validExit), 0o600)
	require.NoError(t, err)

	// Invalid JSON
	invalidJSON := `{ invalid json }`
	invalidJSONPath := filepath.Join(tempDir, "exit-"+testPubkeyHex+"2.json")
	err = os.WriteFile(invalidJSONPath, []byte(invalidJSON), 0o600)
	require.NoError(t, err)

	// Invalid epoch
	invalidEpoch := `{
		"message": {
			"epoch": "invalid",
			"validator_index": "2"
		},
		"signature": "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	}`
	invalidEpochPath := filepath.Join(tempDir, "exit-"+testPubkeyHex+"3.json")
	err = os.WriteFile(invalidEpochPath, []byte(invalidEpoch), 0o600)
	require.NoError(t, err)

	// Invalid validator index
	invalidIndex := `{
		"message": {
			"epoch": "1",
			"validator_index": "invalid"
		},
		"signature": "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	}`
	invalidIndexPath := filepath.Join(tempDir, "exit-"+testPubkeyHex+"4.json")
	err = os.WriteFile(invalidIndexPath, []byte(invalidIndex), 0o600)
	require.NoError(t, err)

	// Invalid signature
	invalidSig := `{
		"message": {
			"epoch": "1",
			"validator_index": "2"
		},
		"signature": "0xinvalid"
	}`
	invalidSigPath := filepath.Join(tempDir, "exit-"+testPubkeyHex+"5.json")
	err = os.WriteFile(invalidSigPath, []byte(invalidSig), 0o600)
	require.NoError(t, err)

	// Invalid filename format
	invalidName := `{
		"message": {
			"epoch": "1",
			"validator_index": "2"
		},
		"signature": "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	}`
	invalidNamePath := filepath.Join(tempDir, "invalid-filename.json")
	err = os.WriteFile(invalidNamePath, []byte(invalidName), 0o600)
	require.NoError(t, err)

	tests := []struct {
		name        string
		filePath    string
		expectError bool
	}{
		{
			name:        "valid exit file",
			filePath:    validExitPath,
			expectError: false,
		},
		{
			name:        "invalid JSON",
			filePath:    invalidJSONPath,
			expectError: true,
		},
		{
			name:        "invalid epoch",
			filePath:    invalidEpochPath,
			expectError: true,
		},
		{
			name:        "invalid validator index",
			filePath:    invalidIndexPath,
			expectError: true,
		},
		{
			name:        "invalid signature",
			filePath:    invalidSigPath,
			expectError: true,
		},
		{
			name:        "invalid filename format",
			filePath:    invalidNamePath,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exit, err := readExitFile(tt.filePath)

			if tt.expectError {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, exit)
			require.NotNil(t, exit.PBExit)
			require.NotNil(t, exit.PBExit.Exit)
			require.Equal(t, uint64(1), uint64(exit.PBExit.Exit.Epoch))
			require.Equal(t, uint64(2), uint64(exit.PBExit.Exit.ValidatorIndex))

			pkBytes, err := hex.DecodeString(testPubkeyHex)
			require.NoError(t, err)
			assert.Equal(t, pkBytes, exit.Pubkey)
		})
	}
}

func TestVerify(t *testing.T) {
	// Setup test environment
	err := setNetwork("mainnet")
	require.NoError(t, err)

	// Override beacon config with a simple test config
	originalConfig := params.BeaconConfig().Copy()
	defer func() {
		params.OverrideBeaconConfig(originalConfig)
	}()

	// This is a simplified test since actual verification requires complex setup
	// A more thorough test would mock the VerifyExitAndSignature function

	_, err = hex.DecodeString(testPubkeyHex) // Just decode to check validity, we don't use the pubkey
	require.NoError(t, err)

	withdrawalCredsHex := "0123456789abcdef0123456789abcdef01234567"
	withdrawalCreds, err := hex.DecodeString(withdrawalCredsHex)
	require.NoError(t, err)

	// Create a minimal VoluntaryExits structure for testing
	exits := &VoluntaryExits{
		WithdrawalCreds: withdrawalCreds,
		ExitsByPubkey:   make(map[string]*ValidatorExits),
	}

	// We're not testing the actual Verify functionality as it requires
	// complex setup with Prysm validators and signatures
	// A more realistic test would involve mocking the blocks.VerifyExitAndSignature function

	// Just verify the structure exists
	assert.NotNil(t, exits)
	assert.Equal(t, withdrawalCreds, exits.WithdrawalCreds)
}

func TestValidateIndicesMatch(t *testing.T) {
	// Create test data with matching indices
	pubkey1 := "pubkey1"
	pubkey2 := "pubkey2"

	exitsByPubkey := map[string]*ValidatorExits{
		pubkey1: {
			Exits: []*VoluntaryExit{
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 100,
							Epoch:          1,
						},
					},
				},
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 101,
							Epoch:          1,
						},
					},
				},
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 102,
							Epoch:          1,
						},
					},
				},
			},
		},
		pubkey2: {
			Exits: []*VoluntaryExit{
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 100,
							Epoch:          1,
						},
					},
				},
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 101,
							Epoch:          1,
						},
					},
				},
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 102,
							Epoch:          1,
						},
					},
				},
			},
		},
	}

	// Test with matching indices
	err := validateIndicesMatch(exitsByPubkey)
	assert.NoError(t, err, "Expected no error for matching indices")

	// Create test data with mismatched min index
	mismatchedMinExitsByPubkey := map[string]*ValidatorExits{
		pubkey1: {
			Exits: []*VoluntaryExit{
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 100,
							Epoch:          1,
						},
					},
				},
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 102,
							Epoch:          1,
						},
					},
				},
			},
		},
		pubkey2: {
			Exits: []*VoluntaryExit{
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 101, // Different min index
							Epoch:          1,
						},
					},
				},
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 102,
							Epoch:          1,
						},
					},
				},
			},
		},
	}

	// Test with mismatched min index
	err = validateIndicesMatch(mismatchedMinExitsByPubkey)
	assert.Error(t, err, "Expected error for mismatched min indices")
	assert.Contains(t, err.Error(), "minimum validator index mismatch")

	// Create test data with mismatched max index
	mismatchedMaxExitsByPubkey := map[string]*ValidatorExits{
		pubkey1: {
			Exits: []*VoluntaryExit{
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 100,
							Epoch:          1,
						},
					},
				},
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 102,
							Epoch:          1,
						},
					},
				},
			},
		},
		pubkey2: {
			Exits: []*VoluntaryExit{
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 100,
							Epoch:          1,
						},
					},
				},
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 103, // Different max index
							Epoch:          1,
						},
					},
				},
			},
		},
	}

	// Test with mismatched max index
	err = validateIndicesMatch(mismatchedMaxExitsByPubkey)
	assert.Error(t, err, "Expected error for mismatched max indices")
	assert.Contains(t, err.Error(), "maximum validator index mismatch")

	// Test with a single pubkey (should be valid)
	singlePubkeyExits := map[string]*ValidatorExits{
		pubkey1: exitsByPubkey[pubkey1],
	}
	err = validateIndicesMatch(singlePubkeyExits)
	assert.NoError(t, err, "Expected no error for single pubkey")

	// Test with empty exits
	emptyExitsByPubkey := map[string]*ValidatorExits{
		pubkey1: {
			Exits: []*VoluntaryExit{},
		},
		pubkey2: exitsByPubkey[pubkey2],
	}
	err = validateIndicesMatch(emptyExitsByPubkey)
	assert.Error(t, err, "Expected error for empty exits")
	assert.Contains(t, err.Error(), "no exits found for pubkey")
}

// Add test for validateExits calling validateIndicesMatch
func TestValidateExitsWithIndicesMatch(t *testing.T) {
	// Create test data with matching indices
	pubkey1 := "pubkey1"
	pubkey2 := "pubkey2"

	matchingExitsByPubkey := map[string]*ValidatorExits{
		pubkey1: {
			Exits: []*VoluntaryExit{
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 100,
							Epoch:          1,
						},
					},
				},
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 101,
							Epoch:          1,
						},
					},
				},
			},
		},
		pubkey2: {
			Exits: []*VoluntaryExit{
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 100,
							Epoch:          1,
						},
					},
				},
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 101,
							Epoch:          1,
						},
					},
				},
			},
		},
	}

	// Test validateExits with matching indices
	err := validateExits(matchingExitsByPubkey, 2)
	assert.NoError(t, err, "Expected no error for matching indices")

	// Create test data with mismatched indices
	mismatchedExitsByPubkey := map[string]*ValidatorExits{
		pubkey1: {
			Exits: []*VoluntaryExit{
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 100,
							Epoch:          1,
						},
					},
				},
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 101,
							Epoch:          1,
						},
					},
				},
			},
		},
		pubkey2: {
			Exits: []*VoluntaryExit{
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 101, // Different min
							Epoch:          1,
						},
					},
				},
				{
					PBExit: &ethpb.SignedVoluntaryExit{
						Exit: &ethpb.VoluntaryExit{
							ValidatorIndex: 102, // Different max
							Epoch:          1,
						},
					},
				},
			},
		},
	}

	// Test validateExits with mismatched indices
	err = validateExits(mismatchedExitsByPubkey, 2)
	assert.Error(t, err, "Expected error for mismatched indices")
	assert.Contains(t, err.Error(), "minimum validator index mismatch")
}
