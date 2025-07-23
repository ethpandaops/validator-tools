package validator

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/prysmaticlabs/prysm/v5/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/state"
	state_native "github.com/prysmaticlabs/prysm/v5/beacon-chain/state/state-native"
	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/primitives"
	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// VoluntaryExits represents a collection of voluntary exits for validators
type VoluntaryExits struct {
	WithdrawalCreds []byte
	ExitsByPubkey   map[string]*ValidatorExits
}

// ValidatorExits represents the state and exits for a validator
type ValidatorExits struct {
	State state.BeaconState
	Exits []*VoluntaryExit
}

// VoluntaryExit represents a single voluntary exit
type VoluntaryExit struct {
	PBExit *ethpb.SignedVoluntaryExit
	Pubkey []byte
	Path   string
}

// SignedVoluntaryExit represents the JSON structure of a signed voluntary exit
type SignedVoluntaryExit struct {
	Message struct {
		Epoch          string `json:"epoch"`
		ValidatorIndex string `json:"validator_index"`
	} `json:"message"`
	Signature string `json:"signature"`
}

type VerifyResponse struct {
	FirstIndex uint64 `json:"first_index"`
	LastIndex  uint64 `json:"last_index"`
}

// verifyWorkerInput represents work for a verification worker
type verifyWorkerInput struct {
	pubkey         string
	validatorExits *ValidatorExits
}

// verifyWorkerResult represents the result from a verification worker
type verifyWorkerResult struct {
	pubkey        string
	verifiedCount int
	err           error
}

// sharedVerifyState holds state shared between verification workers
type sharedVerifyState struct {
	mu          sync.Mutex
	initialized bool
	firstIndex  primitives.ValidatorIndex
	lastIndex   primitives.ValidatorIndex
}

// NewVoluntaryExits creates a new VoluntaryExits instance
func NewVoluntaryExits(path, network, withdrawalCreds string, expectedPubkeys []string) (*VoluntaryExits, error) {
	if err := setNetwork(network); err != nil {
		log.WithError(err).WithField("network", network).Error("Failed to set network")

		return nil, err
	}

	exitsByPubkey := make(map[string]*ValidatorExits)

	log.WithField("path", path).Info("Starting to read exit files from directory")

	files, err := os.ReadDir(path)
	if err != nil {
		log.WithError(err).WithField("path", path).Error("Failed to read directory")

		return nil, err
	}

	log.WithField("file_count", len(files)).Info("Found files in directory")

	// Create a map of expected pubkeys for quick lookup
	expectedPubkeyMap := make(map[string]bool)
	for _, pubkey := range expectedPubkeys {
		expectedPubkeyMap[strings.TrimPrefix(pubkey, "0x")] = true
	}

	log.WithField("expected_pubkeys", len(expectedPubkeys)).Info("Processing exit files")

	processedFiles := 0
	for _, file := range files {
		if !isExitFile(file) {
			continue
		}

		filePath := filepath.Join(path, file.Name())

		log.WithField("file", file.Name()).Debug("Reading exit file")

		vexit, rErr := readExitFile(filePath)
		if rErr != nil {
			log.WithError(rErr).WithField("file", file.Name()).Warn("Skipping file")

			continue
		}

		processedFiles++

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

	log.WithFields(logrus.Fields{
		"processed_files": processedFiles,
		"unique_pubkeys":  len(exitsByPubkey),
	}).Info("Finished reading exit files")

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

// setNetwork configures the network parameters
func setNetwork(network string) error {
	switch network {
	case "mainnet":
		params.OverrideBeaconConfig(params.MainnetConfig())
	case "holesky":
		params.OverrideBeaconConfig(params.HoleskyConfig())
	case "hoodi":
		params.OverrideBeaconConfig(params.HoodiConfig())
	default:
		return fmt.Errorf("unknown network: %s", network)
	}

	return nil
}

// isExitFile checks if a file is a JSON exit file
func isExitFile(file os.DirEntry) bool {
	return !file.IsDir() && strings.Contains(file.Name(), ".json")
}

// initializeExitState initializes the state for a validator's exits
func initializeExitState(exitsByPubkey map[string]*ValidatorExits, pubkeyStr string, vexit *VoluntaryExit) error {
	if _, exists := exitsByPubkey[pubkeyStr]; exists {
		return nil
	}

	slot := primitives.Slot(uint64(params.BeaconConfig().SlotsPerEpoch) * uint64(vexit.PBExit.Exit.Epoch))

	st, err := state_native.InitializeFromProtoDeneb(&ethpb.BeaconStateDeneb{
		Slot:                  slot,
		GenesisValidatorsRoot: params.BeaconConfig().GenesisValidatorsRoot[:],
	})
	if err != nil {
		return err
	}

	for i := primitives.ValidatorIndex(0); i < vexit.PBExit.Exit.ValidatorIndex; i++ {
		if err := st.AppendValidator(&ethpb.Validator{}); err != nil {
			return err
		}
	}

	exitsByPubkey[pubkeyStr] = &ValidatorExits{
		State: st,
		Exits: []*VoluntaryExit{},
	}

	return nil
}

// CheckCount validates the number and sequence of exits
func (e *VoluntaryExits) ValidateCount(numExits int) error {
	if len(e.ExitsByPubkey) == 0 {
		return fmt.Errorf("no voluntary exits found")
	}

	for pubkey, validatorExits := range e.ExitsByPubkey {
		total := uint64(validatorExits.Exits[len(validatorExits.Exits)-1].PBExit.Exit.ValidatorIndex - validatorExits.Exits[0].PBExit.Exit.ValidatorIndex + 1)

		if total != uint64(len(validatorExits.Exits)) {
			return fmt.Errorf("%d files found but expected %d for pubkey %s", len(validatorExits.Exits), total, pubkey)
		}

		if numExits > 0 && len(validatorExits.Exits) != numExits {
			return fmt.Errorf("expected %d exits for pubkey %s but found %d", numExits, pubkey, len(validatorExits.Exits))
		}
	}

	return nil
}

// validateIndicesMatch ensures the min and max validator indices match across all pubkeys
func (e *VoluntaryExits) ValidateIndices() error {
	if len(e.ExitsByPubkey) <= 1 {
		return nil // Nothing to compare with a single pubkey
	}

	var firstPubkey string

	var firstMin, firstMax primitives.ValidatorIndex

	// Initialize with the first pubkey's values
	for pubkey, validatorExits := range e.ExitsByPubkey {
		if len(validatorExits.Exits) == 0 {
			return fmt.Errorf("no exits found for pubkey %s", pubkey)
		}

		firstPubkey = pubkey
		firstMin = validatorExits.Exits[0].PBExit.Exit.ValidatorIndex
		firstMax = validatorExits.Exits[len(validatorExits.Exits)-1].PBExit.Exit.ValidatorIndex

		break
	}

	// Compare with all other pubkeys
	for pubkey, validatorExits := range e.ExitsByPubkey {
		if pubkey == firstPubkey {
			continue
		}

		if len(validatorExits.Exits) == 0 {
			return fmt.Errorf("no exits found for pubkey %s", pubkey)
		}

		currentMin := validatorExits.Exits[0].PBExit.Exit.ValidatorIndex
		currentMax := validatorExits.Exits[len(validatorExits.Exits)-1].PBExit.Exit.ValidatorIndex

		if currentMin != firstMin {
			return fmt.Errorf("minimum validator index mismatch: %d for pubkey %s vs %d for pubkey %s",
				currentMin, pubkey, firstMin, firstPubkey)
		}

		if currentMax != firstMax {
			return fmt.Errorf("maximum validator index mismatch: %d for pubkey %s vs %d for pubkey %s",
				currentMax, pubkey, firstMax, firstPubkey)
		}
	}

	return nil
}

// readExitFile reads and parses a voluntary exit file
func readExitFile(filePath string) (*VoluntaryExit, error) {
	parts := strings.Split(strings.TrimSuffix(filepath.Base(filePath), ".json"), "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid file name format: %s", filePath)
	}

	pubkey, err := hex.DecodeString(strings.TrimPrefix(parts[1], "0x"))
	if err != nil {
		log.WithError(err).WithField("file", filePath).Error("Invalid pubkey in filename")

		return nil, fmt.Errorf("invalid pubkey in filename: %s", filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.WithError(err).WithField("file", filePath).Error("Failed to read exit file")

		return nil, err
	}

	var signedExit SignedVoluntaryExit

	if uErr := json.Unmarshal(data, &signedExit); uErr != nil {
		log.WithError(uErr).WithField("file", filePath).Error("Failed to unmarshal exit file")

		return nil, uErr
	}

	epoch, err := strconv.ParseUint(signedExit.Message.Epoch, 10, 64)
	if err != nil {
		log.WithError(err).WithField("file", filePath).Error("Invalid epoch in exit file")

		return nil, err
	}

	validatorIndex, err := strconv.ParseUint(signedExit.Message.ValidatorIndex, 10, 64)
	if err != nil {
		log.WithError(err).WithField("file", filePath).Error("Invalid validator index in exit file")

		return nil, err
	}

	signature, err := hex.DecodeString(strings.TrimPrefix(signedExit.Signature, "0x"))
	if err != nil {
		log.WithError(err).WithField("file", filePath).Error("Invalid signature in exit file")

		return nil, err
	}

	return &VoluntaryExit{
		PBExit: &ethpb.SignedVoluntaryExit{
			Exit: &ethpb.VoluntaryExit{
				Epoch:          primitives.Epoch(epoch),
				ValidatorIndex: primitives.ValidatorIndex(validatorIndex),
			},
			Signature: signature,
		},
		Pubkey: pubkey,
		Path:   filePath,
	}, nil
}

// verifyPubkeyExits verifies all exits for a single pubkey
func (e *VoluntaryExits) verifyPubkeyExits(
	ctx context.Context,
	pubkey string,
	validatorExits *ValidatorExits,
	sharedState *sharedVerifyState,
) (int, error) {
	log := log.WithField("pubkey", pubkey)
	verifiedCount := 0

	log.WithField("exit_count", len(validatorExits.Exits)).Info("Starting verification for pubkey")

	// Update shared first/last index with synchronization
	if len(validatorExits.Exits) > 0 {
		sharedState.mu.Lock()
		if !sharedState.initialized {
			sharedState.firstIndex = validatorExits.Exits[0].PBExit.Exit.ValidatorIndex
			sharedState.lastIndex = validatorExits.Exits[len(validatorExits.Exits)-1].PBExit.Exit.ValidatorIndex
			sharedState.initialized = true
		}
		sharedState.mu.Unlock()
	}

	// Verify each exit sequentially for this pubkey
	for _, exit := range validatorExits.Exits {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return verifiedCount, ctx.Err()
		default:
		}

		if err := validatorExits.State.AppendValidator(&ethpb.Validator{
			PublicKey:             exit.Pubkey,
			WithdrawalCredentials: e.WithdrawalCreds,
			ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
		}); err != nil {
			log.WithError(err).WithField("validator_index", exit.PBExit.Exit.ValidatorIndex).Error("Failed to append validator")
			return verifiedCount, err
		}

		validator, err := validatorExits.State.ValidatorAtIndexReadOnly(exit.PBExit.Exit.ValidatorIndex)
		if err != nil {
			log.WithError(err).WithField("validator_index", exit.PBExit.Exit.ValidatorIndex).Error("Failed to get validator")
			return verifiedCount, err
		}

		if err := blocks.VerifyExitAndSignature(validator, validatorExits.State, exit.PBExit); err != nil {
			log.WithError(err).WithField("validator_index", exit.PBExit.Exit.ValidatorIndex).Error("Failed to verify exit and signature")
			return verifiedCount, err
		}

		verifiedCount++
		log.WithField("validator_index", exit.PBExit.Exit.ValidatorIndex).Debug("Exit verified")
	}

	log.WithFields(logrus.Fields{
		"verified": verifiedCount,
		"total":    len(validatorExits.Exits),
	}).Info("Completed verification for pubkey")

	return verifiedCount, nil
}

// Verify verifies all voluntary exits using concurrent workers
func (e *VoluntaryExits) Verify() (*VerifyResponse, error) {
	// Determine number of workers based on CPU count
	numWorkers := runtime.NumCPU()
	if numWorkers > len(e.ExitsByPubkey) {
		numWorkers = len(e.ExitsByPubkey)
	}

	log.WithFields(logrus.Fields{
		"workers": numWorkers,
		"pubkeys": len(e.ExitsByPubkey),
	}).Info("Starting verification with concurrent workers")

	// Initialize shared state
	sharedState := &sharedVerifyState{}

	// Create work channel
	workCh := make(chan verifyWorkerInput, len(e.ExitsByPubkey))

	// Queue all work
	for pubkey, validatorExits := range e.ExitsByPubkey {
		workCh <- verifyWorkerInput{
			pubkey:         pubkey,
			validatorExits: validatorExits,
		}
	}
	close(workCh)

	// Create error group with context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	// Results channel for collecting verification counts
	resultsCh := make(chan verifyWorkerResult, len(e.ExitsByPubkey))

	// Start workers
	for i := 0; i < numWorkers; i++ {
		workerID := i
		g.Go(func() error {
			log.WithField("worker_id", workerID).Debug("Worker started")
			for work := range workCh {
				log.WithFields(logrus.Fields{
					"worker_id": workerID,
					"pubkey":    work.pubkey,
				}).Debug("Worker processing pubkey")

				verifiedCount, err := e.verifyPubkeyExits(ctx, work.pubkey, work.validatorExits, sharedState)

				resultsCh <- verifyWorkerResult{
					pubkey:        work.pubkey,
					verifiedCount: verifiedCount,
					err:           err,
				}

				if err != nil {
					cancel() // Cancel all workers on error
					return err
				}
			}
			log.WithField("worker_id", workerID).Debug("Worker completed")
			return nil
		})
	}

	// Wait for all workers in a separate goroutine
	go func() {
		g.Wait()
		close(resultsCh)
	}()

	// Collect results
	totalVerified := 0
	for result := range resultsCh {
		if result.err != nil {
			return nil, result.err
		}
		totalVerified += result.verifiedCount
	}

	// Wait for all workers to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	log.WithField("total_verified", totalVerified).Info("All exits verified successfully")

	return &VerifyResponse{
		FirstIndex: uint64(sharedState.firstIndex),
		LastIndex:  uint64(sharedState.lastIndex),
	}, nil
}

func (e *VoluntaryExits) Extract(beaconURL, outputDir string) error {
	// Create a VoluntaryExitGenerator to use the FetchJSON method
	generator := &VoluntaryExitGenerator{BeaconURL: beaconURL}

	// Fetch validator data from beacon API
	resp, err := generator.FetchJSON(beaconURL + "/eth/v1/beacon/states/finalized/validators")
	if err != nil {
		log.WithError(err).Error("Failed to fetch validator data from beacon API")

		return err
	}

	// Parse API response
	var validatorResponse struct {
		Data []struct {
			Index     string `json:"index"`
			Validator struct {
				Pubkey                string `json:"pubkey"`
				WithdrawalCredentials string `json:"withdrawal_credentials"`
			} `json:"validator"`
			Status string `json:"status"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp, &validatorResponse); err != nil {
		log.WithError(err).Error("Failed to parse validator response")

		return err
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		log.WithError(err).WithField("output_dir", outputDir).Error("Failed to create output directory")

		return err
	}

	// Create map of pubkey -> validator info for active validators
	validatorMap := make(map[string]struct {
		index  string
		status string
	})

	for _, validator := range validatorResponse.Data {
		// Remove 0x prefix if present
		pubkey := strings.TrimPrefix(validator.Validator.Pubkey, "0x")
		validatorMap[pubkey] = struct {
			index  string
			status string
		}{
			index:  validator.Index,
			status: validator.Status,
		}
	}

	// Track which validators we've processed
	processedValidators := make(map[string]bool)

	// For each pubkey in our exit data, find the matching validator and copy files
	for pubkey, validatorExits := range e.ExitsByPubkey {
		validatorInfo, exists := validatorMap[pubkey]
		if !exists {
			return fmt.Errorf("validator with pubkey %s not found in beacon state", pubkey)
		}

		// Check if validator is active (can be active_ongoing, active_exiting, etc.)
		if !strings.HasPrefix(validatorInfo.status, "active") && validatorInfo.status != "pending_initialized" && validatorInfo.status != "pending_queued" {
			return fmt.Errorf("validator with pubkey %s is not active (status: %s)", pubkey, validatorInfo.status)
		}

		log.WithFields(logrus.Fields{
			"pubkey": pubkey,
			"index":  validatorInfo.index,
			"status": validatorInfo.status,
		}).Info("Extracting exit files for validator")

		// For each exit file for this validator
		for _, exit := range validatorExits.Exits {
			expectedIndex := fmt.Sprintf("%d", exit.PBExit.Exit.ValidatorIndex)

			// Verify the validator index matches what we expect
			if validatorInfo.index != expectedIndex {
				continue
			}

			// Find the source file
			sourceFileName := fmt.Sprintf("%s-%s.json", expectedIndex, pubkey)

			// Copy file to output directory
			destFilePath := filepath.Join(outputDir, sourceFileName)

			if err := copyFile(exit.Path, destFilePath); err != nil {
				log.WithError(err).WithFields(logrus.Fields{
					"source": exit.Path,
					"dest":   destFilePath,
				}).Error("Failed to copy file")

				return err
			}
		}

		processedValidators[pubkey] = true
	}

	// Verify all expected validators were processed
	for pubkey := range e.ExitsByPubkey {
		if !processedValidators[pubkey] {
			return fmt.Errorf("validator %s was not processed", pubkey)
		}
	}

	log.WithField("count", len(processedValidators)).Info("Successfully extracted all validator exit files")

	return nil
}

// copyFile copies a file from source to destination
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)

	return err
}
