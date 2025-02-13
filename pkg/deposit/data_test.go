package deposit_test

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethpandaops/validator-tools/pkg/deposit"
	ethpb "github.com/prysmaticlabs/prysm/v5/proto/prysm/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestNewDepositData(t *testing.T) {
	holeskyDeposits := []*deposit.Deposit{
		{
			PubKey:                "a63bffb2b9be4830811150bdaefd904d32aad8a09998aa9bc836cda7cbab97c594293b995199afe659560ee7a930149d",
			WithdrawalCredentials: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
			Amount:                32000000000,
			Signature:             "93f06f7867b898a807d9425c2e647bebcf23bc16adca708ae54958949aced24acd4ccb813b04bb9a431fd6df437f689614246109b008eadafc444f2bd5f0323005b77f4c2810f9e3630a91ffc5859bd2a92ee84aeb97b352e5e2f1f90cf84aca",
			DepositMessageRoot:    "7fe71257fdc44d492ba5c46ee4a3c8692fecfdfb3377abacd93e3b540fe7acbb",
			DepositDataRoot:       "422b4b4ec62d367ce42e0ae98b6b8a1351a480cce3b2e31ac6c3fe205827db3f",
			ForkVersion:           "01017000",
			NetworkName:           "holesky",
			DepositCliVersion:     "10.6.0",
		},
		{
			PubKey:                "83e8519e3c69669c1141ef7a5e66c710c67ab52cc6f57c4ee35200c35154daa3cdc18bc52a47ef6c5900df29aedcf302",
			WithdrawalCredentials: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
			Amount:                32000000000,
			Signature:             "aedf8d96a3a88d50249925b765ff8ea29ae621e01f2da05f733e08aeea8644204ae6e90756c160f4c172140e67cf57640201401961b4d8d76276400b07ad242e4feb185db90ffe4a84589b8ff57874b27117dd988b25684cf86ae185e9ecf5a7",
			DepositMessageRoot:    "ecf288cca52578464f4f94caa42650d8d8a6b6ad7ef99c4851f4da7ef807ef7b",
			DepositDataRoot:       "c70451a6a86e3623d2cbc0ffd2586f8d0033ab8cc198286def319e2a42ea5664",
			ForkVersion:           "01017000",
			NetworkName:           "holesky",
			DepositCliVersion:     "10.6.0",
		},
		{
			PubKey:                "ae17df015acee11b422707e25ba7ca3900a425a1107660a4409d2dca35e1535ce5b871c61b240c331bdb18fcf811944b",
			WithdrawalCredentials: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
			Amount:                32000000000,
			Signature:             "ab07ecd8008365562ecdf7dc698cf9e24957c7442364f52b3dbb96020006e7edbda75ed2fb58fa9085c240102a2c526009e3c7a9740221c624292f4b7c06a58cbcc8b59f6bcd2cf37072b6773dac3ff3ac1c97be0e94f4cf8340dd5040c19b44",
			DepositMessageRoot:    "f9f18be072a4eb609e78943f33f6d67b279c2868831cb71dcaee2acd7ecf38d7",
			DepositDataRoot:       "4bc1c4179e77fe4acea3c3b8defda5bf735dd9e669f0d6ebaaf1ec2f8e27a079",
			ForkVersion:           "01017000",
			NetworkName:           "holesky",
			DepositCliVersion:     "10.6.0",
		},
	}

	mainnetDeposits := []*deposit.Deposit{
		{
			PubKey:                "a63bffb2b9be4830811150bdaefd904d32aad8a09998aa9bc836cda7cbab97c594293b995199afe659560ee7a930149d",
			WithdrawalCredentials: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
			Amount:                32000000000,
			Signature:             "913b0070abb6f772c4c4533a921865617fe949cf34caef892e5ce7d516ea1be3e5e401a26744dab0310bdf59f77a1f12186ac3d1aae342a00a6f0090b94c98f69481e1a7f3cb45a1cb794607aa8f65b752cd40e97c0767be3585308f1b9e5ef2",
			DepositMessageRoot:    "7fe71257fdc44d492ba5c46ee4a3c8692fecfdfb3377abacd93e3b540fe7acbb",
			DepositDataRoot:       "a3b7db7d5bb99bc7591e42d9d24a69ba3591a292faac4cddfecf4af780235a0e",
			ForkVersion:           "00000000",
			NetworkName:           "mainnet",
			DepositCliVersion:     "2.8.0",
		},
		{
			PubKey:                "83e8519e3c69669c1141ef7a5e66c710c67ab52cc6f57c4ee35200c35154daa3cdc18bc52a47ef6c5900df29aedcf302",
			WithdrawalCredentials: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
			Amount:                32000000000,
			Signature:             "944fa6045316effa7b0b7a885b68e8baf9d8e28f6713637b9cba9afe6707bb55e2e78fa0f839fd5dea2e61d6555e8f5114b19ee5df8a4daffe0ca8aac4a03a8808efd942bf92a99fc8899b890b32d9735818eee42880537419987fd1106ead6c",
			DepositMessageRoot:    "ecf288cca52578464f4f94caa42650d8d8a6b6ad7ef99c4851f4da7ef807ef7b",
			DepositDataRoot:       "4b136ca1c27effbded3d0ac43341c32299c74af465f4b05e2d1807f3fdc7adae",
			ForkVersion:           "00000000",
			NetworkName:           "mainnet",
			DepositCliVersion:     "2.8.0",
		},
		{
			PubKey:                "ae17df015acee11b422707e25ba7ca3900a425a1107660a4409d2dca35e1535ce5b871c61b240c331bdb18fcf811944b",
			WithdrawalCredentials: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
			Amount:                32000000000,
			Signature:             "90551cc9e6898338225aa94908539486f08365f8845ffd1bdb06e3fd9fcd6b5662c491de56e074064a8ecd2f6dd35bae079882e159673ff4978e3037751fa106f30d639503fea3fead27dba30cd5bef9d65208f81a9c832f33fdeffec14b9c40",
			DepositMessageRoot:    "f9f18be072a4eb609e78943f33f6d67b279c2868831cb71dcaee2acd7ecf38d7",
			DepositDataRoot:       "1f9e2334ba20dc84466d65338ad68bd598d00b6ad4c8d27961d2855f5e5d3955",
			ForkVersion:           "00000000",
			NetworkName:           "mainnet",
			DepositCliVersion:     "2.8.0",
		},
	}

	// Create temporary files
	tmpDir := t.TempDir()
	holeskyFile := filepath.Join(tmpDir, "holesky_deposit_data.json")
	mainnetFile := filepath.Join(tmpDir, "mainnet_deposit_data.json")

	holeskyData, err := json.Marshal(holeskyDeposits)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(holeskyFile, holeskyData, 0o600))

	mainnetData, err := json.Marshal(mainnetDeposits)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(mainnetFile, mainnetData, 0o600))

	tests := []struct {
		name                   string
		path                   string
		expectedNetwork        string
		expectedWithdrawalCred string
		expectedAmount         uint64
		expectedCount          int
		wantErr                bool
	}{
		{
			name:                   "valid holesky deposit data",
			path:                   holeskyFile,
			expectedNetwork:        "holesky",
			expectedWithdrawalCred: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
			expectedAmount:         32000000000,
			expectedCount:          3,
			wantErr:                false,
		},
		{
			name:                   "valid mainnet deposit data",
			path:                   mainnetFile,
			expectedNetwork:        "mainnet",
			expectedWithdrawalCred: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
			expectedAmount:         32000000000,
			expectedCount:          3,
			wantErr:                false,
		},
		{
			name:                   "invalid file path",
			path:                   "nonexistent.json",
			expectedNetwork:        "mainnet",
			expectedWithdrawalCred: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
			expectedAmount:         32000000000,
			expectedCount:          3,
			wantErr:                true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := deposit.NewDepositData(
				tt.path,
				tt.expectedNetwork,
				tt.expectedWithdrawalCred,
				tt.expectedAmount,
				tt.expectedCount,
			)

			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, tt.expectedCount, len(got.DepositData))
			require.Equal(t, tt.expectedNetwork, got.ExpectedData.Network)
			require.Equal(t, tt.expectedAmount, got.ExpectedData.Amount)
			require.Equal(t, tt.expectedWithdrawalCred, got.ExpectedData.WithdrawalCred)
		})
	}
}

func TestData_Validate(t *testing.T) {
	tests := []struct {
		name    string
		data    *deposit.Data
		wantErr bool
	}{
		{
			name: "valid data",
			data: &deposit.Data{
				DepositData: []*deposit.ParsedData{
					{
						Deposit: &deposit.Deposit{
							NetworkName:           "holesky",
							Amount:                32000000000,
							WithdrawalCredentials: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
						},
					},
				},
				ExpectedData: &deposit.ExpectedData{
					Network:        "holesky",
					Amount:         32000000000,
					WithdrawalCred: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
					Count:          1,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid count",
			data: &deposit.Data{
				DepositData: []*deposit.ParsedData{
					{
						Deposit: &deposit.Deposit{
							NetworkName:           "holesky",
							Amount:                32000000000,
							WithdrawalCredentials: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
						},
					},
				},
				ExpectedData: &deposit.ExpectedData{
					Network:        "holesky",
					Amount:         32000000000,
					WithdrawalCred: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
					Count:          2,
				},
			},
			wantErr: true,
		},
		{
			name: "network mismatch",
			data: &deposit.Data{
				DepositData: []*deposit.ParsedData{
					{
						Deposit: &deposit.Deposit{
							NetworkName:           "mainnet",
							Amount:                32000000000,
							WithdrawalCredentials: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
						},
					},
				},
				ExpectedData: &deposit.ExpectedData{
					Network:        "holesky",
					Amount:         32000000000,
					WithdrawalCred: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
					Count:          1,
				},
			},
			wantErr: true,
		},
		{
			name: "amount mismatch",
			data: &deposit.Data{
				DepositData: []*deposit.ParsedData{
					{
						Deposit: &deposit.Deposit{
							NetworkName:           "holesky",
							Amount:                16000000000, // Half the expected amount
							WithdrawalCredentials: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
						},
					},
				},
				ExpectedData: &deposit.ExpectedData{
					Network:        "holesky",
					Amount:         32000000000,
					WithdrawalCred: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
					Count:          1,
				},
			},
			wantErr: true,
		},
		{
			name: "withdrawal credentials mismatch",
			data: &deposit.Data{
				DepositData: []*deposit.ParsedData{
					{
						Deposit: &deposit.Deposit{
							NetworkName:           "holesky",
							Amount:                32000000000,
							WithdrawalCredentials: "0100000000000000000000005124cd4a34790c0da4cbcdd89f536b9508b8bc42", // Different address
						},
					},
				},
				ExpectedData: &deposit.ExpectedData{
					Network:        "holesky",
					Amount:         32000000000,
					WithdrawalCred: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
					Count:          1,
				},
			},
			wantErr: true,
		},
		{
			name: "multiple deposits with mixed validity",
			data: &deposit.Data{
				DepositData: []*deposit.ParsedData{
					{
						Deposit: &deposit.Deposit{
							NetworkName:           "holesky",
							Amount:                32000000000,
							WithdrawalCredentials: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
						},
					},
					{
						Deposit: &deposit.Deposit{
							NetworkName:           "mainnet", // Wrong network
							Amount:                32000000000,
							WithdrawalCredentials: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
						},
					},
				},
				ExpectedData: &deposit.ExpectedData{
					Network:        "holesky",
					Amount:         32000000000,
					WithdrawalCred: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
					Count:          2,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.data.Validate()
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestIsValidDepositSignature(t *testing.T) {
	tests := []struct {
		name        string
		pubkey      string
		signature   string
		forkVersion string
		network     string
		amount      uint64
		wantErr     bool
	}{
		{
			name:        "valid holesky signature",
			pubkey:      "a63bffb2b9be4830811150bdaefd904d32aad8a09998aa9bc836cda7cbab97c594293b995199afe659560ee7a930149d",
			signature:   "93f06f7867b898a807d9425c2e647bebcf23bc16adca708ae54958949aced24acd4ccb813b04bb9a431fd6df437f689614246109b008eadafc444f2bd5f0323005b77f4c2810f9e3630a91ffc5859bd2a92ee84aeb97b352e5e2f1f90cf84aca",
			forkVersion: "01017000",
			network:     "holesky",
			amount:      32000000000,
			wantErr:     false,
		},
		{
			name:        "valid mainnet signature",
			pubkey:      "a63bffb2b9be4830811150bdaefd904d32aad8a09998aa9bc836cda7cbab97c594293b995199afe659560ee7a930149d",
			signature:   "913b0070abb6f772c4c4533a921865617fe949cf34caef892e5ce7d516ea1be3e5e401a26744dab0310bdf59f77a1f12186ac3d1aae342a00a6f0090b94c98f69481e1a7f3cb45a1cb794607aa8f65b752cd40e97c0767be3585308f1b9e5ef2",
			forkVersion: "00000000",
			network:     "mainnet",
			amount:      32000000000,
			wantErr:     false,
		},
		{
			name:        "invalid pubkey length",
			pubkey:      "a63bffb2", // Too short
			signature:   "93f06f7867b898a807d9425c2e647bebcf23bc16adca708ae54958949aced24acd4ccb813b04bb9a431fd6df437f689614246109b008eadafc444f2bd5f0323005b77f4c2810f9e3630a91ffc5859bd2a92ee84aeb97b352e5e2f1f90cf84aca",
			forkVersion: "01017000",
			network:     "holesky",
			amount:      32000000000,
			wantErr:     true,
		},
		{
			name:        "invalid signature length",
			pubkey:      "a63bffb2b9be4830811150bdaefd904d32aad8a09998aa9bc836cda7cbab97c594293b995199afe659560ee7a930149d",
			signature:   "93f06f78", // Too short
			forkVersion: "01017000",
			network:     "holesky",
			amount:      32000000000,
			wantErr:     true,
		},
		{
			name:        "wrong fork version for network",
			pubkey:      "a63bffb2b9be4830811150bdaefd904d32aad8a09998aa9bc836cda7cbab97c594293b995199afe659560ee7a930149d",
			signature:   "93f06f7867b898a807d9425c2e647bebcf23bc16adca708ae54958949aced24acd4ccb813b04bb9a431fd6df437f689614246109b008eadafc444f2bd5f0323005b77f4c2810f9e3630a91ffc5859bd2a92ee84aeb97b352e5e2f1f90cf84aca",
			forkVersion: "00000000", // Mainnet fork version with holesky signature
			network:     "holesky",
			amount:      32000000000,
			wantErr:     true,
		},
		{
			name:        "invalid amount",
			pubkey:      "a63bffb2b9be4830811150bdaefd904d32aad8a09998aa9bc836cda7cbab97c594293b995199afe659560ee7a930149d",
			signature:   "93f06f7867b898a807d9425c2e647bebcf23bc16adca708ae54958949aced24acd4ccb813b04bb9a431fd6df437f689614246109b008eadafc444f2bd5f0323005b77f4c2810f9e3630a91ffc5859bd2a92ee84aeb97b352e5e2f1f90cf84aca",
			forkVersion: "01017000",
			network:     "holesky",
			amount:      16000000000, // Wrong amount
			wantErr:     true,
		},
		{
			name:        "mismatched signature for pubkey",
			pubkey:      "83e8519e3c69669c1141ef7a5e66c710c67ab52cc6f57c4ee35200c35154daa3cdc18bc52a47ef6c5900df29aedcf302", // Different pubkey
			signature:   "93f06f7867b898a807d9425c2e647bebcf23bc16adca708ae54958949aced24acd4ccb813b04bb9a431fd6df437f689614246109b008eadafc444f2bd5f0323005b77f4c2810f9e3630a91ffc5859bd2a92ee84aeb97b352e5e2f1f90cf84aca",
			forkVersion: "01017000",
			network:     "holesky",
			amount:      32000000000,
			wantErr:     true,
		},
	}

	withdrawalCreds, err := hex.DecodeString("0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41")
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forkVersion, err := hex.DecodeString(tt.forkVersion)
			if err != nil && tt.wantErr {
				return
			}

			require.NoError(t, err)

			pubkey, err := hex.DecodeString(tt.pubkey)
			if err != nil && tt.wantErr {
				return
			}

			require.NoError(t, err)

			signature, err := hex.DecodeString(tt.signature)
			if err != nil && tt.wantErr {
				return
			}

			require.NoError(t, err)

			data := &ethpb.Deposit_Data{
				PublicKey:             pubkey,
				WithdrawalCredentials: withdrawalCreds,
				Amount:                tt.amount,
				Signature:             signature,
			}

			ok, err := deposit.IsValidDepositSignature(data, forkVersion)
			if tt.wantErr {
				require.False(t, ok)
				require.Error(t, err)

				return
			}

			require.NoError(t, err)

			require.True(t, ok)
		})
	}
}

func TestData_Verify(t *testing.T) {
	validHoleskyDeposit := &deposit.ParsedData{
		Deposit: &deposit.Deposit{
			PubKey:                "a63bffb2b9be4830811150bdaefd904d32aad8a09998aa9bc836cda7cbab97c594293b995199afe659560ee7a930149d",
			WithdrawalCredentials: "0100000000000000000000004124cd4a34790c0da4cbcdd89f536b9508b8bc41",
			Amount:                32000000000,
			Signature:             "93f06f7867b898a807d9425c2e647bebcf23bc16adca708ae54958949aced24acd4ccb813b04bb9a431fd6df437f689614246109b008eadafc444f2bd5f0323005b77f4c2810f9e3630a91ffc5859bd2a92ee84aeb97b352e5e2f1f90cf84aca",
			ForkVersion:           "01017000",
			NetworkName:           "holesky",
		},
	}

	pubkey, err := hex.DecodeString(validHoleskyDeposit.Deposit.PubKey)
	require.NoError(t, err)
	withdrawalCreds, err := hex.DecodeString(validHoleskyDeposit.Deposit.WithdrawalCredentials)
	require.NoError(t, err)
	signature, err := hex.DecodeString(validHoleskyDeposit.Deposit.Signature)
	require.NoError(t, err)

	validHoleskyDeposit.PBData = &ethpb.Deposit_Data{
		PublicKey:             pubkey,
		WithdrawalCredentials: withdrawalCreds,
		Amount:                validHoleskyDeposit.Deposit.Amount,
		Signature:             signature,
	}

	invalidSignatureDeposit := &deposit.ParsedData{
		Deposit: &deposit.Deposit{
			PubKey:                validHoleskyDeposit.Deposit.PubKey,
			WithdrawalCredentials: validHoleskyDeposit.Deposit.WithdrawalCredentials,
			Amount:                validHoleskyDeposit.Deposit.Amount,
			Signature:             "invalid",
			ForkVersion:           validHoleskyDeposit.Deposit.ForkVersion,
			NetworkName:           "holesky",
		},
	}

	invalidPubkey, err := hex.DecodeString(validHoleskyDeposit.Deposit.PubKey)
	require.NoError(t, err)
	invalidWithdrawalCreds, err := hex.DecodeString(validHoleskyDeposit.Deposit.WithdrawalCredentials)
	require.NoError(t, err)

	invalidSignature := []byte("invalid")

	invalidSignatureDeposit.PBData = &ethpb.Deposit_Data{
		PublicKey:             invalidPubkey,
		WithdrawalCredentials: invalidWithdrawalCreds,
		Amount:                invalidSignatureDeposit.Deposit.Amount,
		Signature:             invalidSignature,
	}

	tests := []struct {
		name    string
		data    *deposit.Data
		wantErr bool
	}{
		{
			name: "valid deposit",
			data: &deposit.Data{
				DepositData: []*deposit.ParsedData{validHoleskyDeposit},
			},
			wantErr: false,
		},
		{
			name: "invalid fork version",
			data: &deposit.Data{
				DepositData: []*deposit.ParsedData{
					{
						Deposit: &deposit.Deposit{
							PubKey:                validHoleskyDeposit.Deposit.PubKey,
							WithdrawalCredentials: validHoleskyDeposit.Deposit.WithdrawalCredentials,
							Amount:                validHoleskyDeposit.Deposit.Amount,
							Signature:             validHoleskyDeposit.Deposit.Signature,
							ForkVersion:           "invalid",
							NetworkName:           "holesky",
						},
						PBData: validHoleskyDeposit.PBData,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple deposits with one invalid",
			data: &deposit.Data{
				DepositData: []*deposit.ParsedData{
					validHoleskyDeposit,
					invalidSignatureDeposit,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.data.Verify()
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
		})
	}
}
