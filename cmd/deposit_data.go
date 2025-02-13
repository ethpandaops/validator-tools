package cmd

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/ethpandaops/validator-tools/pkg/deposit"
)

var (
	depositDataPath        string
	expectedNetwork        string
	expectedAmount         uint64
	expectedWithdrawalCred string
	expectedCount          int
)

var depositDataCmd = &cobra.Command{
	Use:   "data",
	Short: "Check deposit data",
	Long:  `Verifies and validates deposit data file format and contents.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		initCommon()

		err := verifyDepositData()
		if err != nil {
			log.Fatal(err)
		}

		return nil
	},
}

func init() {
	depositCmd.AddCommand(depositDataCmd)

	depositDataCmd.Flags().StringVar(&depositDataPath, "deposit-data", "", "Path to deposit data JSON file")
	depositDataCmd.Flags().StringVar(&expectedNetwork, "network", "", "Expected network (e.g. mainnet, goerli)")
	depositDataCmd.Flags().Uint64Var(&expectedAmount, "amount", 32000000000, "Expected deposit amount in Gwei")
	depositDataCmd.Flags().StringVar(&expectedWithdrawalCred, "withdrawal-credentials", "", "Expected withdrawal credentials (hex)")
	depositDataCmd.Flags().IntVar(&expectedCount, "count", 0, "Expected number of deposits")

	err := depositDataCmd.MarkFlagRequired("deposit-data")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "deposit-data")
	}
}

func verifyDepositData() error {
	depositData, err := deposit.NewDepositData(
		depositDataPath,
		expectedNetwork,
		expectedWithdrawalCred,
		expectedAmount,
		expectedCount,
	)

	if err != nil {
		return errors.Wrap(err, "failed to load deposit data")
	}

	if err := depositData.Validate(); err != nil {
		return errors.Wrap(err, "failed to validate deposit data")
	}

	if err := depositData.Verify(); err != nil {
		return errors.Wrap(err, "failed to verify deposit data")
	}

	pubkeys := make([]string, len(depositData.DepositData))
	for i, d := range depositData.DepositData {
		pubkeys[i] = "0x" + d.Deposit.PubKey
	}

	fmt.Printf("✅ Successfully verified %d deposit(s)\n", len(depositData.DepositData))
	fmt.Println("Pubkeys one line for copy paste:")
	fmt.Printf("[\"%s\"]\n", strings.Join(pubkeys, "\", \""))

	return nil
}
