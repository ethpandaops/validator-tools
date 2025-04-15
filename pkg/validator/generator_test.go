package validator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewVoluntaryExitGenerator(t *testing.T) {
	tests := []struct {
		name            string
		outputDir       string
		withdrawalCreds string
		passphrase      string
		beaconURL       string
		iterations      int
		indexStart      int
		indexOffset     int
		numWorkers      int
		expected        *VoluntaryExitGenerator
	}{
		{
			name:            "basic generator",
			outputDir:       "/tmp/output",
			withdrawalCreds: "0x123",
			passphrase:      "test",
			beaconURL:       "http://localhost:8080",
			iterations:      10,
			indexStart:      0,
			indexOffset:     0,
			numWorkers:      4,
			expected: &VoluntaryExitGenerator{
				OutputDir:             "/tmp/output",
				WithdrawalCredentials: "0x123",
				Passphrase:            "test",
				BeaconURL:             "http://localhost:8080",
				Iterations:            10,
				IndexStart:            0,
				IndexOffset:           0,
				NumWorkers:            4,
				CurrentKeystore:       0,
				TotalKeystores:        0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewVoluntaryExitGenerator(
				tt.outputDir,
				tt.withdrawalCreds,
				tt.passphrase,
				tt.beaconURL,
				tt.iterations,
				tt.indexStart,
				tt.indexOffset,
				tt.numWorkers,
			)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestSetTotalKeystores(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		expected int32
	}{
		{
			name:     "positive number",
			total:    100,
			expected: 100,
		},
		{
			name:     "negative number",
			total:    -1,
			expected: 0,
		},
		{
			name:     "zero",
			total:    0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &VoluntaryExitGenerator{}
			g.SetTotalKeystores(tt.total)
			assert.Equal(t, tt.expected, g.TotalKeystores)
		})
	}
}

func TestGetValidatorStartIndex(t *testing.T) {
	tests := []struct {
		name        string
		indexStart  int
		indexOffset int
		expected    int
		expectErr   bool
	}{
		{
			name:        "valid index start",
			indexStart:  100,
			indexOffset: 10,
			expected:    110,
			expectErr:   false,
		},
		{
			name:        "negative index start",
			indexStart:  -1,
			indexOffset: 0,
			expected:    0,
			expectErr:   true, // Will fail due to missing beacon URL
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &VoluntaryExitGenerator{
				IndexStart:  tt.indexStart,
				IndexOffset: tt.indexOffset,
			}

			got, err := g.GetValidatorStartIndex()
			if tt.expectErr {
				assert.Error(t, err)

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGenerateExits(t *testing.T) {
	// Create a temporary keystore file for testing
	tempDir := t.TempDir()
	keystorePath := filepath.Join(tempDir, "keystore.json")
	keystoreData := map[string]string{
		"pubkey": "0x1234567890abcdef",
	}

	keystoreJSON, err := json.Marshal(keystoreData)
	require.NoError(t, err)

	err = os.WriteFile(keystorePath, keystoreJSON, 0o600)
	require.NoError(t, err)

	tests := []struct {
		name         string
		keystorePath string
		config       *BeaconConfig
		startIndex   int
		expectErr    bool
	}{
		{
			name:         "valid keystore",
			keystorePath: keystorePath,
			config:       &BeaconConfig{},
			startIndex:   0,
			expectErr:    false,
		},
		{
			name:         "invalid keystore path",
			keystorePath: "/nonexistent/path",
			config:       &BeaconConfig{},
			startIndex:   0,
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &VoluntaryExitGenerator{
				OutputDir:  tempDir,
				Iterations: 1,
			}

			err := g.GenerateExits(tt.keystorePath, tt.config, tt.startIndex)
			if tt.expectErr {
				assert.Error(t, err)

				return
			}

			assert.NoError(t, err)
		})
	}
}
