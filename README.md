# validator-tools

This repository contains various tools for validators.

## Usage

### Deposit Data

Verify deposit data json file with expected network and withdrawal credentials.

```
validator-tools deposit data \
    --network <holesky|mainnet> \
    --deposit-data <PATH> # Path to deposit data json file \
    --count <COUNT> # Expected number of deposits in the file \
    --withdrawal-credentials <WITHDRAWAL_CREDENTIALS>
```

### Deposit Exits

Verify (pre-signed) voluntary exits json files with expected network and withdrawal credentials.

> Files within the directory must follow the format `<validator_index>-<pubkey>.json`

```
validator-tools deposit exits \
    --network <holesky|mainnet> \
    --path <PATH> # Path to directory containing voluntary exits json files \
    --count <COUNT> # Expected number of exits per validator pubkey eg. 50000 \
    --withdrawal-credentials <WITHDRAWAL_CREDENTIALS>
```
