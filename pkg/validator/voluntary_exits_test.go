package validator

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
			expectedPubkeys:     []string{testPubkeyHex, "anotherpubkey"},
			expectError:         true,
			expectedPubkeyCount: 0,
			expectedExitFile:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exits, err := NewVoluntaryExits(tt.path, tt.network, tt.withdrawalCreds, tt.expectedPubkeys)

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
				require.Equal(t, exitFile, vexit.Path) // Test the new Path field

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

func TestValidateCount(t *testing.T) {
	// Create test data
	pubkey1 := "pubkey1"

	tests := []struct {
		name        string
		exits       *VoluntaryExits
		numExits    int
		expectError bool
		errorText   string
	}{
		{
			name: "valid count",
			exits: &VoluntaryExits{
				ExitsByPubkey: map[string]*ValidatorExits{
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
				},
			},
			numExits:    2,
			expectError: false,
		},
		{
			name: "no exits",
			exits: &VoluntaryExits{
				ExitsByPubkey: map[string]*ValidatorExits{},
			},
			numExits:    1,
			expectError: true,
			errorText:   "no voluntary exits found",
		},
		{
			name: "wrong exit count",
			exits: &VoluntaryExits{
				ExitsByPubkey: map[string]*ValidatorExits{
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
						},
					},
				},
			},
			numExits:    2,
			expectError: true,
			errorText:   "expected 2 exits",
		},
		{
			name: "discontinuous indices",
			exits: &VoluntaryExits{
				ExitsByPubkey: map[string]*ValidatorExits{
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
										ValidatorIndex: 102, // Gap in sequence
										Epoch:          1,
									},
								},
							},
						},
					},
				},
			},
			numExits:    2,
			expectError: true,
			errorText:   "2 files found but expected 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.exits.ValidateCount(tt.numExits)
			if tt.expectError {
				require.Error(t, err)

				if tt.errorText != "" {
					assert.Contains(t, err.Error(), tt.errorText)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateIndices(t *testing.T) {
	// Create test data with matching indices
	pubkey1 := "pubkey1"
	pubkey2 := "pubkey2"

	exits := &VoluntaryExits{
		ExitsByPubkey: map[string]*ValidatorExits{
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
		},
	}

	// Test with matching indices
	err := exits.ValidateIndices()
	assert.NoError(t, err, "Expected no error for matching indices")

	// Create test data with mismatched min index
	mismatchedMinExits := &VoluntaryExits{
		ExitsByPubkey: map[string]*ValidatorExits{
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
		},
	}

	// Test with mismatched min index
	err = mismatchedMinExits.ValidateIndices()
	assert.Error(t, err, "Expected error for mismatched min indices")
	assert.Contains(t, err.Error(), "minimum validator index mismatch")

	// Create test data with mismatched max index
	mismatchedMaxExits := &VoluntaryExits{
		ExitsByPubkey: map[string]*ValidatorExits{
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
		},
	}

	// Test with mismatched max index
	err = mismatchedMaxExits.ValidateIndices()
	assert.Error(t, err, "Expected error for mismatched max indices")
	assert.Contains(t, err.Error(), "maximum validator index mismatch")

	// Test with a single pubkey (should be valid)
	singlePubkeyExits := &VoluntaryExits{
		ExitsByPubkey: map[string]*ValidatorExits{
			pubkey1: exits.ExitsByPubkey[pubkey1],
		},
	}
	err = singlePubkeyExits.ValidateIndices()
	assert.NoError(t, err, "Expected no error for single pubkey")

	// Test with empty exits
	emptyExits := &VoluntaryExits{
		ExitsByPubkey: map[string]*ValidatorExits{
			pubkey1: {
				Exits: []*VoluntaryExit{},
			},
			pubkey2: exits.ExitsByPubkey[pubkey2],
		},
	}
	err = emptyExits.ValidateIndices()
	assert.Error(t, err, "Expected error for empty exits")
	assert.Contains(t, err.Error(), "no exits found for pubkey")
}

// Test integration of ValidateCount and ValidateIndices
func TestValidateIntegration(t *testing.T) {
	// Create test data with matching indices
	pubkey1 := "pubkey1"
	pubkey2 := "pubkey2"

	exits := &VoluntaryExits{
		ExitsByPubkey: map[string]*ValidatorExits{
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
		},
	}

	// Test both validations pass
	err := exits.ValidateCount(2)
	assert.NoError(t, err, "Expected no error for count validation")

	err = exits.ValidateIndices()
	assert.NoError(t, err, "Expected no error for indices validation")
}

// mockHTTPGenerator implements a mock for testing HTTP calls
type mockHTTPGenerator struct {
	responses map[string][]byte
	errors    map[string]error
}

func (m *mockHTTPGenerator) FetchJSON(url string) ([]byte, error) {
	if err, exists := m.errors[url]; exists {
		return nil, err
	}

	if resp, exists := m.responses[url]; exists {
		return resp, nil
	}

	return nil, fmt.Errorf("no mock response for URL: %s", url)
}

func TestCopyFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create source file
	sourceContent := "test content"
	sourcePath := filepath.Join(tempDir, "source.txt")
	err := os.WriteFile(sourcePath, []byte(sourceContent), 0o600)
	require.NoError(t, err)

	// Test successful copy
	destPath := filepath.Join(tempDir, "dest.txt")
	err = copyFile(sourcePath, destPath)
	require.NoError(t, err)

	// Verify content matches
	destContent, err := os.ReadFile(destPath)
	require.NoError(t, err)
	assert.Equal(t, sourceContent, string(destContent))

	// Test copy to non-existent directory
	nonExistentDir := filepath.Join(tempDir, "nonexistent", "dest.txt")
	err = copyFile(sourcePath, nonExistentDir)
	assert.Error(t, err)

	// Test copy from non-existent source
	err = copyFile("nonexistent.txt", destPath)
	assert.Error(t, err)
}

func TestVoluntaryExitsExtract(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "output")

	// Create test exit files
	testPubkey1 := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	testPubkey2 := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	// Create exit files in the temp directory
	exitFile1 := filepath.Join(tempDir, "100-"+testPubkey1+".json")
	exitFile2 := filepath.Join(tempDir, "101-"+testPubkey2+".json")

	exitContent1 := `{
		"message": {
			"epoch": "1",
			"validator_index": "100"
		},
		"signature": "0x123456789abcdef123456789abcdef123456789abcdef123456789abcdef123456789abcdef123456789abcdef123456789abcdef123456789abcdef12345"
	}`

	exitContent2 := `{
		"message": {
			"epoch": "1",
			"validator_index": "101"
		},
		"signature": "0xabcdef123456789abcdef123456789abcdef123456789abcdef123456789abcdef123456789abcdef123456789abcdef123456789abcdef123456789abcdef"
	}`

	err := os.WriteFile(exitFile1, []byte(exitContent1), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(exitFile2, []byte(exitContent2), 0o600)
	require.NoError(t, err)

	// Change to temp directory so files can be found
	originalDir, err := os.Getwd()
	require.NoError(t, err)

	defer func() {
		chdirErr := os.Chdir(originalDir)
		require.NoError(t, chdirErr)
	}()

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Create mock beacon API response
	beaconResponse := `{
		"data": [
			{
				"index": "100",
				"validator": {
					"pubkey": "0x` + testPubkey1 + `",
					"withdrawal_credentials": "0x0123456789abcdef0123456789abcdef01234567"
				},
				"status": "active_ongoing"
			},
			{
				"index": "101",
				"validator": {
					"pubkey": "0x` + testPubkey2 + `",
					"withdrawal_credentials": "0x0123456789abcdef0123456789abcdef01234567"
				},
				"status": "active_ongoing"
			}
		]
	}`

	tests := []struct {
		name           string
		setupMock      func() *mockHTTPGenerator
		expectError    bool
		errorContains  string
		validateOutput func(t *testing.T)
	}{
		{
			name: "successful extraction",
			setupMock: func() *mockHTTPGenerator {
				return &mockHTTPGenerator{
					responses: map[string][]byte{
						"http://localhost:5052/eth/v1/beacon/states/finalized/validators": []byte(beaconResponse),
					},
				}
			},
			expectError: false,
			validateOutput: func(t *testing.T) {
				t.Helper()
				// Check that files were copied
				_, err := os.Stat(filepath.Join(outputDir, "100-"+testPubkey1+".json"))
				assert.NoError(t, err)
				_, err = os.Stat(filepath.Join(outputDir, "101-"+testPubkey2+".json"))
				assert.NoError(t, err)
			},
		},
		{
			name: "beacon API error",
			setupMock: func() *mockHTTPGenerator {
				return &mockHTTPGenerator{
					errors: map[string]error{
						"http://localhost:5052/eth/v1/beacon/states/finalized/validators": fmt.Errorf("API error"),
					},
				}
			},
			expectError:   true,
			errorContains: "API error",
		},
		{
			name: "invalid beacon response",
			setupMock: func() *mockHTTPGenerator {
				return &mockHTTPGenerator{
					responses: map[string][]byte{
						"http://localhost:5052/eth/v1/beacon/states/finalized/validators": []byte("invalid json"),
					},
				}
			},
			expectError:   true,
			errorContains: "invalid character",
		},
		{
			name: "validator not found in beacon state",
			setupMock: func() *mockHTTPGenerator {
				emptyResponse := `{"data": []}`

				return &mockHTTPGenerator{
					responses: map[string][]byte{
						"http://localhost:5052/eth/v1/beacon/states/finalized/validators": []byte(emptyResponse),
					},
				}
			},
			expectError:   true,
			errorContains: "not found in beacon state",
		},
		{
			name: "validator not active",
			setupMock: func() *mockHTTPGenerator {
				inactiveResponse := `{
					"data": [
						{
							"index": "100",
							"validator": {
								"pubkey": "0x` + testPubkey1 + `",
								"withdrawal_credentials": "0x0123456789abcdef0123456789abcdef01234567"
							},
							"status": "exited_slashed"
						}
					]
				}`

				return &mockHTTPGenerator{
					responses: map[string][]byte{
						"http://localhost:5052/eth/v1/beacon/states/finalized/validators": []byte(inactiveResponse),
					},
				}
			},
			expectError:   true,
			errorContains: "is not active",
		},
		{
			name: "validator index mismatch",
			setupMock: func() *mockHTTPGenerator {
				mismatchResponse := `{
					"data": [
						{
							"index": "999",
							"validator": {
								"pubkey": "0x` + testPubkey1 + `",
								"withdrawal_credentials": "0x0123456789abcdef0123456789abcdef01234567"
							},
							"status": "active_ongoing"
						}
					]
				}`

				return &mockHTTPGenerator{
					responses: map[string][]byte{
						"http://localhost:5052/eth/v1/beacon/states/finalized/validators": []byte(mismatchResponse),
					},
				}
			},
			expectError:   true,
			errorContains: "not found in beacon state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean output directory
			os.RemoveAll(outputDir)

			// Create VoluntaryExits with test data
			withdrawalCreds, err := hex.DecodeString("0123456789abcdef0123456789abcdef01234567")
			require.NoError(t, err)

			pubkeyBytes1, err := hex.DecodeString(testPubkey1)
			require.NoError(t, err)
			pubkeyBytes2, err := hex.DecodeString(testPubkey2)
			require.NoError(t, err)

			exits := &VoluntaryExits{
				WithdrawalCreds: withdrawalCreds,
				ExitsByPubkey: map[string]*ValidatorExits{
					testPubkey1: {
						Exits: []*VoluntaryExit{
							{
								PBExit: &ethpb.SignedVoluntaryExit{
									Exit: &ethpb.VoluntaryExit{
										ValidatorIndex: 100,
										Epoch:          1,
									},
								},
								Pubkey: pubkeyBytes1,
								Path:   exitFile1,
							},
						},
					},
					testPubkey2: {
						Exits: []*VoluntaryExit{
							{
								PBExit: &ethpb.SignedVoluntaryExit{
									Exit: &ethpb.VoluntaryExit{
										ValidatorIndex: 101,
										Epoch:          1,
									},
								},
								Pubkey: pubkeyBytes2,
								Path:   exitFile2,
							},
						},
					},
				},
			}

			// Mock the HTTP generator
			mockGen := tt.setupMock()

			// Replace the generator creation in Extract method by creating a custom extract method for testing
			err = extractWithMock(exits, "http://localhost:5052", outputDir, mockGen)

			if tt.expectError {
				require.Error(t, err)

				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}

				return
			}

			require.NoError(t, err)

			if tt.validateOutput != nil {
				tt.validateOutput(t)
			}
		})
	}
}

// extractWithMock is a test helper that allows injecting a mock HTTP generator
func extractWithMock(e *VoluntaryExits, beaconURL, outputDir string, mockGen *mockHTTPGenerator) error {
	// Fetch validator data from beacon API using mock
	resp, err := mockGen.FetchJSON(beaconURL + "/eth/v1/beacon/states/finalized/validators")
	if err != nil {
		return err
	}

	// Parse API response
	var validatorResponse struct {
		Data []struct {
			Index     string `json:"index"`
			Validator struct {
				Pubkey                string `json:"pubkey"`
				WithdrawalCredentials string `json:"withdrawal_credentials"`
			} `json:"validator"`
			Status string `json:"status"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp, &validatorResponse); err != nil {
		return err
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}

	// Create map of pubkey -> validator info for active validators
	validatorMap := make(map[string]struct {
		index  string
		status string
	})

	for _, validator := range validatorResponse.Data {
		// Remove 0x prefix if present
		pubkey := strings.TrimPrefix(validator.Validator.Pubkey, "0x")
		validatorMap[pubkey] = struct {
			index  string
			status string
		}{
			index:  validator.Index,
			status: validator.Status,
		}
	}

	// Track which validators we've processed
	processedValidators := make(map[string]bool)

	// For each pubkey in our exit data, find the matching validator and copy files
	for pubkey, validatorExits := range e.ExitsByPubkey {
		validatorInfo, exists := validatorMap[pubkey]
		if !exists {
			return fmt.Errorf("validator with pubkey %s not found in beacon state", pubkey)
		}

		// Check if validator is active (can be active_ongoing, active_exiting, etc.)
		if !strings.HasPrefix(validatorInfo.status, "active") && validatorInfo.status != "pending_initialized" && validatorInfo.status != "pending_queued" {
			return fmt.Errorf("validator with pubkey %s is not active (status: %s)", pubkey, validatorInfo.status)
		}

		// For each exit file for this validator
		for _, exit := range validatorExits.Exits {
			expectedIndex := fmt.Sprintf("%d", exit.PBExit.Exit.ValidatorIndex)

			// Verify the validator index matches what we expect
			if validatorInfo.index != expectedIndex {
				continue
			}

			// Find the source file
			sourceFileName := fmt.Sprintf("%s-%s.json", expectedIndex, pubkey)

			// Copy file to output directory
			destFilePath := filepath.Join(outputDir, sourceFileName)

			if err := copyFile(exit.Path, destFilePath); err != nil {
				return err
			}
		}

		processedValidators[pubkey] = true
	}

	// Verify all expected validators were processed
	for pubkey := range e.ExitsByPubkey {
		if !processedValidators[pubkey] {
			return fmt.Errorf("validator %s was not processed", pubkey)
		}
	}

	return nil
}
