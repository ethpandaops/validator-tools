package validator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

// processExitTasks processes validator exit tasks using a worker pool
func (g *VoluntaryExitGenerator) processExitTasks(tasks chan exitTask, config *BeaconConfig, keystoreNum int32) error {
	var wg sync.WaitGroup

	errChan := make(chan error, g.NumWorkers)

	var completedExits uint64

	// Start progress reporter
	stopProgress := make(chan struct{})
	go g.reportProgress(keystoreNum, &completedExits, stopProgress)

	// Start workers
	for i := 0; i < g.NumWorkers; i++ {
		wg.Add(1)

		go g.worker(i, tasks, &wg, errChan, &completedExits, config)
	}

	// Wait for all workers to complete
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

	return nil
}

// worker processes exit tasks
func (g *VoluntaryExitGenerator) worker(id int, tasks chan exitTask, wg *sync.WaitGroup, errChan chan error, completedExits *uint64, config *BeaconConfig) {
	defer wg.Done()

	// Create temporary directory for this worker
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("ethdo-worker-%d-", id))
	if err != nil {
		errChan <- errors.Wrapf(err, "worker %d failed to create temp directory", id)

		return
	}

	defer os.RemoveAll(tmpDir)

	workerLog := log.WithField("worker", id)
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
					WithdrawalCredentials: g.WithdrawalCredentials,
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
				id, task.validatorIndex)

			return
		}

		if err := os.WriteFile(prepFilePath, prepFileData, 0o600); err != nil {
			errChan <- errors.Wrapf(err, "worker %d failed to write preparation file for index %d",
				id, task.validatorIndex)

			return
		}

		outFile := filepath.Join(g.OutputDir, fmt.Sprintf("%d-%s.json", task.validatorIndex, task.pubkey))
		if err := g.runEthdoCommand(task.keystorePath, outFile, tmpDir, workerLog); err != nil {
			errChan <- errors.Wrapf(err, "worker %d failed to run ethdo for index %d",
				id, task.validatorIndex)

			return
		}

		atomic.AddUint64(completedExits, 1)
		workerLog.Debugf("Completed validator index %d", task.validatorIndex)
	}
}

// reportProgress reports progress of exit generation
func (g *VoluntaryExitGenerator) reportProgress(keystoreNum int32, completedExits *uint64, stop chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			completed := atomic.LoadUint64(completedExits)
			log.Infof("Progress: Keystore %d/%d - %d/%d exits generated (%.1f%%)",
				keystoreNum, g.TotalKeystores,
				completed, g.Iterations,
				float64(completed)*100/float64(g.Iterations))
		case <-stop:
			return
		}
	}
}
