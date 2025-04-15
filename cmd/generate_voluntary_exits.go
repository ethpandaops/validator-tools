package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/ethpandaops/validator-tools/pkg/validator"
)

var (
	voluntaryExitsOutputDir             string
	voluntaryExitsInputDir              string
	voluntaryExitsInputPrefix           string
	voluntaryExitsWithdrawCreds         string
	voluntaryExitsPassphrase            string
	voluntaryExitsBeaconURL             string
	voluntaryDomainBlsToExecutionChange string
	voluntaryExitsIterations            int
	voluntaryExitsIndexStart            int
	voluntaryExitsIndexOffset           int
	voluntaryExitsWorkers               int
)

var generateVoluntaryExitsCmd = &cobra.Command{
	Use:   "voluntary_exits",
	Short: "Generate validator voluntary exit messages",
	Long: `Generate validator voluntary exit messages for multiple keystores.
This command processes keystore files and generates exit messages using ethdo.
It requires ethdo, jq, and curl to be installed on the system.

The command supports parallel processing using multiple workers, each with its own
temporary directory for ethdo operations. The number of workers can be specified
with the --workers flag, defaulting to the number of CPU cores.`,
	RunE: runGenerateVoluntaryExits,
}

func init() {
	// Default to number of CPU threads if possible, otherwise use 1
	defaultWorkers := runtime.NumCPU()
	if defaultWorkers < 1 {
		defaultWorkers = 1
	}

	generateCmd.AddCommand(generateVoluntaryExitsCmd)

	generateVoluntaryExitsCmd.Flags().StringVar(&voluntaryExitsOutputDir, "output", "", "Path to directory where result files will be written")
	generateVoluntaryExitsCmd.Flags().StringVar(&voluntaryExitsInputDir, "input", "", "Path to directory containing keystore files")
	generateVoluntaryExitsCmd.Flags().StringVar(&voluntaryExitsInputPrefix, "prefix", "keystore-", "Prefix for input files to match")
	generateVoluntaryExitsCmd.Flags().StringVar(&voluntaryExitsWithdrawCreds, "withdrawal-credentials", "", "Withdrawal credentials (hex)")
	generateVoluntaryExitsCmd.Flags().StringVar(&voluntaryExitsPassphrase, "passphrase", "", "Passphrase for your keystore(s)")
	generateVoluntaryExitsCmd.Flags().StringVar(&voluntaryExitsBeaconURL, "beacon", "", "Beacon node endpoint URL (e.g. 'http://localhost:5052')")
	generateVoluntaryExitsCmd.Flags().IntVar(&voluntaryExitsIterations, "count", 50000, "Number of validators to process")
	generateVoluntaryExitsCmd.Flags().IntVar(&voluntaryExitsIndexStart, "index-start", -1, "Starting validator index (optional, will query beacon node if not set)")
	generateVoluntaryExitsCmd.Flags().IntVar(&voluntaryExitsIndexOffset, "index-offset", 0, "Offset to add to the starting validator index")
	generateVoluntaryExitsCmd.Flags().IntVar(&voluntaryExitsWorkers, "workers", defaultWorkers, "Number of parallel workers (default: number of CPU cores)")
	generateVoluntaryExitsCmd.Flags().StringVar(&voluntaryDomainBlsToExecutionChange, "domain-bls-to-execution-change", "", "BLS to execution change domain (optional, may be required as only some clients provide DOMAIN_BLS_TO_EXECUTION_CHANGE via /eth/v1/config/spec)")

	if err := generateVoluntaryExitsCmd.MarkFlagRequired("output"); err != nil {
		panic(err)
	}

	if err := generateVoluntaryExitsCmd.MarkFlagRequired("input"); err != nil {
		panic(err)
	}

	if err := generateVoluntaryExitsCmd.MarkFlagRequired("withdrawal-credentials"); err != nil {
		panic(err)
	}

	if err := generateVoluntaryExitsCmd.MarkFlagRequired("passphrase"); err != nil {
		panic(err)
	}

	if err := generateVoluntaryExitsCmd.MarkFlagRequired("beacon"); err != nil {
		panic(err)
	}
}

func runGenerateVoluntaryExits(cmd *cobra.Command, args []string) error {
	if voluntaryExitsWorkers < 1 {
		return errors.New("number of workers must be at least 1")
	}

	if _, err := exec.LookPath("ethdo"); err != nil {
		return errors.Errorf("Required command 'ethdo' not found. Please install it first.\nFor ethdo, please visit: https://github.com/wealdtech/ethdo")
	}

	if err := os.MkdirAll(voluntaryExitsOutputDir, 0o755); err != nil {
		return errors.Wrap(err, "failed to create output directory")
	}

	// Read all keystore files from input directory
	log.Infof("Reading keystore files from directory: %s", voluntaryExitsInputDir)
	log.Infof("Using file prefix: %s", voluntaryExitsInputPrefix)

	entries, err := os.ReadDir(voluntaryExitsInputDir)
	if err != nil {
		return errors.Wrap(err, "failed to read input directory")
	}

	log.Infof("Found %d total entries in directory", len(entries))

	var keystoreFiles []string

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), voluntaryExitsInputPrefix) {
			keystorePath := filepath.Join(voluntaryExitsInputDir, entry.Name())
			keystoreFiles = append(keystoreFiles, keystorePath)
			log.Debugf("Added keystore file: %s", keystorePath)
		} else {
			log.Debugf("Skipping entry: %s (is directory: %t, has prefix: %t)",
				entry.Name(), entry.IsDir(), strings.HasPrefix(entry.Name(), voluntaryExitsInputPrefix))
		}
	}

	log.Infof("Found %d matching keystore files", len(keystoreFiles))

	if len(keystoreFiles) == 0 {
		return errors.New("no keystore files found in input directory")
	}

	generator := validator.NewVoluntaryExitGenerator(
		voluntaryExitsOutputDir,
		voluntaryExitsWithdrawCreds,
		voluntaryExitsPassphrase,
		voluntaryExitsBeaconURL,
		voluntaryExitsIterations,
		voluntaryExitsIndexStart,
		voluntaryExitsIndexOffset,
		voluntaryExitsWorkers,
	)

	// Set total number of keystores
	generator.SetTotalKeystores(len(keystoreFiles))

	startIdx, err := generator.GetValidatorStartIndex()
	if err != nil {
		return errors.Wrap(err, "failed to get validator start index")
	}

	config, err := generator.FetchBeaconConfig()
	if err != nil {
		return errors.Wrap(err, "failed to fetch beacon configuration")
	}

	if voluntaryDomainBlsToExecutionChange != "" {
		config.BlsToExecutionChangeDomain = voluntaryDomainBlsToExecutionChange
	}

	log.Info("Beacon configuration fetched successfully")
	log.Infof("Latest validator index on chain: %d", startIdx)
	log.Infof("Using %d workers for parallel processing", voluntaryExitsWorkers)
	log.Infof("Processing %d keystores", len(keystoreFiles))

	for _, keystore := range keystoreFiles {
		log.Infof("Processing keystore: %s", keystore)

		if err := generator.GenerateExits(keystore, config, startIdx); err != nil {
			return errors.Wrapf(err, "failed to generate exits for keystore: %s", keystore)
		}
	}

	log.Infof("Processing complete. Processed %d iterations for each keystore.", voluntaryExitsIterations)

	return nil
}
