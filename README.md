# validator-tools

This repository contains various tools for validators.

## Usage

### Deposit Data

Verify deposit data json file with expected network and withdrawal credentials.

```
validator-tools verify deposit_data \
    --network <mainnet|hoodi|holesky> \
    --deposit-data <PATH> # Path to deposit data json file \
    --count <COUNT> # Expected number of deposits in the file \
    --withdrawal-credentials <WITHDRAWAL_CREDENTIALS> \
    --amount <AMOUNT> # Expected deposit amount in Gwei (default: 32000000000)
```

### Voluntary Exits

#### Generate Voluntary Exits

Generate validator voluntary exit messages for multiple keystores. Requires ethdo, jq, and curl to be installed.

```
validator-tools generate voluntary_exits [keystore_files...] \
    --path <PATH> # Path to directory where result files will be written \
    --withdrawal-credentials <WITHDRAWAL_CREDENTIALS> \
    --passphrase <PASSPHRASE> # Passphrase for your keystore(s) \
    --beacon <URL> # Beacon node endpoint URL (e.g. 'http://localhost:5052') \
    --count <COUNT> # Number of validators to process (default: 50000) \
    --index-start <INDEX> # Starting validator index (optional) \
    --index-offset <OFFSET> # Offset to add to the starting validator index (default: 0) \
    --workers <COUNT> # Number of parallel workers (default: number of CPU cores)
```

#### Verify Voluntary Exits

Verify voluntary exit messages for Ethereum validators.

```
validator-tools verify voluntary_exits \
    --path <PATH> # Path to directory containing exit files \
    --network <mainnet|hoodi|holesky> \
    --withdrawal-credentials <WITHDRAWAL_CREDENTIALS> \
    --count <COUNT> # Number of exits that should have been generated
    --pubkeys <PUBKEYS> # Expected validator pubkeys (comma-separated)
```
