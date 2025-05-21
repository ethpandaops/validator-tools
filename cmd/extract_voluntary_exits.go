package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/ethpandaops/validator-tools/pkg/validator"
)

var (
	extractExitsInput           string
	extractExitsOutput          string
	extractExitsNetwork         string
	extractExitsWithdrawalCreds string
	extractExitsPubkeys         []string
	extractExitsBeaconURL       string
)

var extractVoluntaryExitsCmd = &cobra.Command{
	Use:   "voluntary_exits",
	Short: "Extract voluntary exit messages",
	Long:  `Extract voluntary exit messages for Ethereum validators.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		exits, err := validator.NewVoluntaryExits(extractExitsInput, extractExitsNetwork, extractExitsWithdrawalCreds, extractExitsPubkeys)
		if err != nil {
			return errors.Wrap(err, "failed to load exits")
		}

		err = exits.Extract(extractExitsBeaconURL, extractExitsOutput)
		if err != nil {
			return errors.Wrap(err, "failed to extract exits")
		}

		foundExits, err := validator.NewVoluntaryExits(extractExitsOutput, extractExitsNetwork, extractExitsWithdrawalCreds, extractExitsPubkeys)
		if err != nil {
			return errors.Wrap(err, "failed to load extracted exits")
		}

		err = foundExits.ValidateCount(1)
		if err != nil {
			return errors.Wrap(err, "failed to check exit count")
		}

		_, err = foundExits.Verify()
		if err != nil {
			return errors.Wrap(err, "failed to verify extracted exits")
		}

		fmt.Printf("✅ Successfully extracted %d sets of validator exits\n", len(exits.ExitsByPubkey))

		return nil
	},
}

func init() {
	extractCmd.AddCommand(extractVoluntaryExitsCmd)

	extractVoluntaryExitsCmd.Flags().StringVar(&extractExitsInput, "input", "", "Path to directory containing exit files")
	extractVoluntaryExitsCmd.Flags().StringVar(&extractExitsOutput, "output", "", "Path to directory to save extracted exit files")
	extractVoluntaryExitsCmd.Flags().StringVar(&extractExitsNetwork, "network", "", "Network (mainnet, holesky or hoodi)")
	extractVoluntaryExitsCmd.Flags().StringVar(&extractExitsWithdrawalCreds, "withdrawal-credentials", "", "Withdrawal credentials (hex)")
	extractVoluntaryExitsCmd.Flags().StringSliceVar(&extractExitsPubkeys, "pubkeys", []string{}, "Expected validator pubkeys (comma-separated)")
	extractVoluntaryExitsCmd.Flags().StringVar(&extractExitsBeaconURL, "beacon", "", "Beacon node endpoint URL (e.g. 'http://localhost:5052')")

	err := extractVoluntaryExitsCmd.MarkFlagRequired("input")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "input")
	}

	err = extractVoluntaryExitsCmd.MarkFlagRequired("output")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "output")
	}

	err = extractVoluntaryExitsCmd.MarkFlagRequired("network")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "network")
	}

	err = extractVoluntaryExitsCmd.MarkFlagRequired("withdrawal-credentials")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "withdrawal-credentials")
	}

	err = extractVoluntaryExitsCmd.MarkFlagRequired("pubkeys")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "pubkeys")
	}

	err = extractVoluntaryExitsCmd.MarkFlagRequired("beacon")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "beacon")
	}
}
