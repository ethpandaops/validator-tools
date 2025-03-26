package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/ethpandaops/validator-tools/pkg/validator"
)

var (
	verifyExitsPath            string
	verifyExitsNetwork         string
	verifyExitsWithdrawalCreds string
	verifyExitsNumExits        int
)

var verifyVoluntaryExitsCmd = &cobra.Command{
	Use:   "voluntary_exits",
	Short: "Verify voluntary exit messages",
	Long:  `Verify voluntary exit messages for Ethereum validators.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		exits, err := validator.NewVoluntaryExits(verifyExitsPath, verifyExitsNetwork, verifyExitsWithdrawalCreds, verifyExitsNumExits)
		if err != nil {
			return errors.Wrap(err, "failed to generate exits")
		}

		err = exits.Verify()
		if err != nil {
			return errors.Wrap(err, "failed to verify exits")
		}

		fmt.Printf("✅ Successfully verified %d sets of validator exits\n", len(exits.ExitsByPubkey))

		return nil
	},
}

func init() {
	verifyCmd.AddCommand(verifyVoluntaryExitsCmd)

	verifyVoluntaryExitsCmd.Flags().StringVar(&verifyExitsPath, "path", "", "Path to directory containing exit files")
	verifyVoluntaryExitsCmd.Flags().StringVar(&verifyExitsNetwork, "network", "", "Network (mainnet or holesky)")
	verifyVoluntaryExitsCmd.Flags().StringVar(&verifyExitsWithdrawalCreds, "withdrawal-credentials", "", "Withdrawal credentials (hex)")
	verifyVoluntaryExitsCmd.Flags().IntVar(&verifyExitsNumExits, "count", 0, "Number of exits that should have be generated")

	err := verifyVoluntaryExitsCmd.MarkFlagRequired("path")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "path")
	}

	err = verifyVoluntaryExitsCmd.MarkFlagRequired("network")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "network")
	}

	err = verifyVoluntaryExitsCmd.MarkFlagRequired("withdrawal-credentials")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "withdrawal-credentials")
	}
}
