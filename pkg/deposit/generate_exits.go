package deposit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

type BeaconConfig struct {
	GenesisValidatorsRoot      string `json:"genesis_validators_root"`
	GenesisVersion             string `json:"genesis_fork_version"`
	ExitForkVersion            string `json:"exit_fork_version"`
	CurrentForkVersion         string `json:"current_fork_version"`
	Epoch                      string `json:"epoch"`
	BlsToExecutionChangeDomain string `json:"bls_to_execution_change_domain_type"`
	VoluntaryExitDomain        string `json:"voluntary_exit_domain_type"`
}

type ValidatorInfo struct {
	Index                 string `json:"index"`
	Pubkey                string `json:"pubkey"`
	State                 string `json:"state"`
	WithdrawalCredentials string `json:"withdrawal_credentials"`
}

type PrepFile struct {
	Version                    string          `json:"version"`
	Validators                 []ValidatorInfo `json:"validators"`
	GenesisValidatorsRoot      string          `json:"genesis_validators_root"`
	Epoch                      string          `json:"epoch"`
	GenesisVersion             string          `json:"genesis_fork_version"`
	ExitForkVersion            string          `json:"exit_fork_version"`
	CurrentForkVersion         string          `json:"current_fork_version"`
	BlsToExecutionChangeDomain string          `json:"bls_to_execution_change_domain_type"`
	VoluntaryExitDomain        string          `json:"voluntary_exit_domain_type"`
}

type ExitGenerator struct {
	outputDir             string
	withdrawalCredentials string
	passphrase            string
	beaconURL             string
	iterations            int
	validatorStartIndex   int
}

func NewExitGenerator(outputDir, withdrawalCreds, passphrase, beaconURL string, iterations, startIndex int) *ExitGenerator {
	log.Info("Creating new ExitGenerator")
	log.Infof("Output dir: %s", outputDir)
	log.Infof("Withdrawal creds: %s", withdrawalCreds)
	log.Infof("Beacon URL: %s", beaconURL)
	log.Infof("Iterations: %d", iterations)
	log.Infof("Start index: %d", startIndex)

	return &ExitGenerator{
		outputDir:             outputDir,
		withdrawalCredentials: withdrawalCreds,
		passphrase:            passphrase,
		beaconURL:             beaconURL,
		iterations:            iterations,
		validatorStartIndex:   startIndex,
	}
}

func (g *ExitGenerator) fetchJSON(url string) ([]byte, error) {
	log.Infof("Fetching JSON from URL: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		log.Errorf("Failed to fetch URL: %v", err)
		return nil, errors.Wrap(err, "failed to fetch URL")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Errorf("HTTP request failed with status: %s", resp.Status)
		return nil, fmt.Errorf("HTTP request failed with status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Failed to read response body: %v", err)
		return nil, errors.Wrap(err, "failed to read response body")
	}

	log.Infof("Successfully fetched %d bytes", len(body))
	return body, nil
}

func (g *ExitGenerator) GetValidatorStartIndex() (int, error) {
	log.Info("Getting validator start index")

	if g.validatorStartIndex >= 0 {
		log.Infof("Using provided start index: %d", g.validatorStartIndex)
		return g.validatorStartIndex, nil
	}

	resp, err := g.fetchJSON(g.beaconURL + "/eth/v1/beacon/states/head/validators")
	if err != nil {
		log.Errorf("Failed to fetch validators: %v", err)
		return 0, err
	}

	var result struct {
		Data []struct {
			Index string `json:"index"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		log.Errorf("Failed to parse validator response: %v", err)
		return 0, errors.Wrap(err, "failed to parse validator response")
	}

	maxIndex := -1
	for _, v := range result.Data {
		index, err := strconv.Atoi(v.Index)
		if err != nil {
			log.Infof("Skipping invalid index: %s", v.Index)
			continue
		}
		if index > maxIndex {
			maxIndex = index
		}
	}

	if maxIndex == -1 {
		log.Error("No valid validator indices found")
		return 0, errors.New("no valid validator indices found")
	}

	log.Infof("Found max validator index: %d", maxIndex)
	return maxIndex, nil
}

func (g *ExitGenerator) FetchBeaconConfig() (*BeaconConfig, error) {
	log.Info("Fetching beacon config")
	config := &BeaconConfig{}

	// Fetch genesis data
	log.Info("Fetching genesis data")
	genesisResp, err := g.fetchJSON(g.beaconURL + "/eth/v1/beacon/genesis")
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
	if err := json.Unmarshal(genesisResp, &genesisData); err != nil {
		log.Errorf("Failed to parse genesis response: %v", err)
		return nil, errors.Wrap(err, "failed to parse genesis response")
	}

	config.GenesisValidatorsRoot = genesisData.Data.GenesisValidatorsRoot
	config.GenesisVersion = genesisData.Data.GenesisForkVersion
	log.Infof("Genesis validators root: %s", config.GenesisValidatorsRoot)
	log.Infof("Genesis version: %s", config.GenesisVersion)

	// Fetch fork data
	log.Info("Fetching fork data")
	forkResp, err := g.fetchJSON(g.beaconURL + "/eth/v1/beacon/states/head/fork")
	if err != nil {
		log.Errorf("Failed to fetch fork data: %v", err)
		return nil, err
	}

	var forkData struct {
		Data struct {
			Epoch           string `json:"epoch"`
			PreviousVersion string `json:"previous_version"`
			CurrentVersion  string `json:"current_version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(forkResp, &forkData); err != nil {
		log.Errorf("Failed to parse fork response: %v", err)
		return nil, errors.Wrap(err, "failed to parse fork response")
	}

	config.Epoch = forkData.Data.Epoch
	config.ExitForkVersion = forkData.Data.PreviousVersion
	config.CurrentForkVersion = forkData.Data.CurrentVersion
	log.Infof("Epoch: %s", config.Epoch)
	log.Infof("Exit fork version: %s", config.ExitForkVersion)
	log.Infof("Current fork version: %s", config.CurrentForkVersion)

	// Fetch spec data
	log.Info("Fetching spec data")
	specResp, err := g.fetchJSON(g.beaconURL + "/eth/v1/config/spec")
	if err != nil {
		log.Errorf("Failed to fetch spec data: %v", err)
		return nil, err
	}

	var specData struct {
		Data struct {
			DomainBlsToExecutionChange string `json:"DOMAIN_BLS_TO_EXECUTION_CHANGE"`
			DomainVoluntaryExit        string `json:"DOMAIN_VOLUNTARY_EXIT"`
		} `json:"data"`
	}
	if err := json.Unmarshal(specResp, &specData); err != nil {
		log.Errorf("Failed to parse spec response: %v", err)
		return nil, errors.Wrap(err, "failed to parse spec response")
	}

	config.BlsToExecutionChangeDomain = specData.Data.DomainBlsToExecutionChange
	config.VoluntaryExitDomain = specData.Data.DomainVoluntaryExit
	log.Infof("BLS to execution change domain: %s", config.BlsToExecutionChangeDomain)
	log.Infof("Voluntary exit domain: %s", config.VoluntaryExitDomain)

	return config, nil
}

func (g *ExitGenerator) GenerateExits(keystorePath string, config *BeaconConfig, startIndex int) error {
	log.Info("Generating exits")
	log.Infof("Keystore path: %s", keystorePath)
	log.Infof("Start index: %d", startIndex)

	// Get absolute path for keystore
	absKeystorePath, err := filepath.Abs(keystorePath)
	if err != nil {
		log.Errorf("Failed to get absolute path for keystore: %v", err)
		return errors.Wrapf(err, "failed to get absolute path for keystore: %s", keystorePath)
	}
	log.Infof("Absolute keystore path: %s", absKeystorePath)

	// Read pubkey from keystore
	log.Info("Reading pubkey from keystore")
	keystoreData, err := os.ReadFile(absKeystorePath)
	if err != nil {
		log.Errorf("Failed to read keystore file: %v", err)
		return errors.Wrapf(err, "failed to read keystore file: %s", absKeystorePath)
	}

	var keystoreJSON struct {
		Pubkey string `json:"pubkey"`
	}
	if err := json.Unmarshal(keystoreData, &keystoreJSON); err != nil {
		log.Errorf("Failed to parse keystore JSON: %v", err)
		return errors.Wrapf(err, "failed to parse keystore JSON: %s", absKeystorePath)
	}

	if keystoreJSON.Pubkey == "" {
		log.Error("Empty or null pubkey in keystore")
		return fmt.Errorf("empty or null pubkey in keystore: %s", absKeystorePath)
	}
	log.Infof("Pubkey: %s", keystoreJSON.Pubkey)

	for i := 1; i <= g.iterations; i++ {
		log.Infof("Processing iteration %d/%d", i, g.iterations)

		validatorIndex := startIndex + i
		log.Infof("Validator index: %d", validatorIndex)

		prepFile := PrepFile{
			Version: "3",
			Validators: []ValidatorInfo{
				{
					Index:                 strconv.Itoa(validatorIndex),
					Pubkey:                keystoreJSON.Pubkey,
					State:                 "active_ongoing",
					WithdrawalCredentials: g.withdrawalCredentials,
				},
			},
			GenesisValidatorsRoot:      config.GenesisValidatorsRoot,
			Epoch:                      config.Epoch,
			GenesisVersion:             config.GenesisVersion,
			ExitForkVersion:            config.ExitForkVersion,
			CurrentForkVersion:         config.CurrentForkVersion,
			BlsToExecutionChangeDomain: config.BlsToExecutionChangeDomain,
			VoluntaryExitDomain:        config.VoluntaryExitDomain,
		}

		prepFilePath := filepath.Join(g.outputDir, "offline-preparation.json")
		log.Infof("Preparation file path: %s", prepFilePath)

		prepFileData, err := json.MarshalIndent(prepFile, "", "  ")
		if err != nil {
			log.Errorf("Failed to marshal preparation file: %v", err)
			return errors.Wrap(err, "failed to marshal preparation file")
		}

		if err := os.WriteFile(prepFilePath, prepFileData, 0644); err != nil {
			log.Errorf("Failed to write preparation file: %v", err)
			return errors.Wrap(err, "failed to write preparation file")
		}

		outFile := filepath.Join(g.outputDir, fmt.Sprintf("%d-%s.json", validatorIndex, keystoreJSON.Pubkey))
		log.Infof("Output file: %s", outFile)

		if err := g.runEthdoCommand(absKeystorePath, outFile); err != nil {
			fmt.Println() // New line after progress
			log.Errorf("Failed to run ethdo command: %v", err)
			return err
		}

		if err := os.Remove(prepFilePath); err != nil {
			log.Warnf("Failed to remove preparation file: %v", err)
			fmt.Printf("\nWarning: Failed to remove preparation file: %v\n", err)
		}
	}
	fmt.Println() // New line after progress bar
	log.Info("Exit generation completed successfully")

	return nil
}

func (g *ExitGenerator) runEthdoCommand(keystorePath, outFile string) error {
	log.Info("Running ethdo command")
	log.Infof("Keystore path: %s", keystorePath)
	log.Infof("Output file: %s", outFile)

	args := []string{
		"validator", "exit",
		"--validator=" + keystorePath,
		"--passphrase=" + g.passphrase,
		"--json",
		"--offline",
	}

	// Redact passphrase in debug output
	debugArgs := make([]string, len(args))
	copy(debugArgs, args)
	for i, arg := range debugArgs {
		if strings.HasPrefix(arg, "--passphrase=") {
			debugArgs[i] = "--passphrase=********"
		}
	}

	fmt.Printf("\nExecuting command: ethdo %s\n", strings.Join(debugArgs, " "))
	log.Infof("Executing command: ethdo %s", strings.Join(debugArgs, " "))

	cmd := NewCommand("ethdo", args)
	cmd.Dir = g.outputDir
	log.Infof("Working directory: %s", g.outputDir)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("ethdo command failed: %v", err)
		log.Errorf("Command output: %s", string(output))
		return errors.Wrapf(err, "ethdo command failed for keystore: %s: %s",
			keystorePath, string(output))
	}

	if err := os.WriteFile(outFile, output, 0644); err != nil {
		log.Errorf("Failed to write output file: %v", err)
		return errors.Wrapf(err, "failed to write output file: %s", outFile)
	}
	log.Info("ethdo command completed successfully")

	return nil
}
