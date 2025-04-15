package validator

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

// FetchJSON fetches and returns JSON data from a URL
func (g *VoluntaryExitGenerator) FetchJSON(url string) ([]byte, error) {
	log.Infof("Fetching JSON from URL: %s", url)

	req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		log.Errorf("Failed to create request: %v", err)

		return nil, errors.Wrap(err, "failed to create request")
	}

	client := &http.Client{
		Timeout: 10 * time.Minute,
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("Failed to fetch URL: %v", err)

		return nil, errors.Wrap(err, "failed to fetch URL")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Errorf("HTTP request failed with status: %s", resp.Status)

		return nil, errors.Errorf("HTTP request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Failed to read response body: %v", err)

		return nil, errors.Wrap(err, "failed to read response body")
	}

	log.Infof("Successfully fetched %d bytes", len(body))

	return body, nil
}

func (g *VoluntaryExitGenerator) FetchBeaconConfig() (*BeaconConfig, error) {
	log.Info("Fetching beacon config")

	config := &BeaconConfig{}

	log.Info("Fetching genesis data")

	genesisResp, err := g.FetchJSON(g.BeaconURL + "/eth/v1/beacon/genesis")
	if err != nil {
		log.Errorf("Failed to fetch genesis data: %v", err)

		return nil, err
	}

	var genesisData struct {
		Data struct {
			GenesisValidatorsRoot string `json:"genesis_validators_root"`
			GenesisForkVersion    string `json:"genesis_fork_version"`
		} `json:"data"`
	}

	if errGen := json.Unmarshal(genesisResp, &genesisData); errGen != nil {
		log.Errorf("Failed to parse genesis response: %v", errGen)

		return nil, errors.Wrap(errGen, "failed to parse genesis response")
	}

	config.GenesisValidatorsRoot = genesisData.Data.GenesisValidatorsRoot
	config.GenesisVersion = genesisData.Data.GenesisForkVersion
	log.Infof("Genesis validators root: %s", config.GenesisValidatorsRoot)
	log.Infof("Genesis version: %s", config.GenesisVersion)

	// Fetch fork data
	log.Info("Fetching fork data")

	forkResp, err := g.FetchJSON(g.BeaconURL + "/eth/v1/beacon/states/head/fork")
	if err != nil {
		log.Errorf("Failed to fetch fork data: %v", err)

		return nil, err
	}

	var forkData struct {
		Data struct {
			PreviousVersion string `json:"previous_version"`
			CurrentVersion  string `json:"current_version"`
		} `json:"data"`
	}

	if errFork := json.Unmarshal(forkResp, &forkData); errFork != nil {
		log.Errorf("Failed to parse fork response: %v", errFork)

		return nil, errors.Wrap(errFork, "failed to parse fork response")
	}

	config.ExitForkVersion = forkData.Data.PreviousVersion
	config.CurrentForkVersion = forkData.Data.CurrentVersion
	log.Infof("Exit fork version: %s", config.ExitForkVersion)
	log.Infof("Current fork version: %s", config.CurrentForkVersion)

	// Fetch spec data
	log.Info("Fetching spec data")

	specResp, err := g.FetchJSON(g.BeaconURL + "/eth/v1/config/spec")
	if err != nil {
		log.Errorf("Failed to fetch spec data: %v", err)

		return nil, err
	}

	var specData struct {
		Data struct {
			DomainBlsToExecutionChange       string `json:"DOMAIN_BLS_TO_EXECUTION_CHANGE"`
			DomainVoluntaryExit              string `json:"DOMAIN_VOLUNTARY_EXIT"`
			MinValidatorWithdrawabilityDelay string `json:"MIN_VALIDATOR_WITHDRAWABILITY_DELAY"`
			CapellaForkVersion               string `json:"CAPELLA_FORK_VERSION"`
			CapellaForkEpoch                 string `json:"CAPELLA_FORK_EPOCH"`
		} `json:"data"`
	}

	if errSpec := json.Unmarshal(specResp, &specData); errSpec != nil {
		log.Errorf("Failed to parse spec response: %v", errSpec)

		return nil, errors.Wrap(errSpec, "failed to parse spec response")
	}

	config.Epoch = specData.Data.CapellaForkEpoch
	config.ExitForkVersion = specData.Data.CapellaForkVersion

	// make sure config.Epoch is >= specData.Data.MinValidatorWithdrawabilityDelay
	if config.Epoch < specData.Data.MinValidatorWithdrawabilityDelay {
		config.Epoch = specData.Data.MinValidatorWithdrawabilityDelay
	}

	config.BlsToExecutionChangeDomain = specData.Data.DomainBlsToExecutionChange
	config.VoluntaryExitDomain = specData.Data.DomainVoluntaryExit
	log.Infof("BLS to execution change domain: %s", config.BlsToExecutionChangeDomain)
	log.Infof("Voluntary exit domain: %s", config.VoluntaryExitDomain)

	return config, nil
}

func (c *BeaconConfig) Validate() error {
	if c.GenesisValidatorsRoot == "" {
		return errors.New("genesis validators root is required")
	}

	if c.GenesisVersion == "" {
		return errors.New("genesis version is required")
	}

	if c.ExitForkVersion == "" {
		return errors.New("exit fork version is required")
	}

	if c.CurrentForkVersion == "" {
		return errors.New("current fork version is required")
	}

	if c.Epoch == "" {
		return errors.New("epoch is required")
	}

	if c.BlsToExecutionChangeDomain == "" {
		return errors.New("BLS to execution change domain is required")
	}

	if c.VoluntaryExitDomain == "" {
		return errors.New("voluntary exit domain is required")
	}

	return nil
}
