package deposit

import (
	"encoding/hex"
	"encoding/json"
	"log"
	"os"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/v5/beacon-chain/core/signing"
	"github.com/prysmaticlabs/prysm/v5/config/params"
	"github.com/prysmaticlabs/prysm/v5/contracts/deposit"
	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
)

type Data struct {
	DepositData  []*ParsedData
	ExpectedData *ExpectedData
}

type ExpectedData struct {
	Network        string
	Amount         uint64
	WithdrawalCred string
	Count          int
}

type ParsedData struct {
	Deposit *Deposit
	PBData  *ethpb.Deposit_Data
}

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

func NewDepositData(path, expectedNetwork, expectedWithdrawalCred string, expectedAmount uint64, expectedCount int) (*Data, error) {
	data, err := os.ReadFile(path)
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
