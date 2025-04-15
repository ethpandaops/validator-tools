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
	tests := []struct {
		name          string
		genesisResp   string
		forkResp      string
		specResp      string
		expectedError bool
	}{
		{
			name: "successful fetch",
			genesisResp: `{
				"data": {
					"genesis_validators_root": "0x1234",
					"genesis_fork_version": "0x5678"
				}
			}`,
			forkResp: `{
				"data": {
					"previous_version": "0x9abc",
					"current_version": "0xdef0"
				}
			}`,
			specResp: `{
				"data": {
					"DOMAIN_BLS_TO_EXECUTION_CHANGE": "0x0abc",
					"DOMAIN_VOLUNTARY_EXIT": "0x0def",
					"MIN_VALIDATOR_WITHDRAWABILITY_DELAY": "1000",
					"CAPELLA_FORK_VERSION": "0x9abc",
					"CAPELLA_FORK_EPOCH": "1000"
				}
			}`,
			expectedError: false,
		},
		{
			name: "empty responses",
			genesisResp: `{
				"data": {
					"genesis_validators_root": "",
					"genesis_fork_version": ""
				}
			}`,
			forkResp: `{
				"data": {
					"previous_version": "",
					"current_version": ""
				}
			}`,
			specResp: `{
				"data": {
					"DOMAIN_BLS_TO_EXECUTION_CHANGE": "",
					"DOMAIN_VOLUNTARY_EXIT": "",
					"MIN_VALIDATOR_WITHDRAWABILITY_DELAY": "0",
					"CAPELLA_FORK_VERSION": "",
					"CAPELLA_FORK_EPOCH": "0"
				}
			}`,
			expectedError: false,
		},
		{
			name: "genesis fetch error",
			genesisResp: `{
				"error": "not found"
			}`,
			forkResp:      `{}`,
			specResp:      `{}`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var resp string

				switch r.URL.Path {
				case "/eth/v1/beacon/genesis":
					if tt.name == "genesis fetch error" {
						w.WriteHeader(http.StatusNotFound)
					}

					resp = tt.genesisResp
				case "/eth/v1/beacon/states/head/fork":
					resp = tt.forkResp
				case "/eth/v1/config/spec":
					resp = tt.specResp
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
			if tt.expectedError {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)

			if tt.name == "successful fetch" {
				assert.Equal(t, "0x1234", config.GenesisValidatorsRoot)
				assert.Equal(t, "0x5678", config.GenesisVersion)
				assert.Equal(t, "1000", config.Epoch)
				assert.Equal(t, "0x9abc", config.ExitForkVersion)
				assert.Equal(t, "0xdef0", config.CurrentForkVersion)
				assert.Equal(t, "0x0abc", config.BlsToExecutionChangeDomain)
				assert.Equal(t, "0x0def", config.VoluntaryExitDomain)
			} else if tt.name == "empty responses" {
				assert.Equal(t, "", config.GenesisValidatorsRoot)
				assert.Equal(t, "", config.GenesisVersion)
				assert.Equal(t, "0", config.Epoch)
				assert.Equal(t, "", config.ExitForkVersion)
				assert.Equal(t, "", config.CurrentForkVersion)
				assert.Equal(t, "", config.BlsToExecutionChangeDomain)
				assert.Equal(t, "", config.VoluntaryExitDomain)
			}
		})
	}
}
