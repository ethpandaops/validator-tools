package validator

import (
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type commander interface {
	CombinedOutput() ([]byte, error)
}

var execCommand = func(name string, args ...string) commander {
	return exec.Command(name, args...)
}

// runEthdoCommand executes the ethdo command for generating voluntary exits
func (g *VoluntaryExitGenerator) runEthdoCommand(keystorePath, outFile, workDir string, log *logrus.Entry) error {
	log.Debug("Running ethdo command")
	log.Debugf("Keystore path: %s", keystorePath)
	log.Debugf("Output file: %s", outFile)
	log.Debugf("Working directory: %s", workDir)

	args := []string{
		"validator", "exit",
		"--validator=" + keystorePath,
		"--passphrase=" + g.Passphrase,
		"--json",
		"--offline",
	}

	// Redact passphrase in debug output
	debugArgs := make([]string, len(args))
	copy(debugArgs, args)

	for i, arg := range debugArgs {
		if strings.HasPrefix(arg, "--passphrase=") {
			debugArgs[i] = "--passphrase=********"
		}
	}

	log.Debugf("Executing command: ethdo %s", strings.Join(debugArgs, " "))

	cmd := execCommand("ethdo", args...)
	if execCmd, ok := cmd.(*exec.Cmd); ok {
		execCmd.Dir = workDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("ethdo command failed: %v", err)
		log.Errorf("Command output: %s", string(output))

		return errors.Wrapf(err, "ethdo command failed: %s", string(output))
	}

	if err := os.WriteFile(outFile, output, 0o600); err != nil {
		log.Errorf("Failed to write output file: %v", err)

		return errors.Wrapf(err, "failed to write output file: %s", outFile)
	}

	log.Debug("ethdo command completed successfully")

	return nil
}
