package validator

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/prysmaticlabs/prysm/v5/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/state"
	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/primitives"
	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"github.com/sirupsen/logrus"
)

// fileReadResult represents the result of reading an exit file
type fileReadResult struct {
	exit     *VoluntaryExit
	err      error
	fileName string
}

// verifyResult represents the result of verifying an exit
type verifyResult struct {
	index primitives.ValidatorIndex
	err   error
}

// NewVoluntaryExitsOptimized creates a new VoluntaryExits instance with parallel file reading
func NewVoluntaryExitsOptimized(path, network, withdrawalCreds string, expectedPubkeys []string) (*VoluntaryExits, error) {
	if err := setNetwork(network); err != nil {
		log.WithError(err).WithField("network", network).Error("Failed to set network")
		return nil, err
	}

	// Create a map of expected pubkeys for quick lookup
	expectedPubkeyMap := make(map[string]bool)
	for _, pubkey := range expectedPubkeys {
		expectedPubkeyMap[strings.TrimPrefix(pubkey, "0x")] = true
	}

	files, err := os.ReadDir(path)
	if err != nil {
		log.WithError(err).WithField("path", path).Error("Failed to read directory")
		return nil, err
	}

	// Filter exit files
	var exitFiles []os.DirEntry
	for _, file := range files {
		if isExitFile(file) {
			exitFiles = append(exitFiles, file)
		}
	}

	// Determine number of workers
	numWorkers := runtime.NumCPU()
	if len(exitFiles) < numWorkers {
		numWorkers = len(exitFiles)
	}

	// Create channels for work distribution
	fileChan := make(chan string, len(exitFiles))
	resultChan := make(chan fileReadResult, len(exitFiles))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range fileChan {
				vexit, err := readExitFile(filePath)
				resultChan <- fileReadResult{
					exit:     vexit,
					err:      err,
					fileName: filepath.Base(filePath),
				}
			}
		}()
	}

	// Send work to workers
	for _, file := range exitFiles {
		fileChan <- filepath.Join(path, file.Name())
	}
	close(fileChan)

	// Wait for workers to complete and close result channel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	exitsByPubkey := make(map[string]*ValidatorExits)
	var mu sync.Mutex

	for result := range resultChan {
		if result.err != nil {
			log.WithError(result.err).WithField("file", result.fileName).Warn("Skipping file")
			continue
		}

		pubkeyStr := hex.EncodeToString(result.exit.Pubkey)

		// Check if pubkey is in expected list
		if !expectedPubkeyMap[pubkeyStr] {
			return nil, fmt.Errorf("unexpected pubkey found: %s", pubkeyStr)
		}

		mu.Lock()
		if err := initializeExitState(exitsByPubkey, pubkeyStr, result.exit); err != nil {
			mu.Unlock()
			log.WithError(err).WithField("pubkey", pubkeyStr).Error("Failed to initialize exit state")
			return nil, err
		}
		exitsByPubkey[pubkeyStr].Exits = append(exitsByPubkey[pubkeyStr].Exits, result.exit)
		mu.Unlock()
	}

	// Check if all expected pubkeys were found
	for pubkey := range expectedPubkeyMap {
		if _, found := exitsByPubkey[pubkey]; !found {
			return nil, fmt.Errorf("expected pubkey not found: %s", pubkey)
		}
	}

	creds, err := hex.DecodeString(strings.TrimPrefix(withdrawalCreds, "0x"))
	if err != nil {
		log.WithError(err).WithField("withdrawal_creds", withdrawalCreds).Error("Failed to decode withdrawal credentials")
		return nil, err
	}

	return &VoluntaryExits{
		WithdrawalCreds: creds,
		ExitsByPubkey:   exitsByPubkey,
	}, nil
}

// VerifyOptimized verifies all voluntary exits with concurrent signature verification
func (e *VoluntaryExits) VerifyOptimized() (*VerifyResponse, error) {
	var firstIndex, lastIndex primitives.ValidatorIndex
	var initialized bool

	// Determine number of workers for verification
	numWorkers := runtime.NumCPU()
	totalExits := 0
	for _, validatorExits := range e.ExitsByPubkey {
		totalExits += len(validatorExits.Exits)
	}
	if totalExits < numWorkers {
		numWorkers = totalExits
	}

	for pubkey, validatorExits := range e.ExitsByPubkey {
		log := log.WithField("pubkey", pubkey)

		if !initialized && len(validatorExits.Exits) > 0 {
			firstIndex = validatorExits.Exits[0].PBExit.Exit.ValidatorIndex
			lastIndex = validatorExits.Exits[len(validatorExits.Exits)-1].PBExit.Exit.ValidatorIndex
			initialized = true
		}

		// First, append all validators to the state
		for _, exit := range validatorExits.Exits {
			if err := validatorExits.State.AppendValidator(&ethpb.Validator{
				PublicKey:             exit.Pubkey,
				WithdrawalCredentials: e.WithdrawalCreds,
				ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
			}); err != nil {
				log.WithError(err).WithField("validator_index", exit.PBExit.Exit.ValidatorIndex).Error("Failed to append validator")
				return nil, err
			}
		}

		// Create channels for concurrent verification
		type verifyJob struct {
			exit  *VoluntaryExit
			state state.BeaconState
		}
		jobChan := make(chan verifyJob, len(validatorExits.Exits))
		resultChan := make(chan verifyResult, len(validatorExits.Exits))

		// Start verification workers
		var wg sync.WaitGroup
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobChan {
					validator, err := job.state.ValidatorAtIndexReadOnly(job.exit.PBExit.Exit.ValidatorIndex)
					if err != nil {
						resultChan <- verifyResult{
							index: job.exit.PBExit.Exit.ValidatorIndex,
							err:   fmt.Errorf("failed to get validator: %w", err),
						}
						continue
					}

					if err := blocks.VerifyExitAndSignature(validator, job.state, job.exit.PBExit); err != nil {
						resultChan <- verifyResult{
							index: job.exit.PBExit.Exit.ValidatorIndex,
							err:   fmt.Errorf("failed to verify exit and signature: %w", err),
						}
						continue
					}

					resultChan <- verifyResult{
						index: job.exit.PBExit.Exit.ValidatorIndex,
						err:   nil,
					}
				}
			}()
		}

		// Send verification jobs
		for _, exit := range validatorExits.Exits {
			jobChan <- verifyJob{
				exit:  exit,
				state: validatorExits.State,
			}
		}
		close(jobChan)

		// Wait for workers and close result channel
		go func() {
			wg.Wait()
			close(resultChan)
		}()

		// Collect results
		verifiedCount := 0
		for result := range resultChan {
			if result.err != nil {
				log.WithError(result.err).WithField("validator_index", result.index).Error("Verification failed")
				return nil, result.err
			}
			verifiedCount++
			log.WithField("validator_index", result.index).Debug("Exit verified")
		}

		log.WithFields(logrus.Fields{
			"verified": verifiedCount,
			"total":    len(validatorExits.Exits),
		}).Info("Exits verified")
	}

	return &VerifyResponse{
		FirstIndex: uint64(firstIndex),
		LastIndex:  uint64(lastIndex),
	}, nil
}

// NewVoluntaryExitsSequential is the original sequential implementation for benchmarking comparison
func NewVoluntaryExitsSequential(path, network, withdrawalCreds string, expectedPubkeys []string) (*VoluntaryExits, error) {
	if err := setNetwork(network); err != nil {
		log.WithError(err).WithField("network", network).Error("Failed to set network")
		return nil, err
	}

	exitsByPubkey := make(map[string]*ValidatorExits)

	files, err := os.ReadDir(path)
	if err != nil {
		log.WithError(err).WithField("path", path).Error("Failed to read directory")
		return nil, err
	}

	// Create a map of expected pubkeys for quick lookup
	expectedPubkeyMap := make(map[string]bool)
	for _, pubkey := range expectedPubkeys {
		expectedPubkeyMap[strings.TrimPrefix(pubkey, "0x")] = true
	}

	for _, file := range files {
		if !isExitFile(file) {
			continue
		}

		filePath := filepath.Join(path, file.Name())

		vexit, rErr := readExitFile(filePath)
		if rErr != nil {
			log.WithError(rErr).WithField("file", file.Name()).Warn("Skipping file")
			continue
		}

		pubkeyStr := hex.EncodeToString(vexit.Pubkey)

		// Check if pubkey is in expected list
		if !expectedPubkeyMap[pubkeyStr] {
			return nil, fmt.Errorf("unexpected pubkey found: %s", pubkeyStr)
		}

		if iErr := initializeExitState(exitsByPubkey, pubkeyStr, vexit); iErr != nil {
			log.WithError(iErr).WithField("pubkey", pubkeyStr).Error("Failed to initialize exit state")
			return nil, iErr
		}

		exitsByPubkey[pubkeyStr].Exits = append(exitsByPubkey[pubkeyStr].Exits, vexit)
	}

	// Check if all expected pubkeys were found
	for pubkey := range expectedPubkeyMap {
		if _, found := exitsByPubkey[pubkey]; !found {
			return nil, fmt.Errorf("expected pubkey not found: %s", pubkey)
		}
	}

	creds, err := hex.DecodeString(strings.TrimPrefix(withdrawalCreds, "0x"))
	if err != nil {
		log.WithError(err).WithField("withdrawal_creds", withdrawalCreds).Error("Failed to decode withdrawal credentials")
		return nil, err
	}

	return &VoluntaryExits{
		WithdrawalCreds: creds,
		ExitsByPubkey:   exitsByPubkey,
	}, nil
}