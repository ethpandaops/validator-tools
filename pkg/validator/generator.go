package validator

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"

	"github.com/pkg/errors"
)

type VoluntaryExitGenerator struct {
	OutputDir             string
	WithdrawalCredentials string
	Passphrase            string
	BeaconURL             string
	Iterations            int
	IndexStart            int
	IndexOffset           int
	NumWorkers            int
	TotalKeystores        int32
	CurrentKeystore       int32
}

func NewVoluntaryExitGenerator(outputDir, withdrawalCreds, passphrase, beaconURL string, iterations, indexStart, indexOffset, numWorkers int) *VoluntaryExitGenerator {
	log.Info("Creating new Generator")
	log.Infof("Output dir: %s", outputDir)
	log.Infof("Withdrawal creds: %s", withdrawalCreds)
	log.Infof("Beacon URL: %s", beaconURL)
	log.Infof("Iterations: %d", iterations)

	if indexStart >= 0 {
		log.Infof("Start index: %d", indexStart+indexOffset)
	}

	log.Infof("Number of workers: %d", numWorkers)

	return &VoluntaryExitGenerator{
		OutputDir:             outputDir,
		WithdrawalCredentials: withdrawalCreds,
		Passphrase:            passphrase,
		BeaconURL:             beaconURL,
		Iterations:            iterations,
		IndexStart:            indexStart,
		IndexOffset:           indexOffset,
		NumWorkers:            numWorkers,
		CurrentKeystore:       0,
		TotalKeystores:        0,
	}
}

func (g *VoluntaryExitGenerator) SetTotalKeystores(total int) {
	if total < 0 {
		log.Warnf("Negative total keystores %d, setting to 0", total)

		g.TotalKeystores = 0

		return
	}

	if total > math.MaxInt32 {
		log.Warnf("Total keystores %d exceeds maximum int32 value, capping at %d", total, math.MaxInt32)

		g.TotalKeystores = math.MaxInt32
	} else {
		g.TotalKeystores = int32(total)
	}

	log.Infof("Total keystores to process: %d", g.TotalKeystores)
}

func (g *VoluntaryExitGenerator) GetValidatorStartIndex() (int, error) {
	if g.IndexStart >= 0 {
		return g.IndexStart + g.IndexOffset, nil
	}

	resp, err := g.FetchJSON(g.BeaconURL + "/eth/v1/beacon/states/head/validators")
	if err != nil {
		return 0, err
	}

	var result struct {
		Data []struct {
			Index string `json:"index"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return 0, errors.Wrap(err, "failed to parse validator response")
	}

	maxIndex := -1

	for _, v := range result.Data {
		index, err := strconv.Atoi(v.Index)
		if err != nil {
			continue
		}

		if index > maxIndex {
			maxIndex = index
		}
	}

	if maxIndex == -1 {
		return 0, errors.New("no valid validator indices found")
	}

	return maxIndex + g.IndexOffset, nil
}

func (g *VoluntaryExitGenerator) GenerateExits(keystorePath string, config *BeaconConfig, startIndex int) error {
	atomic.AddInt32(&g.CurrentKeystore, 1)
	keystoreNum := atomic.LoadInt32(&g.CurrentKeystore)

	log.Info("Generating exits")
	log.Infof("Processing keystore %d/%d: %s", keystoreNum, g.TotalKeystores, keystorePath)
	log.Infof("Start index: %d", startIndex)

	absKeystorePath, err := filepath.Abs(keystorePath)
	if err != nil {
		log.Errorf("Failed to get absolute path for keystore: %v", err)

		return errors.Wrapf(err, "failed to get absolute path for keystore: %s", keystorePath)
	}

	log.Infof("Absolute keystore path: %s", absKeystorePath)

	log.Info("Reading pubkey from keystore")

	keystoreData, err := os.ReadFile(absKeystorePath)
	if err != nil {
		log.Errorf("Failed to read keystore file: %v", err)

		return errors.Wrapf(err, "failed to read keystore file: %s", absKeystorePath)
	}

	var keystoreJSON struct {
		Pubkey string `json:"pubkey"`
	}

	if err := json.Unmarshal(keystoreData, &keystoreJSON); err != nil {
		log.Errorf("Failed to parse keystore JSON: %v", err)

		return errors.Wrapf(err, "failed to parse keystore JSON: %s", absKeystorePath)
	}

	if keystoreJSON.Pubkey == "" {
		log.Error("Empty or null pubkey in keystore")

		return fmt.Errorf("empty or null pubkey in keystore: %s", absKeystorePath)
	}

	log.Infof("Pubkey: %s", keystoreJSON.Pubkey)

	tasks := make(chan exitTask, g.Iterations)

	log.Info("Sending tasks to workers")

	for i := 1; i <= g.Iterations; i++ {
		tasks <- exitTask{
			validatorIndex: startIndex + i,
			pubkey:         keystoreJSON.Pubkey,
			keystorePath:   absKeystorePath,
		}
	}

	close(tasks)

	if err := g.processExitTasks(tasks, config, keystoreNum); err != nil {
		return err
	}

	log.Infof("Exit generation completed for keystore %d/%d", keystoreNum, g.TotalKeystores)

	return nil
}
