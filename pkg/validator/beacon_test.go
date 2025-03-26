package validator

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBeaconAPI(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{
			name:     "with trailing slash",
			baseURL:  "http://localhost:5052/",
			expected: "http://localhost:5052",
		},
		{
			name:     "without trailing slash",
			baseURL:  "http://localhost:5052",
			expected: "http://localhost:5052",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewBeaconAPI(tt.baseURL)
			assert.Equal(t, tt.expected, api.baseURL)
		})
	}
}

func TestGetLatestValidatorIndex(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		expectedIndex  int
		expectError    bool
	}{
		{
			name:           "successful response",
			responseStatus: http.StatusOK,
			responseBody:   `{"data":[{"index":"0"},{"index":"1"},{"index":"2"}]}`,
			expectedIndex:  2,
			expectError:    false,
		},
		{
			name:           "empty validators",
			responseStatus: http.StatusOK,
			responseBody:   `{"data":[]}`,
			expectError:    true,
		},
		{
			name:           "invalid index",
			responseStatus: http.StatusOK,
			responseBody:   `{"data":[{"index":"invalid"}]}`,
			expectError:    true,
		},
		{
			name:           "server error",
			responseStatus: http.StatusInternalServerError,
			responseBody:   `{"error":"internal server error"}`,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/eth/v1/beacon/states/head/validators", r.URL.Path)
				w.WriteHeader(tt.responseStatus)
				_, err := w.Write([]byte(tt.responseBody))
				require.NoError(t, err)
			}))
			defer server.Close()

			api := NewBeaconAPI(server.URL)
			index, err := api.GetLatestValidatorIndex()

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedIndex, index)
			}
		})
	}
}

func TestBeaconAPIFetchConfig(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		expectedConfig *BeaconConfig
		expectError    bool
	}{
		{
			name:           "successful response",
			responseStatus: http.StatusOK,
			responseBody: `{
				"data": {
					"GENESIS_VALIDATORS_ROOT": "0x1234",
					"GENESIS_FORK_VERSION": "0x00000000",
					"CURRENT_FORK_VERSION": "0x00000001",
					"BLS_TO_EXECUTION_CHANGE_DOMAIN": "0x0A000000",
					"DOMAIN_VOLUNTARY_EXIT": "0x04000000"
				}
			}`,
			expectedConfig: &BeaconConfig{
				GenesisValidatorsRoot:      "0x1234",
				GenesisVersion:             "0x00000000",
				ExitForkVersion:            "0x00000001",
				CurrentForkVersion:         "0x00000001",
				BlsToExecutionChangeDomain: "0x0A000000",
				VoluntaryExitDomain:        "0x04000000",
			},
			expectError: false,
		},
		{
			name:           "server error",
			responseStatus: http.StatusInternalServerError,
			responseBody:   `{"error":"internal server error"}`,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/eth/v1/config/spec", r.URL.Path)
				w.WriteHeader(tt.responseStatus)
				_, err := w.Write([]byte(tt.responseBody))
				require.NoError(t, err)
			}))
			defer server.Close()

			api := NewBeaconAPI(server.URL)
			config, err := api.FetchBeaconConfig()

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedConfig, config)
			}
		})
	}
}
