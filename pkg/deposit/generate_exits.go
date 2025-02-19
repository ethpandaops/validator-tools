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
	"sync"
	"sync/atomic"
	"time"

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
	numWorkers            int
	totalKeystores        int
	currentKeystore       int32
}

func NewExitGenerator(outputDir, withdrawalCreds, passphrase, beaconURL string, iterations, startIndex, numWorkers int) *ExitGenerator {
	log.Info("Creating new ExitGenerator")
	log.Infof("Output dir: %s", outputDir)
	log.Infof("Withdrawal creds: %s", withdrawalCreds)
	log.Infof("Beacon URL: %s", beaconURL)
	log.Infof("Iterations: %d", iterations)
	if startIndex >= 0 {
		log.Infof("Start index: %d", startIndex)
	}
	log.Infof("Number of workers: %d", numWorkers)

	return &ExitGenerator{
		outputDir:             outputDir,
		withdrawalCredentials: withdrawalCreds,
		passphrase:            passphrase,
		beaconURL:             beaconURL,
		iterations:            iterations,
		validatorStartIndex:   startIndex,
		numWorkers:            numWorkers,
		currentKeystore:       0,
		totalKeystores:        0,
	}
}

// SetTotalKeystores sets the total number of keystores to be processed
func (g *ExitGenerator) SetTotalKeystores(total int) {
	g.totalKeystores = total
	log.Infof("Total keystores to process: %d", total)
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

type exitTask struct {
	validatorIndex int
	pubkey         string
	keystorePath   string
}

func (g *ExitGenerator) GenerateExits(keystorePath string, config *BeaconConfig, startIndex int) error {
	atomic.AddInt32(&g.currentKeystore, 1)
	keystoreNum := atomic.LoadInt32(&g.currentKeystore)

	log.Info("Generating exits")
	log.Infof("Processing keystore %d/%d: %s", keystoreNum, g.totalKeystores, keystorePath)
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

	// Create task channel and worker pool
	tasks := make(chan exitTask, g.iterations)
	var wg sync.WaitGroup
	errChan := make(chan error, g.numWorkers)

	// Counter for completed exits
	var completedExits uint64

	// Start progress reporter
	stopProgress := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				completed := atomic.LoadUint64(&completedExits)
				log.Infof("Progress: Keystore %d/%d - %d/%d exits generated (%.1f%%)",
					keystoreNum, g.totalKeystores,
					completed, g.iterations,
					float64(completed)*100/float64(g.iterations))
			case <-stopProgress:
				return
			}
		}
	}()

	// Start workers
	for i := 0; i < g.numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Create temporary directory for this worker
			tmpDir, err := os.MkdirTemp("", fmt.Sprintf("ethdo-worker-%d-", workerID))
			if err != nil {
				errChan <- errors.Wrapf(err, "worker %d failed to create temp directory", workerID)
				return
			}
			defer os.RemoveAll(tmpDir)

			workerLog := log.WithField("worker", workerID)
			workerLog.Debugf("Worker started with temp dir: %s", tmpDir)

			for task := range tasks {
				workerLog.Debugf("Processing validator index %d", task.validatorIndex)

				prepFile := PrepFile{
					Version: "3",
					Validators: []ValidatorInfo{
						{
							Index:                 strconv.Itoa(task.validatorIndex),
							Pubkey:                task.pubkey,
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

				prepFilePath := filepath.Join(tmpDir, "offline-preparation.json")
				prepFileData, err := json.MarshalIndent(prepFile, "", "  ")
				if err != nil {
					errChan <- errors.Wrapf(err, "worker %d failed to marshal preparation file for index %d",
						workerID, task.validatorIndex)
					return
				}

				if err := os.WriteFile(prepFilePath, prepFileData, 0644); err != nil {
					errChan <- errors.Wrapf(err, "worker %d failed to write preparation file for index %d",
						workerID, task.validatorIndex)
					return
				}

				outFile := filepath.Join(g.outputDir, fmt.Sprintf("%d-%s.json", task.validatorIndex, task.pubkey))
				if err := g.runEthdoCommand(task.keystorePath, outFile, tmpDir, workerLog); err != nil {
					errChan <- errors.Wrapf(err, "worker %d failed to run ethdo for index %d",
						workerID, task.validatorIndex)
					return
				}

				atomic.AddUint64(&completedExits, 1)
				workerLog.Debugf("Completed validator index %d", task.validatorIndex)
			}
		}(i)
	}

	// Send tasks to workers
	log.Info("Sending tasks to workers")
	for i := 1; i <= g.iterations; i++ {
		tasks <- exitTask{
			validatorIndex: startIndex + i,
			pubkey:         keystoreJSON.Pubkey,
			keystorePath:   absKeystorePath,
		}
	}
	close(tasks)

	// Wait for all workers to complete
	log.Info("Waiting for workers to complete")
	wg.Wait()
	close(errChan)
	close(stopProgress)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			log.Error("Worker error encountered:", err)
			return err
		}
	}

	log.Infof("Exit generation completed for keystore %d/%d", keystoreNum, g.totalKeystores)
	return nil
}

func (g *ExitGenerator) runEthdoCommand(keystorePath, outFile, workDir string, log *logrus.Entry) error {
	log.Debug("Running ethdo command")
	log.Debugf("Keystore path: %s", keystorePath)
	log.Debugf("Output file: %s", outFile)
	log.Debugf("Working directory: %s", workDir)

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
	log.Debugf("Executing command: ethdo %s", strings.Join(debugArgs, " "))

	cmd := NewCommand("ethdo", args)
	cmd.Dir = workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("ethdo command failed: %v", err)
		log.Errorf("Command output: %s", string(output))
		return errors.Wrapf(err, "ethdo command failed: %s", string(output))
	}

	if err := os.WriteFile(outFile, output, 0644); err != nil {
		log.Errorf("Failed to write output file: %v", err)
		return errors.Wrapf(err, "failed to write output file: %s", outFile)
	}
	log.Debug("ethdo command completed successfully")

	return nil
}
