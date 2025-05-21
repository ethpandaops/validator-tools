package cmd

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/ethpandaops/validator-tools/pkg/validator"
)

var (
	verifyExitsInput                   string
	verifyExitsNetwork                 string
	verifyExitsWithdrawalCreds         string
	verifyExitsNumExits                int
	verifyExitsPubkeys                 []string
	verifyExitsSkipIndexMissmatchCheck bool
	verifyExitsSkipMessage             bool
)

var verifyVoluntaryExitsCmd = &cobra.Command{
	Use:   "voluntary_exits",
	Short: "Verify voluntary exit messages",
	Long:  `Verify voluntary exit messages for Ethereum validators.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		exits, err := validator.NewVoluntaryExits(verifyExitsInput, verifyExitsNetwork, verifyExitsWithdrawalCreds, verifyExitsPubkeys)
		if err != nil {
			return errors.Wrap(err, "failed to verify exits")
		}

		err = exits.ValidateCount(verifyExitsNumExits)
		if err != nil {
			return errors.Wrap(err, "failed to check exit count")
		}

		if !verifyExitsSkipIndexMissmatchCheck {
			err = exits.ValidateIndices()
			if err != nil {
				return errors.Wrap(err, "failed to check exit indices")
			}
		}

		rsp, err := exits.Verify()
		if err != nil {
			return errors.Wrap(err, "failed to verify exits")
		}

		if !verifyExitsSkipMessage {
			log.WithFields(logrus.Fields{
				"first_validator_index": rsp.FirstIndex,
				"last_validator_index":  rsp.LastIndex,
				"network":               verifyExitsNetwork,
			}).Info("Please check that the latest live validator index sits between these values.")

			log.Info("You can use a command like this to check the current highest finalized validator index: \n\n" +
				"curl -H \"Content-Type: application/json\" http://localhost:5052/eth/v1/beacon/states/finalized/validators | jq -r '[.data[].index | tonumber] | max' \n\n")
		}

		fmt.Printf("✅ Successfully verified %d sets of validator exits\n", len(exits.ExitsByPubkey))

		return nil
	},
}

func init() {
	verifyCmd.AddCommand(verifyVoluntaryExitsCmd)

	verifyVoluntaryExitsCmd.Flags().StringVar(&verifyExitsInput, "input", "", "Path to directory containing exit files")
	verifyVoluntaryExitsCmd.Flags().StringVar(&verifyExitsNetwork, "network", "", "Network (mainnet, holesky or hoodi)")
	verifyVoluntaryExitsCmd.Flags().StringVar(&verifyExitsWithdrawalCreds, "withdrawal-credentials", "", "Withdrawal credentials (hex)")
	verifyVoluntaryExitsCmd.Flags().IntVar(&verifyExitsNumExits, "count", 0, "Number of exits that should have been generated")
	verifyVoluntaryExitsCmd.Flags().StringSliceVar(&verifyExitsPubkeys, "pubkeys", []string{}, "Expected validator pubkeys (comma-separated)")
	verifyVoluntaryExitsCmd.Flags().BoolVar(&verifyExitsSkipIndexMissmatchCheck, "skip-index-missmatch-check", false, "Skip validator index missmatch check")
	verifyVoluntaryExitsCmd.Flags().BoolVar(&verifyExitsSkipMessage, "skip-check-message", false, "Skip check message")

	err := verifyVoluntaryExitsCmd.MarkFlagRequired("input")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "input")
	}

	err = verifyVoluntaryExitsCmd.MarkFlagRequired("network")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "network")
	}

	err = verifyVoluntaryExitsCmd.MarkFlagRequired("withdrawal-credentials")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "withdrawal-credentials")
	}

	err = verifyVoluntaryExitsCmd.MarkFlagRequired("pubkeys")
	if err != nil {
		log.WithError(err).Fatalf("Failed to mark flag %s as required", "pubkeys")
	}
}
