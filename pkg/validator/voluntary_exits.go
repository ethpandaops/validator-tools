package validator

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/prysmaticlabs/prysm/v5/beacon-chain/core/blocks"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/state"
	state_native "github.com/prysmaticlabs/prysm/v5/beacon-chain/state/state-native"
	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/consensus-types/primitives"
	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"github.com/sirupsen/logrus"
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
}

// SignedVoluntaryExit represents the JSON structure of a signed voluntary exit
type SignedVoluntaryExit struct {
	Message struct {
		Epoch          string `json:"epoch"`
		ValidatorIndex string `json:"validator_index"`
	} `json:"message"`
	Signature string `json:"signature"`
}

// NewVoluntaryExits creates a new VoluntaryExits instance
func NewVoluntaryExits(path, network, withdrawalCreds string, numExits int, expectedPubkeys []string) (*VoluntaryExits, error) {
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

	if vErr := validateExits(exitsByPubkey, numExits); vErr != nil {
		log.WithError(vErr).Error("Failed to validate exits")

		return nil, vErr
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

// validateExits validates the number and sequence of exits
func validateExits(exitsByPubkey map[string]*ValidatorExits, numExits int) error {
	if len(exitsByPubkey) == 0 {
		return fmt.Errorf("no voluntary exits found")
	}

	for pubkey, validatorExits := range exitsByPubkey {
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
	}, nil
}

// Verify verifies all voluntary exits
func (e *VoluntaryExits) Verify() error {
	for pubkey, validatorExits := range e.ExitsByPubkey {
		log := log.WithField("pubkey", pubkey)
		verifiedCount := 0

		for _, exit := range validatorExits.Exits {
			if err := validatorExits.State.AppendValidator(&ethpb.Validator{
				PublicKey:             exit.Pubkey,
				WithdrawalCredentials: e.WithdrawalCreds,
				ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
			}); err != nil {
				log.WithError(err).WithField("validator_index", exit.PBExit.Exit.ValidatorIndex).Error("Failed to append validator")

				return err
			}

			validator, err := validatorExits.State.ValidatorAtIndexReadOnly(exit.PBExit.Exit.ValidatorIndex)
			if err != nil {
				log.WithError(err).WithField("validator_index", exit.PBExit.Exit.ValidatorIndex).Error("Failed to get validator")

				return err
			}

			if err := blocks.VerifyExitAndSignature(validator, validatorExits.State, exit.PBExit); err != nil {
				log.WithError(err).WithField("validator_index", exit.PBExit.Exit.ValidatorIndex).Error("Failed to verify exit and signature")

				return err
			}

			verifiedCount++

			log.WithField("validator_index", exit.PBExit.Exit.ValidatorIndex).Debug("Exit verified")
		}

		log.WithFields(logrus.Fields{
			"verified": verifiedCount,
			"total":    len(validatorExits.Exits),
		}).Info("Exits verified")
	}

	return nil
}
