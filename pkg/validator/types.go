package validator

// BeaconConfig contains configuration data from the beacon node
type BeaconConfig struct {
	GenesisValidatorsRoot      string `json:"genesis_validators_root"`
	GenesisVersion             string `json:"genesis_fork_version"`
	ExitForkVersion            string `json:"exit_fork_version"`
	CurrentForkVersion         string `json:"current_fork_version"`
	Epoch                      string `json:"epoch"`
	BlsToExecutionChangeDomain string `json:"bls_to_execution_change_domain_type"`
	VoluntaryExitDomain        string `json:"voluntary_exit_domain_type"`
}

// ValidatorInfo contains information about a validator
type ValidatorInfo struct {
	Index                 string `json:"index"`
	Pubkey                string `json:"pubkey"`
	State                 string `json:"state"`
	WithdrawalCredentials string `json:"withdrawal_credentials"`
}

// PrepFile represents the preparation file for ethdo
type PrepFile struct {
	Version                    string          `json:"version"`
	Validators                 []ValidatorInfo `json:"validators"`
	GenesisValidatorsRoot      string          `json:"genesis_validators_root"`
	Epoch                      string          `json:"epoch"`
	GenesisVersion             string          `json:"genesis_fork_version"`
	ExitForkVersion            string          `json:"exit_fork_version"`
	CurrentForkVersion         string          `json:"current_fork_version"`
	BlsToExecutionChangeDomain string          `json:"bls_to_execution_change_domain_type"`
	VoluntaryExitDomain        string          `json:"voluntary_exit_domain_type"`
}

// exitTask represents a single validator exit task
type exitTask struct {
	validatorIndex int
	pubkey         string
	keystorePath   string
}
