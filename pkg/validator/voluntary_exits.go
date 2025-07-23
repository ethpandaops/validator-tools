package validator

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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

// VoluntaryExits represents a memory-optimized voluntary exits processor
type VoluntaryExits struct {
	WithdrawalCreds []byte
	Metadata        PreValidationResult
	BatchSize       int
	network         string
}

// ValidatorExits represents the state and exits for a validator
type ValidatorExits struct {
	State state.BeaconState
	Exits []*VoluntaryExit
}

// ExitFileInfo represents metadata about an exit file without loading its content
type ExitFileInfo struct {
	Path           string
	Pubkey         string
	ValidatorIndex uint64
}

// PreValidationResult contains the result of the pre-validation phase
type PreValidationResult struct {
	FilesByPubkey map[string][]ExitFileInfo
	MinIndex      uint64
	MaxIndex      uint64
}

// IndexRange represents the min and max validator indices for a pubkey
type IndexRange struct {
	Min uint64
	Max uint64
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

// NewVoluntaryExits creates a new VoluntaryExits instance with memory-optimized processing
func NewVoluntaryExits(path, network, withdrawalCreds string, expectedPubkeys []string) (*VoluntaryExits, error) {
	if err := setNetwork(network); err != nil {
		log.WithError(err).WithField("network", network).Error("Failed to set network")
		return nil, err
	}

	creds, err := hex.DecodeString(strings.TrimPrefix(withdrawalCreds, "0x"))
	if err != nil {
		log.WithError(err).WithField("withdrawal_creds", withdrawalCreds).Error("Failed to decode withdrawal credentials")
		return nil, err
	}

	// Perform pre-validation
	metadata, err := preValidateExits(path, expectedPubkeys)
	if err != nil {
		return nil, err
	}

	// Default batch size to number of CPUs
	batchSize := runtime.NumCPU()

	return &VoluntaryExits{
		WithdrawalCreds: creds,
		Metadata:        metadata,
		BatchSize:       batchSize,
		network:         network,
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

// preValidateExits performs lightweight validation without loading exit data
func preValidateExits(path string, expectedPubkeys []string) (PreValidationResult, error) {
	result := PreValidationResult{
		FilesByPubkey: make(map[string][]ExitFileInfo),
		MinIndex:      ^uint64(0), // Max uint64
		MaxIndex:      0,
	}

	log.WithField("path", path).Info("Starting pre-validation of exit files")

	files, err := os.ReadDir(path)
	if err != nil {
		log.WithError(err).WithField("path", path).Error("Failed to read directory")
		return result, err
	}

	// Create a map of expected pubkeys for quick lookup
	expectedPubkeyMap := make(map[string]bool)
	for _, pubkey := range expectedPubkeys {
		expectedPubkeyMap[strings.TrimPrefix(pubkey, "0x")] = true
	}

	// Scan files and extract metadata
	for _, file := range files {
		if !isExitFile(file) {
			continue
		}

		// Parse filename to extract pubkey and validator index
		parts := strings.Split(strings.TrimSuffix(file.Name(), ".json"), "-")
		if len(parts) != 2 {
			log.WithField("file", file.Name()).Warn("Invalid file name format, skipping")
			continue
		}

		validatorIndex, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			log.WithError(err).WithField("file", file.Name()).Warn("Invalid validator index in filename, skipping")
			continue
		}

		pubkeyStr := strings.TrimPrefix(parts[1], "0x")

		// Check if pubkey is expected
		if !expectedPubkeyMap[pubkeyStr] {
			return result, fmt.Errorf("unexpected pubkey found: %s", pubkeyStr)
		}

		fileInfo := ExitFileInfo{
			Path:           filepath.Join(path, file.Name()),
			Pubkey:         pubkeyStr,
			ValidatorIndex: validatorIndex,
		}

		result.FilesByPubkey[pubkeyStr] = append(result.FilesByPubkey[pubkeyStr], fileInfo)

		// Update global min/max indices
		if validatorIndex < result.MinIndex {
			result.MinIndex = validatorIndex
		}
		if validatorIndex > result.MaxIndex {
			result.MaxIndex = validatorIndex
		}
	}

	// Check if all expected pubkeys were found
	for pubkey := range expectedPubkeyMap {
		if _, found := result.FilesByPubkey[pubkey]; !found {
			return result, fmt.Errorf("expected pubkey not found: %s", pubkey)
		}
	}

	log.WithFields(logrus.Fields{
		"total_files":    len(files),
		"unique_pubkeys": len(result.FilesByPubkey),
		"min_index":      result.MinIndex,
		"max_index":      result.MaxIndex,
	}).Info("Pre-validation completed")

	return result, nil
}

// ValidateCount validates the number and sequence of exits
func (e *VoluntaryExits) ValidateCount(numExits int) error {
	if len(e.Metadata.FilesByPubkey) == 0 {
		return fmt.Errorf("no voluntary exits found")
	}

	for pubkey, fileInfos := range e.Metadata.FilesByPubkey {
		if len(fileInfos) == 0 {
			continue
		}

		// Sort file infos by validator index
		sort.Slice(fileInfos, func(i, j int) bool {
			return fileInfos[i].ValidatorIndex < fileInfos[j].ValidatorIndex
		})

		minIndex := fileInfos[0].ValidatorIndex
		maxIndex := fileInfos[len(fileInfos)-1].ValidatorIndex
		total := maxIndex - minIndex + 1

		if total != uint64(len(fileInfos)) {
			return fmt.Errorf("%d files found but expected %d for pubkey %s", len(fileInfos), total, pubkey)
		}

		if numExits > 0 && len(fileInfos) != numExits {
			return fmt.Errorf("expected %d exits for pubkey %s but found %d", numExits, pubkey, len(fileInfos))
		}
	}

	return nil
}

// ValidateCount validates the number and sequence of exits for a batch
func (b *VoluntaryExitsBatch) ValidateCount(numExits int) error {
	if len(b.ExitsByPubkey) == 0 {
		return fmt.Errorf("no voluntary exits found")
	}

	for pubkey, validatorExits := range b.ExitsByPubkey {
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

// ValidateIndices ensures the min and max validator indices match across all pubkeys
func (e *VoluntaryExits) ValidateIndices() error {
	if len(e.Metadata.FilesByPubkey) <= 1 {
		return nil // Nothing to compare with a single pubkey
	}

	var firstPubkey string
	var firstMin, firstMax uint64

	// Initialize with the first pubkey's values
	for pubkey, fileInfos := range e.Metadata.FilesByPubkey {
		if len(fileInfos) == 0 {
			return fmt.Errorf("no exits found for pubkey %s", pubkey)
		}

		// Sort file infos by validator index
		sort.Slice(fileInfos, func(i, j int) bool {
			return fileInfos[i].ValidatorIndex < fileInfos[j].ValidatorIndex
		})

		firstPubkey = pubkey
		firstMin = fileInfos[0].ValidatorIndex
		firstMax = fileInfos[len(fileInfos)-1].ValidatorIndex

		break
	}

	// Compare with all other pubkeys
	for pubkey, fileInfos := range e.Metadata.FilesByPubkey {
		if pubkey == firstPubkey {
			continue
		}

		if len(fileInfos) == 0 {
			return fmt.Errorf("no exits found for pubkey %s", pubkey)
		}

		// Sort file infos by validator index
		sort.Slice(fileInfos, func(i, j int) bool {
			return fileInfos[i].ValidatorIndex < fileInfos[j].ValidatorIndex
		})

		currentMin := fileInfos[0].ValidatorIndex
		currentMax := fileInfos[len(fileInfos)-1].ValidatorIndex

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

	// Parse validator index from filename
	filenameIndex, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		log.WithError(err).WithField("file", filePath).Error("Invalid validator index in filename")
		return nil, fmt.Errorf("invalid validator index in filename: %s", filePath)
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

	// Validate that filename index matches content index
	if filenameIndex != validatorIndex {
		log.WithFields(logrus.Fields{
			"file":           filePath,
			"filename_index": filenameIndex,
			"content_index":  validatorIndex,
		}).Error("Validator index mismatch between filename and file content")
		return nil, fmt.Errorf("validator index mismatch: filename has %d but content has %d for file %s", filenameIndex, validatorIndex, filePath)
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

// PubkeyBatch represents a batch of pubkeys to process
type PubkeyBatch struct {
	Pubkeys []string
	Files   map[string][]ExitFileInfo
}

// BatchResult contains the result of processing a batch
type BatchResult struct {
	VerifiedCount int
	MinIndex      uint64
	MaxIndex      uint64
}

// createBatches creates batches of pubkeys for processing
func (e *VoluntaryExits) createBatches() []PubkeyBatch {
	var batches []PubkeyBatch
	var currentBatch PubkeyBatch
	currentBatch.Files = make(map[string][]ExitFileInfo)

	pubkeyIndex := 0
	for pubkey, files := range e.Metadata.FilesByPubkey {
		currentBatch.Pubkeys = append(currentBatch.Pubkeys, pubkey)
		currentBatch.Files[pubkey] = files

		pubkeyIndex++
		if pubkeyIndex%e.BatchSize == 0 {
			batches = append(batches, currentBatch)
			currentBatch = PubkeyBatch{
				Files: make(map[string][]ExitFileInfo),
			}
		}
	}

	// Add remaining pubkeys
	if len(currentBatch.Pubkeys) > 0 {
		batches = append(batches, currentBatch)
	}

	return batches
}

// loadBatch loads exit data for a batch of pubkeys
func (e *VoluntaryExits) loadBatch(batch PubkeyBatch) (*VoluntaryExitsBatch, error) {
	exitsByPubkey := make(map[string]*ValidatorExits)

	for _, pubkey := range batch.Pubkeys {
		files := batch.Files[pubkey]
		validatorExits := &ValidatorExits{
			Exits: make([]*VoluntaryExit, 0, len(files)),
		}

		for _, fileInfo := range files {
			vexit, err := readExitFile(fileInfo.Path)
			if err != nil {
				log.WithError(err).WithField("file", fileInfo.Path).Error("Failed to read exit file")
				return nil, err
			}

			// Initialize state if needed
			if validatorExits.State == nil {
				slot := primitives.Slot(uint64(params.BeaconConfig().SlotsPerEpoch) * uint64(vexit.PBExit.Exit.Epoch))
				st, err := state_native.InitializeFromProtoDeneb(&ethpb.BeaconStateDeneb{
					Slot:                  slot,
					GenesisValidatorsRoot: params.BeaconConfig().GenesisValidatorsRoot[:],
				})
				if err != nil {
					return nil, err
				}

				// Pre-fill validators up to the first validator index
				for i := primitives.ValidatorIndex(0); i < vexit.PBExit.Exit.ValidatorIndex; i++ {
					if err := st.AppendValidator(&ethpb.Validator{}); err != nil {
						return nil, err
					}
				}

				validatorExits.State = st
			}

			validatorExits.Exits = append(validatorExits.Exits, vexit)
		}

		exitsByPubkey[pubkey] = validatorExits
	}

	return &VoluntaryExitsBatch{
		WithdrawalCreds: e.WithdrawalCreds,
		ExitsByPubkey:   exitsByPubkey,
	}, nil
}

// VoluntaryExitsBatch represents a batch of exits to be verified
type VoluntaryExitsBatch struct {
	WithdrawalCreds []byte
	ExitsByPubkey   map[string]*ValidatorExits
}

// verifyBatch verifies a single batch of exits using concurrent workers
func (e *VoluntaryExits) verifyBatch(batch *VoluntaryExitsBatch, pubkeyRanges map[string]IndexRange) (*BatchResult, error) {
	// Use the existing concurrent Verify method
	resp, err := batch.Verify()
	if err != nil {
		return nil, err
	}

	result := &BatchResult{
		MinIndex:      resp.FirstIndex,
		MaxIndex:      resp.LastIndex,
		VerifiedCount: 0,
	}

	// Track per-pubkey ranges and count verified exits
	for pubkey, validatorExits := range batch.ExitsByPubkey {
		if len(validatorExits.Exits) > 0 {
			minIdx := uint64(validatorExits.Exits[0].PBExit.Exit.ValidatorIndex)
			maxIdx := uint64(validatorExits.Exits[len(validatorExits.Exits)-1].PBExit.Exit.ValidatorIndex)

			pubkeyRanges[pubkey] = IndexRange{
				Min: minIdx,
				Max: maxIdx,
			}

			result.VerifiedCount += len(validatorExits.Exits)
		}
	}

	return result, nil
}

// validateIndicesFromRanges validates that all pubkeys have the same min/max indices
func (e *VoluntaryExits) validateIndicesFromRanges(pubkeyRanges map[string]IndexRange) error {
	if len(pubkeyRanges) <= 1 {
		return nil
	}

	var firstPubkey string
	var firstRange IndexRange

	// Get first pubkey range
	for pubkey, r := range pubkeyRanges {
		firstPubkey = pubkey
		firstRange = r
		break
	}

	// Compare with all other pubkeys
	for pubkey, r := range pubkeyRanges {
		if pubkey == firstPubkey {
			continue
		}

		if r.Min != firstRange.Min {
			return fmt.Errorf("minimum validator index mismatch: %d for pubkey %s vs %d for pubkey %s",
				r.Min, pubkey, firstRange.Min, firstPubkey)
		}

		if r.Max != firstRange.Max {
			return fmt.Errorf("maximum validator index mismatch: %d for pubkey %s vs %d for pubkey %s",
				r.Max, pubkey, firstRange.Max, firstPubkey)
		}
	}

	return nil
}

// Verify performs streaming verification with optimized memory usage
func (e *VoluntaryExits) Verify() (*VerifyResponse, error) {
	batches := e.createBatches()
	numBatches := len(batches)

	log.WithFields(logrus.Fields{
		"batch_size":    e.BatchSize,
		"num_batches":   numBatches,
		"total_pubkeys": len(e.Metadata.FilesByPubkey),
	}).Info("Starting streaming verification")

	// Global tracking
	var globalFirstIndex, globalLastIndex uint64
	globalFirstIndex = ^uint64(0) // Max uint64
	totalVerified := 0
	pubkeyRanges := make(map[string]IndexRange)

	// Process each batch
	for batchIndex, batch := range batches {
		log.WithFields(logrus.Fields{
			"batch":   batchIndex + 1,
			"total":   numBatches,
			"pubkeys": len(batch.Pubkeys),
		}).Info("Processing batch")

		// Load batch data
		batchExits, err := e.loadBatch(batch)
		if err != nil {
			return nil, fmt.Errorf("failed to load batch %d: %w", batchIndex, err)
		}

		// Perform validations on the batch
		if err := batchExits.ValidateCount(0); err != nil {
			return nil, fmt.Errorf("batch %d validation failed: %w", batchIndex, err)
		}

		// Verify the batch
		batchResult, err := e.verifyBatch(batchExits, pubkeyRanges)
		if err != nil {
			return nil, fmt.Errorf("batch %d verification failed: %w", batchIndex, err)
		}

		// Update global tracking
		totalVerified += batchResult.VerifiedCount
		if batchResult.MinIndex < globalFirstIndex {
			globalFirstIndex = batchResult.MinIndex
		}
		if batchResult.MaxIndex > globalLastIndex {
			globalLastIndex = batchResult.MaxIndex
		}

		log.WithFields(logrus.Fields{
			"batch":          batchIndex + 1,
			"verified_count": batchResult.VerifiedCount,
		}).Info("Batch processed successfully")

		// Explicitly free memory
		batchExits = nil
		runtime.GC()
	}

	// Perform deferred validations using collected metadata
	if err := e.validateIndicesFromRanges(pubkeyRanges); err != nil {
		return nil, err
	}

	log.WithFields(logrus.Fields{
		"total_verified": totalVerified,
		"first_index":    globalFirstIndex,
		"last_index":     globalLastIndex,
	}).Info("Streaming verification completed successfully")

	return &VerifyResponse{
		FirstIndex: globalFirstIndex,
		LastIndex:  globalLastIndex,
	}, nil
}

// Verify verifies all voluntary exits in a batch using concurrent workers
func (b *VoluntaryExitsBatch) Verify() (*VerifyResponse, error) {
	// Determine number of workers based on CPU count
	numWorkers := runtime.NumCPU()
	if numWorkers > len(b.ExitsByPubkey) {
		numWorkers = len(b.ExitsByPubkey)
	}

	log.WithFields(logrus.Fields{
		"workers": numWorkers,
		"pubkeys": len(b.ExitsByPubkey),
	}).Info("Starting verification with concurrent workers")

	// Initialize shared state
	sharedState := &sharedVerifyState{}

	// Create work channel
	workCh := make(chan verifyWorkerInput, len(b.ExitsByPubkey))

	// Queue all work
	for pubkey, validatorExits := range b.ExitsByPubkey {
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
	resultsCh := make(chan verifyWorkerResult, len(b.ExitsByPubkey))

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

				verifiedCount, err := b.verifyPubkeyExits(ctx, work.pubkey, work.validatorExits, sharedState)

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

// verifyPubkeyExits verifies all exits for a single pubkey
func (b *VoluntaryExitsBatch) verifyPubkeyExits(
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
			WithdrawalCredentials: b.WithdrawalCreds,
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

// SetBatchSize sets the batch size for streaming verification
func (e *VoluntaryExits) SetBatchSize(size int) {
	if size > 0 {
		e.BatchSize = size
	}
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

	// For each pubkey in our metadata, find the matching validator and copy files
	for pubkey, fileInfos := range e.Metadata.FilesByPubkey {
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
		for _, fileInfo := range fileInfos {
			expectedIndex := fmt.Sprintf("%d", fileInfo.ValidatorIndex)

			// Verify the validator index matches what we expect
			if validatorInfo.index != expectedIndex {
				continue
			}

			// Find the source file
			sourceFileName := fmt.Sprintf("%s-%s.json", expectedIndex, pubkey)

			// Copy file to output directory
			destFilePath := filepath.Join(outputDir, sourceFileName)

			if err := copyFile(fileInfo.Path, destFilePath); err != nil {
				log.WithError(err).WithFields(logrus.Fields{
					"source": fileInfo.Path,
					"dest":   destFilePath,
				}).Error("Failed to copy file")

				return err
			}
		}

		processedValidators[pubkey] = true
	}

	// Verify all expected validators were processed
	for pubkey := range e.Metadata.FilesByPubkey {
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
