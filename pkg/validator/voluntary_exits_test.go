package validator

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreValidateExits(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Test data
	pubkey1 := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	pubkey2 := "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	expectedPubkeys := []string{pubkey1, pubkey2}

	// Create test exit files
	createTestExitFile(t, tempDir, "100", pubkey1)
	createTestExitFile(t, tempDir, "101", pubkey1)
	createTestExitFile(t, tempDir, "100", pubkey2)
	createTestExitFile(t, tempDir, "101", pubkey2)

	// Test pre-validation
	result, err := preValidateExits(tempDir, expectedPubkeys)
	require.NoError(t, err)

	// Verify results
	assert.Equal(t, 2, len(result.FilesByPubkey))
	assert.Equal(t, uint64(100), result.MinIndex)
	assert.Equal(t, uint64(101), result.MaxIndex)

	// Check pubkey1 files
	pubkey1Files := result.FilesByPubkey[pubkey1[2:]] // Remove 0x prefix
	assert.Equal(t, 2, len(pubkey1Files))
	assert.Equal(t, uint64(100), pubkey1Files[0].ValidatorIndex)
	assert.Equal(t, uint64(101), pubkey1Files[1].ValidatorIndex)

	// Check pubkey2 files
	pubkey2Files := result.FilesByPubkey[pubkey2[2:]] // Remove 0x prefix
	assert.Equal(t, 2, len(pubkey2Files))
	assert.Equal(t, uint64(100), pubkey2Files[0].ValidatorIndex)
	assert.Equal(t, uint64(101), pubkey2Files[1].ValidatorIndex)
}

func TestPreValidateExitsUnexpectedPubkey(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Test data
	pubkey1 := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	unexpectedPubkey := "0xffffffff90abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	expectedPubkeys := []string{pubkey1}

	// Create test exit files including an unexpected pubkey
	createTestExitFile(t, tempDir, "100", pubkey1)
	createTestExitFile(t, tempDir, "101", unexpectedPubkey)

	// Test pre-validation
	_, err := preValidateExits(tempDir, expectedPubkeys)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected pubkey found")
}

func TestPreValidateExitsMissingPubkey(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Test data
	pubkey1 := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	pubkey2 := "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	expectedPubkeys := []string{pubkey1, pubkey2}

	// Create test exit file for only one pubkey
	createTestExitFile(t, tempDir, "100", pubkey1)

	// Test pre-validation
	_, err := preValidateExits(tempDir, expectedPubkeys)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected pubkey not found")
}

func TestValidatorIndexMismatch(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Test data
	pubkey := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	// Create a file with mismatched indices
	filename := fmt.Sprintf("%s-%s.json", "100", pubkey)
	exitData := SignedVoluntaryExit{
		Message: struct {
			Epoch          string `json:"epoch"`
			ValidatorIndex string `json:"validator_index"`
		}{
			Epoch:          "0",
			ValidatorIndex: "200", // Different from filename index
		},
		Signature: "0x" + hex.EncodeToString(make([]byte, 96)),
	}

	data, err := json.Marshal(exitData)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, filename), data, 0644)
	require.NoError(t, err)

	// Test reading the file
	_, err = readExitFile(filepath.Join(tempDir, filename))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validator index mismatch")
	assert.Contains(t, err.Error(), "filename has 100 but content has 200")
}

func TestCreateBatches(t *testing.T) {
	// Create voluntary exits with test metadata
	s := &VoluntaryExits{
		BatchSize: 2,
		Metadata: PreValidationResult{
			FilesByPubkey: map[string][]ExitFileInfo{
				"pubkey1": {{Path: "file1", ValidatorIndex: 100}},
				"pubkey2": {{Path: "file2", ValidatorIndex: 101}},
				"pubkey3": {{Path: "file3", ValidatorIndex: 102}},
				"pubkey4": {{Path: "file4", ValidatorIndex: 103}},
				"pubkey5": {{Path: "file5", ValidatorIndex: 104}},
			},
		},
	}

	batches := s.createBatches()

	// Should create 3 batches (2, 2, 1)
	assert.Equal(t, 3, len(batches))

	// First batch should have 2 pubkeys
	assert.Equal(t, 2, len(batches[0].Pubkeys))
	assert.Equal(t, 2, len(batches[0].Files))

	// Second batch should have 2 pubkeys
	assert.Equal(t, 2, len(batches[1].Pubkeys))
	assert.Equal(t, 2, len(batches[1].Files))

	// Third batch should have 1 pubkey
	assert.Equal(t, 1, len(batches[2].Pubkeys))
	assert.Equal(t, 1, len(batches[2].Files))
}

func TestValidateIndicesFromRanges(t *testing.T) {
	s := &VoluntaryExits{}

	tests := []struct {
		name          string
		pubkeyRanges  map[string]IndexRange
		expectedError bool
		errorContains string
	}{
		{
			name: "matching ranges",
			pubkeyRanges: map[string]IndexRange{
				"pubkey1": {Min: 100, Max: 200},
				"pubkey2": {Min: 100, Max: 200},
				"pubkey3": {Min: 100, Max: 200},
			},
			expectedError: false,
		},
		{
			name: "mismatched min index",
			pubkeyRanges: map[string]IndexRange{
				"pubkey1": {Min: 100, Max: 200},
				"pubkey2": {Min: 101, Max: 200},
			},
			expectedError: true,
			errorContains: "minimum validator index mismatch",
		},
		{
			name: "mismatched max index",
			pubkeyRanges: map[string]IndexRange{
				"pubkey1": {Min: 100, Max: 200},
				"pubkey2": {Min: 100, Max: 201},
			},
			expectedError: true,
			errorContains: "maximum validator index mismatch",
		},
		{
			name:          "single pubkey",
			pubkeyRanges:  map[string]IndexRange{"pubkey1": {Min: 100, Max: 200}},
			expectedError: false,
		},
		{
			name:          "empty ranges",
			pubkeyRanges:  map[string]IndexRange{},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.validateIndicesFromRanges(tt.pubkeyRanges)
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetBatchSize(t *testing.T) {
	s := &VoluntaryExits{
		BatchSize: 10,
	}

	// Test setting valid batch size
	s.SetBatchSize(20)
	assert.Equal(t, 20, s.BatchSize)

	// Test setting zero batch size (should not change)
	s.SetBatchSize(0)
	assert.Equal(t, 20, s.BatchSize)

	// Test setting negative batch size (should not change)
	s.SetBatchSize(-5)
	assert.Equal(t, 20, s.BatchSize)
}

func TestVoluntaryExitsVerification(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Setup network config
	params.OverrideBeaconConfig(params.MainnetConfig())

	// Test data
	pubkey1 := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	pubkey2 := "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	expectedPubkeys := []string{pubkey1, pubkey2}

	// Create test exit files
	createTestExitFile(t, tempDir, "100", pubkey1)
	createTestExitFile(t, tempDir, "101", pubkey1)
	createTestExitFile(t, tempDir, "100", pubkey2)
	createTestExitFile(t, tempDir, "101", pubkey2)

	// Create voluntary exits
	exits, err := NewVoluntaryExits(tempDir, "mainnet", "0x0000000000000000000000000000000000000000000000000000000000000000", expectedPubkeys)
	require.NoError(t, err)

	// Set small batch size to test batching
	exits.SetBatchSize(1)

	// Test ValidateCount
	err = exits.ValidateCount(2)
	assert.NoError(t, err)

	// Test ValidateIndices
	err = exits.ValidateIndices()
	assert.NoError(t, err)

	// Verify metadata
	assert.Equal(t, 2, len(exits.Metadata.FilesByPubkey))
	assert.Equal(t, uint64(100), exits.Metadata.MinIndex)
	assert.Equal(t, uint64(101), exits.Metadata.MaxIndex)
}

// Helper function to create test exit files
func createTestExitFile(t *testing.T, dir, validatorIndex, pubkey string) {
	filename := fmt.Sprintf("%s-%s.json", validatorIndex, pubkey)
	exitData := SignedVoluntaryExit{
		Message: struct {
			Epoch          string `json:"epoch"`
			ValidatorIndex string `json:"validator_index"`
		}{
			Epoch:          "0",
			ValidatorIndex: validatorIndex,
		},
		Signature: "0x" + hex.EncodeToString(make([]byte, 96)), // dummy signature
	}

	data, err := json.Marshal(exitData)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, filename), data, 0644)
	require.NoError(t, err)
}
