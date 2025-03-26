package validator

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// BeaconAPI handles interactions with the beacon node
type BeaconAPI struct {
	baseURL string
}

// NewBeaconAPI creates a new BeaconAPI instance
func NewBeaconAPI(baseURL string) *BeaconAPI {
	return &BeaconAPI{
		baseURL: strings.TrimSuffix(baseURL, "/"),
	}
}

// GetLatestValidatorIndex fetches the latest validator index from the beacon node
func (b *BeaconAPI) GetLatestValidatorIndex() (int, error) {
	endpoint := "/eth/v1/beacon/states/head/validators"

	requestURL, err := url.Parse(b.baseURL)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse base URL")
	}

	requestURL.Path += endpoint

	urlStr := requestURL.String()

	log.WithField("url", urlStr).Debug("Fetching latest validator index")

	client := &http.Client{}

	req, err := http.NewRequest("GET", urlStr, http.NoBody)
	if err != nil {
		return 0, errors.Wrap(err, "failed to create request")
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, errors.Wrap(err, "failed to fetch validator data")
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return 0, errors.Errorf("failed to fetch validator data: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Data []struct {
			Index string `json:"index"`
		} `json:"data"`
	}

	if errD := json.NewDecoder(resp.Body).Decode(&result); errD != nil {
		return 0, errors.Wrap(errD, "failed to decode validator data")
	}

	if len(result.Data) == 0 {
		return 0, errors.New("no validators found")
	}

	lastIndex := result.Data[len(result.Data)-1].Index

	index, err := strconv.Atoi(lastIndex)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse validator index")
	}

	log.WithField("index", index).Debug("Latest validator index fetched")

	return index, nil
}

// FetchBeaconConfig fetches the beacon chain configuration
func (b *BeaconAPI) FetchBeaconConfig() (*BeaconConfig, error) {
	endpoint := "/eth/v1/config/spec"

	requestURL, err := url.Parse(b.baseURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse base URL")
	}

	requestURL.Path += endpoint

	urlStr := requestURL.String()

	log.WithField("url", urlStr).Debug("Fetching beacon config")

	client := &http.Client{}

	req, err := http.NewRequest("GET", urlStr, http.NoBody)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch beacon config")
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, errors.Errorf("failed to fetch beacon config: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Data struct {
			GenesisValidatorsRoot      string `json:"GENESIS_VALIDATORS_ROOT"`
			GenesisEpoch               string `json:"GENESIS_EPOCH"`
			GenesisSlot                string `json:"GENESIS_SLOT"`
			GenesisTime                string `json:"GENESIS_TIME"`
			GenesisForkVersion         string `json:"GENESIS_FORK_VERSION"`
			CurrentForkVersion         string `json:"CURRENT_FORK_VERSION"`
			BlsToExecutionChangeDomain string `json:"BLS_TO_EXECUTION_CHANGE_DOMAIN"`
			VoluntaryExitDomain        string `json:"DOMAIN_VOLUNTARY_EXIT"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, errors.Wrap(err, "failed to decode beacon config")
	}

	config := &BeaconConfig{
		GenesisValidatorsRoot:      result.Data.GenesisValidatorsRoot,
		GenesisVersion:             result.Data.GenesisForkVersion,
		ExitForkVersion:            result.Data.CurrentForkVersion,
		CurrentForkVersion:         result.Data.CurrentForkVersion,
		BlsToExecutionChangeDomain: result.Data.BlsToExecutionChangeDomain,
		VoluntaryExitDomain:        result.Data.VoluntaryExitDomain,
	}

	log.WithField("config", config).Debug("Beacon config fetched")

	return config, nil
}
