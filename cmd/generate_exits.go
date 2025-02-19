package cmd

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/ethpandaops/validator-tools/pkg/deposit"
)

var (
	genExitsOutputDir     string
	genExitsWithdrawCreds string
	genExitsPassphrase    string
	genExitsBeaconURL     string
	genExitsIterations    int
	genExitsStartIndex    int
	genExitsWorkers       int
)

var depositGenerateExitsCmd = &cobra.Command{
	Use:   "generate-exits [keystore_files...]",
	Short: "Generate validator exit messages",
	Long: `Generate validator exit messages for multiple keystores.
This command processes keystore files and generates exit messages using ethdo.
It requires ethdo, jq, and curl to be installed on the system.

The command supports parallel processing using multiple workers, each with its own
temporary directory for ethdo operations. The number of workers can be specified
with the --workers flag, defaulting to the number of CPU cores.`,
	RunE: runGenerateExits,
}

func init() {
	// Default to number of CPU threads if possible, otherwise use 1
	defaultWorkers := runtime.NumCPU()
	if defaultWorkers < 1 {
		defaultWorkers = 1
	}

	depositCmd.AddCommand(depositGenerateExitsCmd)

	depositGenerateExitsCmd.Flags().StringVar(&genExitsOutputDir, "path", "", "Path to directory where result files will be written")
	depositGenerateExitsCmd.Flags().StringVar(&genExitsWithdrawCreds, "withdrawal-credentials", "", "Withdrawal credentials (hex)")
	depositGenerateExitsCmd.Flags().StringVar(&genExitsPassphrase, "passphrase", "", "Passphrase for your keystore(s)")
	depositGenerateExitsCmd.Flags().StringVar(&genExitsBeaconURL, "beacon", "", "Beacon node endpoint URL (e.g. 'http://localhost:5052')")
	depositGenerateExitsCmd.Flags().IntVar(&genExitsIterations, "count", 50000, "Number of validators to process")
	depositGenerateExitsCmd.Flags().IntVar(&genExitsStartIndex, "start", -1, "Starting validator index (optional, will query beacon node if not set)")
	depositGenerateExitsCmd.Flags().IntVar(&genExitsWorkers, "workers", defaultWorkers, "Number of parallel workers (default: number of CPU cores)")

	depositGenerateExitsCmd.MarkFlagRequired("path")
	depositGenerateExitsCmd.MarkFlagRequired("withdrawal-credentials")
	depositGenerateExitsCmd.MarkFlagRequired("passphrase")
	depositGenerateExitsCmd.MarkFlagRequired("beacon")
}

func checkDependencies() error {
	dependencies := []string{"jq", "curl", "ethdo"}
	for _, dep := range dependencies {
		if _, err := exec.LookPath(dep); err != nil {
			msg := "Required command '%s' not found. Please install it first."
			if dep == "ethdo" {
				msg += "\nFor ethdo, please visit: https://github.com/wealdtech/ethdo"
			} else {
				msg += "\nPlease install %s using your system's package manager"
			}
			return errors.Errorf(msg, dep)
		}
	}
	return nil
}

func runGenerateExits(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("at least one keystore file must be specified")
	}

	if genExitsWorkers < 1 {
		return errors.New("number of workers must be at least 1")
	}

	if err := checkDependencies(); err != nil {
		return err
	}

	if err := os.MkdirAll(genExitsOutputDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create output directory")
	}

	generator := deposit.NewExitGenerator(
		genExitsOutputDir,
		genExitsWithdrawCreds,
		genExitsPassphrase,
		genExitsBeaconURL,
		genExitsIterations,
		genExitsStartIndex,
		genExitsWorkers,
	)

	// Set total number of keystores
	generator.SetTotalKeystores(len(args))

	startIdx, err := generator.GetValidatorStartIndex()
	if err != nil {
		return errors.Wrap(err, "failed to get validator start index")
	}

	config, err := generator.FetchBeaconConfig()
	if err != nil {
		return errors.Wrap(err, "failed to fetch beacon configuration")
	}

	log.Info("Beacon configuration fetched successfully")
	log.Infof("Latest validator index on chain: %d", startIdx)
	log.Infof("Using %d workers for parallel processing", genExitsWorkers)
	log.Infof("Processing %d keystores", len(args))

	for _, keystore := range args {
		log.Infof("Processing keystore: %s", keystore)

		if err := generator.GenerateExits(keystore, config, startIdx); err != nil {
			return errors.Wrapf(err, "failed to generate exits for keystore: %s", keystore)
		}
	}

	log.Infof("Processing complete. Processed %d iterations for each keystore.", genExitsIterations)
	return nil
}
