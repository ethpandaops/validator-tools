package validator

import (
	"encoding/hex"
	"encoding/json"
	"os"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/core/signing"
	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/contracts/deposit"
	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
)

// Data represents deposit data with expected values for validation.
type Data struct {
	DepositData  []*ParsedData
	ExpectedData *ExpectedData
}

// ExpectedData contains the expected values for deposit validation.
type ExpectedData struct {
	Network        string
	Amount         uint64
	WithdrawalCred string
	Count          int
}

// ParsedData contains parsed deposit data in different formats.
type ParsedData struct {
	Deposit *Deposit
	PBData  *ethpb.Deposit_Data
}

// Deposit represents a validator deposit with all associated metadata.
type Deposit struct {
	PubKey                string `json:"pubkey"`
	WithdrawalCredentials string `json:"withdrawal_credentials"`
	Amount                uint64 `json:"amount"`
	Signature             string `json:"signature"`
	DepositMessageRoot    string `json:"deposit_message_root"`
	DepositDataRoot       string `json:"deposit_data_root"`
	NetworkName           string `json:"network_name"`
	DepositCliVersion     string `json:"deposit_cli_version"`
	ForkVersion           string `json:"fork_version"`
}

// NewData creates a new Data instance by reading and parsing deposit data from a file.
func NewData(path, expectedNetwork, expectedWithdrawalCred string, expectedAmount uint64, expectedCount int) (*Data, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- Path is validated by caller
	if err != nil {
		return nil, errors.Wrap(err, "failed to read deposit data file")
	}

	var deposits []*Deposit
	if err := json.Unmarshal(data, &deposits); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal deposit data")
	}

	depositData := make([]*ParsedData, len(deposits))

	for i, d := range deposits {
		pubkey, err := hex.DecodeString(d.PubKey)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode pubkey")
		}

		withdrawalCreds, err := hex.DecodeString(d.WithdrawalCredentials)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode withdrawal credentials")
		}

		signature, err := hex.DecodeString(d.Signature)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode signature")
		}

		pbData := ethpb.Deposit_Data{
			PublicKey:             pubkey,
			WithdrawalCredentials: withdrawalCreds,
			Amount:                d.Amount,
			Signature:             signature,
		}

		depositData[i] = &ParsedData{
			Deposit: d,
			PBData:  &pbData,
		}
	}

	log.Printf("Deposit data: %v", depositData)

	return &Data{
		DepositData: depositData,
		ExpectedData: &ExpectedData{
			Network:        expectedNetwork,
			Amount:         expectedAmount,
			WithdrawalCred: expectedWithdrawalCred,
			Count:          expectedCount,
		},
	}, nil
}

// Validate checks if the deposit data matches the expected values.
func (d *Data) Validate() error {
	if d.ExpectedData.Count > 0 && len(d.DepositData) != d.ExpectedData.Count {
		return errors.Errorf("count mismatch: expected %d, got %d", d.ExpectedData.Count, len(d.DepositData))
	}

	for _, set := range d.DepositData {
		if err := set.Deposit.Validate(d.ExpectedData); err != nil {
			return errors.Wrapf(err, "invalid deposit for pubkey %s", set.Deposit.PubKey)
		}
	}

	return nil
}

// Validate checks if the deposit matches the expected data.
func (d *Deposit) Validate(expectedData *ExpectedData) error {
	if expectedData.Network != "" && d.NetworkName != expectedData.Network {
		return errors.Errorf("network mismatch: expected %s, got %s", expectedData.Network, d.NetworkName)
	}

	if d.Amount != expectedData.Amount {
		return errors.Errorf("amount mismatch: expected %d, got %d", expectedData.Amount, d.Amount)
	}

	if expectedData.WithdrawalCred != "" && d.WithdrawalCredentials != expectedData.WithdrawalCred {
		return errors.Errorf("withdrawal credentials mismatch: expected %s, got %s", expectedData.WithdrawalCred, d.WithdrawalCredentials)
	}

	return nil
}

// Verify validates the cryptographic signatures of all deposits.
func (d *Data) Verify() error {
	for _, set := range d.DepositData {
		forkVersion, err := hex.DecodeString(set.Deposit.ForkVersion)
		if err != nil {
			return errors.Wrap(err, "failed to decode fork version")
		}

		ok, err := IsValidDepositSignature(set.PBData, forkVersion)
		if err != nil {
			return errors.Wrapf(err, "invalid deposit for pubkey %s", set.Deposit.PubKey)
		}

		if !ok {
			return errors.Wrapf(err, "invalid deposit signature for pubkey %s", set.Deposit.PubKey)
		}
	}

	return nil
}

// IsValidDepositSignature verifies if a deposit signature is valid for the given fork version.
func IsValidDepositSignature(data *ethpb.Deposit_Data, forkVersion []byte) (bool, error) {
	domain, err := signing.ComputeDomain(params.BeaconConfig().DomainDeposit, forkVersion, nil)
	if err != nil {
		return false, err
	}

	if err := deposit.VerifyDepositSignature(data, domain); err != nil {
		return false, err
	}

	return true, nil
}
