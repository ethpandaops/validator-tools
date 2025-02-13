package deposit

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
)

type Exits struct {
	WithdrawalCreds []byte
	ExitsByPubkey   map[string]*ValidatorExits
}

type ValidatorExits struct {
	State state.BeaconState
	Exits []*VoluntaryExit
}

type VoluntaryExit struct {
	PBExit *ethpb.SignedVoluntaryExit
	Pubkey []byte
}

type SignedVoluntaryExit struct {
	Message struct {
		Epoch          string `json:"epoch"`
		ValidatorIndex string `json:"validator_index"`
	} `json:"message"`
	Signature string `json:"signature"`
}

func NewVoluntaryExits(path, network, withdrawalCreds string, numExits int) (*Exits, error) {
	if err := setNetwork(network); err != nil {
		return nil, err
	}

	exitsByPubkey := make(map[string]*ValidatorExits)

	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !isExitFile(file) {
			continue
		}

		filePath := filepath.Join(path, file.Name())

		vexit, rErr := readExitFile(filePath)
		if rErr != nil {
			fmt.Printf("⚠️ skipping %s: %v\n", file.Name(), rErr)

			continue
		}

		pubkeyStr := hex.EncodeToString(vexit.Pubkey)

		if iErr := initializeExitState(exitsByPubkey, pubkeyStr, vexit); iErr != nil {
			return nil, iErr
		}

		exitsByPubkey[pubkeyStr].Exits = append(exitsByPubkey[pubkeyStr].Exits, vexit)
	}

	if vErr := validateExits(exitsByPubkey, numExits); vErr != nil {
		return nil, vErr
	}

	creds, err := hex.DecodeString(withdrawalCreds)
	if err != nil {
		return nil, err
	}

	return &Exits{
		WithdrawalCreds: creds,
		ExitsByPubkey:   exitsByPubkey,
	}, nil
}

func setNetwork(network string) error {
	switch network {
	case "mainnet":
		params.OverrideBeaconConfig(params.MainnetConfig())
	case "holesky":
		params.OverrideBeaconConfig(params.HoleskyConfig())
	default:
		return fmt.Errorf("unknown network: %s", network)
	}

	return nil
}

func isExitFile(file os.DirEntry) bool {
	return !file.IsDir() && strings.Contains(file.Name(), ".json")
}

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

func validateExits(exitsByPubkey map[string]*ValidatorExits, numExits int) error {
	if len(exitsByPubkey) == 0 {
		return fmt.Errorf("no voluntary exits found")
	}

	for pubkey, validatorExits := range exitsByPubkey {
		total := uint64(validatorExits.Exits[len(validatorExits.Exits)-1].PBExit.Exit.ValidatorIndex - validatorExits.Exits[0].PBExit.Exit.ValidatorIndex + 1)

		if total != uint64(len(validatorExits.Exits)) {
			return fmt.Errorf("%d files found but expected %d for pubkey %s", len(validatorExits.Exits), total, pubkey)
		}

		if len(validatorExits.Exits) != numExits {
			return fmt.Errorf("expected %d exits for pubkey %s but found %d", numExits, pubkey, len(validatorExits.Exits))
		}
	}

	return nil
}

func readExitFile(filePath string) (*VoluntaryExit, error) {
	parts := strings.Split(strings.TrimSuffix(filepath.Base(filePath), ".json"), "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid file name format: %s", filePath)
	}

	pubkey, err := hex.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid pubkey in filename: %s", filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var signedExit SignedVoluntaryExit

	if uErr := json.Unmarshal(data, &signedExit); uErr != nil {
		return nil, uErr
	}

	epoch, err := strconv.ParseUint(signedExit.Message.Epoch, 10, 64)
	if err != nil {
		return nil, err
	}

	validatorIndex, err := strconv.ParseUint(signedExit.Message.ValidatorIndex, 10, 64)
	if err != nil {
		return nil, err
	}

	signature, err := hex.DecodeString(strings.TrimPrefix(signedExit.Signature, "0x"))
	if err != nil {
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

func (e *Exits) Verify() error {
	for pubkey, validatorExits := range e.ExitsByPubkey {
		verifiedCount := 0

		for _, exit := range validatorExits.Exits {
			if err := validatorExits.State.AppendValidator(&ethpb.Validator{
				PublicKey:             exit.Pubkey,
				WithdrawalCredentials: e.WithdrawalCreds,
				ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
			}); err != nil {
				fmt.Printf("Failed to verify exit for pubkey %s: %v\n", pubkey, err)

				return err
			}

			validator, err := validatorExits.State.ValidatorAtIndexReadOnly(exit.PBExit.Exit.ValidatorIndex)
			if err != nil {
				fmt.Printf("Failed to verify exit for pubkey %s: %v\n", pubkey, err)

				return err
			}

			if err := blocks.VerifyExitAndSignature(validator, validatorExits.State, exit.PBExit); err != nil {
				fmt.Printf("Failed to verify exit for pubkey %s: %v\n", pubkey, err)

				return err
			}

			verifiedCount++
		}

		fmt.Printf("✅ %d/%d verified for %s\n", verifiedCount, len(validatorExits.Exits), pubkey)
	}

	return nil
}
