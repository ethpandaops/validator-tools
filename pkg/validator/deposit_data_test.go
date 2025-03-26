package validator

import (
	"encoding/hex"
	"testing"

	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeposit_Validate(t *testing.T) {
	tests := []struct {
		name         string
		deposit      *Deposit
		expectedData *ExpectedData
		wantErr      bool
		errMsg       string
	}{
		{
			name: "valid deposit",
			deposit: &Deposit{
				NetworkName:           "mainnet",
				Amount:                32000000000, // 32 ETH in Gwei
				WithdrawalCredentials: "0x010000000000000000000000844d391c4074c548b7c968739e717a949358c721",
			},
			expectedData: &ExpectedData{
				Network:        "mainnet",
				Amount:         32000000000,
				WithdrawalCred: "0x010000000000000000000000844d391c4074c548b7c968739e717a949358c721",
			},
			wantErr: false,
		},
		{
			name: "network mismatch",
			deposit: &Deposit{
				NetworkName:           "mainnet",
				Amount:                32000000000,
				WithdrawalCredentials: "0x010000000000000000000000844d391c4074c548b7c968739e717a949358c721",
			},
			expectedData: &ExpectedData{
				Network:        "goerli",
				Amount:         32000000000,
				WithdrawalCred: "0x010000000000000000000000844d391c4074c548b7c968739e717a949358c721",
			},
			wantErr: true,
			errMsg:  "network mismatch: expected goerli, got mainnet",
		},
		{
			name: "amount mismatch",
			deposit: &Deposit{
				NetworkName:           "mainnet",
				Amount:                16000000000, // 16 ETH
				WithdrawalCredentials: "0x010000000000000000000000844d391c4074c548b7c968739e717a949358c721",
			},
			expectedData: &ExpectedData{
				Network:        "mainnet",
				Amount:         32000000000,
				WithdrawalCred: "0x010000000000000000000000844d391c4074c548b7c968739e717a949358c721",
			},
			wantErr: true,
			errMsg:  "amount mismatch: expected 32000000000, got 16000000000",
		},
		{
			name: "withdrawal credentials mismatch",
			deposit: &Deposit{
				NetworkName:           "mainnet",
				Amount:                32000000000,
				WithdrawalCredentials: "0x010000000000000000000000844d391c4074c548b7c968739e717a949358c722",
			},
			expectedData: &ExpectedData{
				Network:        "mainnet",
				Amount:         32000000000,
				WithdrawalCred: "0x010000000000000000000000844d391c4074c548b7c968739e717a949358c721",
			},
			wantErr: true,
			errMsg:  "withdrawal credentials mismatch: expected 0x010000000000000000000000844d391c4074c548b7c968739e717a949358c721, got 0x010000000000000000000000844d391c4074c548b7c968739e717a949358c722",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.deposit.Validate(tt.expectedData)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestData_Verify(t *testing.T) {
	// Sample test key and signature (you would need to generate these with proper BLS signatures)
	pubkey := make([]byte, 48)
	sig := make([]byte, 96)
	withdrawalCreds := make([]byte, 32)
	forkVersion := make([]byte, 4)

	depositData := &Data{
		DepositData: []*ParsedData{
			{
				Deposit: &Deposit{
					PubKey:      hex.EncodeToString(pubkey),
					Signature:   hex.EncodeToString(sig),
					ForkVersion: hex.EncodeToString(forkVersion),
				},
				PBData: &ethpb.Deposit_Data{
					PublicKey:             pubkey,
					WithdrawalCredentials: withdrawalCreds,
					Amount:                32000000000,
					Signature:             sig,
				},
			},
		},
	}

	err := depositData.Verify()
	// Note: This will fail without proper BLS signatures
	assert.Error(t, err, "expected error due to invalid signature")
}

func TestData_Validate_Count(t *testing.T) {
	data := &Data{
		DepositData: []*ParsedData{
			{Deposit: &Deposit{}},
			{Deposit: &Deposit{}},
		},
		ExpectedData: &ExpectedData{
			Count: 3,
		},
	}

	err := data.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count mismatch: expected 3, got 2")
}
