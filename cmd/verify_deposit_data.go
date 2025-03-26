package cmd

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/ethpandaops/validator-tools/pkg/validator"
)

var (
	verifyDepositDataPath        string
	verifyExpectedNetwork        string
	verifyExpectedAmount         uint64
	verifyExpectedWithdrawalCred string
	verifyExpectedCount          int
)

// Function signature type for verifyDepositData
type verifyDepositDataFunc func() error

// Default implementation
var verifyDepositData verifyDepositDataFunc = func() error {
	depositData, err := validator.NewData(
		verifyDepositDataPath,
		verifyExpectedNetwork,
		verifyExpectedWithdrawalCred,
		verifyExpectedAmount,
		verifyExpectedCount,
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

	log.WithFields(logrus.Fields{
		"deposit_count": len(depositData.DepositData),
	}).Info("✅ Successfully verified deposit data")

	fmt.Printf("[\"%s\"]\n", strings.Join(pubkeys, "\", \""))

	return nil
}

var verifyDepositDataCmd = &cobra.Command{
	Use:   "deposit_data",
	Short: "Verify deposit data",
	Long:  `Verifies and validates deposit data file format and contents.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return verifyDepositData()
	},
	// Don't show usage on error
	SilenceUsage: true,
}

func init() {
	verifyCmd.AddCommand(verifyDepositDataCmd)

	verifyDepositDataCmd.Flags().StringVar(&verifyDepositDataPath, "deposit-data", "", "Path to deposit data JSON file")
	verifyDepositDataCmd.Flags().StringVar(&verifyExpectedNetwork, "network", "", "Expected network (e.g. mainnet, goerli)")
	verifyDepositDataCmd.Flags().Uint64Var(&verifyExpectedAmount, "amount", 32000000000, "Expected deposit amount in Gwei")
	verifyDepositDataCmd.Flags().StringVar(&verifyExpectedWithdrawalCred, "withdrawal-credentials", "", "Expected withdrawal credentials (hex)")
	verifyDepositDataCmd.Flags().IntVar(&verifyExpectedCount, "count", 0, "Expected number of deposits")

	err := verifyDepositDataCmd.MarkFlagRequired("deposit-data")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "deposit-data")
	}
}
