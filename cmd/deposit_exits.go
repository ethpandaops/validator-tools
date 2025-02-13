package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/ethpandaops/validator-tools/pkg/deposit"
)

var (
	path            string
	network         string
	withdrawalCreds string
	numExits        int
)

var depositExitsCmd = &cobra.Command{
	Use:   "exits",
	Short: "Verifies voluntary exits",
	Long:  `Verifies voluntary exit messages for validators.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		initCommon()

		err := generateExits()
		if err != nil {
			log.Fatal(err)
		}

		return nil
	},
}

func init() {
	depositCmd.AddCommand(depositExitsCmd)

	depositExitsCmd.Flags().StringVar(&path, "path", "", "Path to directory containing exit files")
	depositExitsCmd.Flags().StringVar(&network, "network", "", "Network (mainnet or holesky)")
	depositExitsCmd.Flags().StringVar(&withdrawalCreds, "withdrawal-credentials", "", "Withdrawal credentials (hex)")
	depositExitsCmd.Flags().IntVar(&numExits, "count", 0, "Number of exits that should have be generated")

	err := depositExitsCmd.MarkFlagRequired("path")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "path")
	}

	err = depositExitsCmd.MarkFlagRequired("network")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "network")
	}

	err = depositExitsCmd.MarkFlagRequired("withdrawal-credentials")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "withdrawal-credentials")
	}
}

func generateExits() error {
	if network != "mainnet" && network != "holesky" {
		return errors.New("network must be either mainnet or holesky")
	}

	exits, err := deposit.NewVoluntaryExits(path, network, withdrawalCreds, numExits)
	if err != nil {
		return errors.Wrap(err, "failed to generate exits")
	}

	err = exits.Verify()
	if err != nil {
		return errors.Wrap(err, "failed to verify exits")
	}

	fmt.Printf("✅ Successfully verified %d sets of validator exits\n", len(exits.ExitsByPubkey))

	return nil
}
