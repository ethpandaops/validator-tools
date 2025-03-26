package validator

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchJSON(t *testing.T) {
	tests := []struct {
		name           string
		responseCode   int
		responseBody   string
		expectedError  bool
		expectedOutput string
	}{
		{
			name:           "successful fetch",
			responseCode:   http.StatusOK,
			responseBody:   `{"data": "test"}`,
			expectedError:  false,
			expectedOutput: `{"data": "test"}`,
		},
		{
			name:          "non-200 status code",
			responseCode:  http.StatusNotFound,
			responseBody:  "not found",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseCode)

				_, err := w.Write([]byte(tt.responseBody))
				require.NoError(t, err)
			}))
			defer server.Close()

			g := &VoluntaryExitGenerator{}
			result, err := g.FetchJSON(server.URL)

			if tt.expectedError {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedOutput, string(result))
		})
	}
}

func TestFetchBeaconConfig(t *testing.T) {
	mockGenesisResp := `{
		"data": {
			"genesis_validators_root": "0x1234",
			"genesis_fork_version": "0x5678"
		}
	}`
	mockForkResp := `{
		"data": {
			"epoch": "1000",
			"previous_version": "0x9abc",
			"current_version": "0xdef0"
		}
	}`
	mockSpecResp := `{
		"data": {
			"DOMAIN_BLS_TO_EXECUTION_CHANGE": "0x0abc",
			"DOMAIN_VOLUNTARY_EXIT": "0x0def"
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp string

		switch r.URL.Path {
		case "/eth/v1/beacon/genesis":
			resp = mockGenesisResp
		case "/eth/v1/beacon/states/head/fork":
			resp = mockForkResp
		case "/eth/v1/config/spec":
			resp = mockSpecResp
		default:
			w.WriteHeader(http.StatusNotFound)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(resp))
		require.NoError(t, err)
	}))
	defer server.Close()

	g := &VoluntaryExitGenerator{
		BeaconURL: server.URL,
	}

	config, err := g.FetchBeaconConfig()
	require.NoError(t, err)

	assert.Equal(t, "0x1234", config.GenesisValidatorsRoot)
	assert.Equal(t, "0x5678", config.GenesisVersion)
	assert.Equal(t, "1000", config.Epoch)
	assert.Equal(t, "0x9abc", config.ExitForkVersion)
	assert.Equal(t, "0xdef0", config.CurrentForkVersion)
	assert.Equal(t, "0x0abc", config.BlsToExecutionChangeDomain)
	assert.Equal(t, "0x0def", config.VoluntaryExitDomain)
}
