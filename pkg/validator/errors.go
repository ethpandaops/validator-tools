package validator

import "errors"

// Static errors for the validator package.
var (
	// Generator errors.
	ErrEmptyPubkey = errors.New("empty or null pubkey in keystore")

	// Voluntary exits errors.
	ErrUnknownNetwork              = errors.New("unknown network")
	ErrUnexpectedPubkey            = errors.New("unexpected pubkey found")
	ErrExpectedPubkeyNotFound      = errors.New("expected pubkey not found")
	ErrNoVoluntaryExitsFound       = errors.New("no voluntary exits found")
	ErrFilesCountMismatch          = errors.New("files count mismatch")
	ErrExitsCountMismatch          = errors.New("exits count mismatch")
	ErrNoExitsForPubkey            = errors.New("no exits found for pubkey")
	ErrMinIndexMismatch            = errors.New("minimum validator index mismatch")
	ErrMaxIndexMismatch            = errors.New("maximum validator index mismatch")
	ErrInvalidFileNameFormat       = errors.New("invalid file name format")
	ErrInvalidValidatorIndex       = errors.New("invalid validator index in filename")
	ErrInvalidPubkeyInFilename     = errors.New("invalid pubkey in filename")
	ErrValidatorIndexMismatch      = errors.New("validator index mismatch")
	ErrInvalidPubkeyFormat         = errors.New("invalid pubkey format")
	ErrPubkeyMismatch              = errors.New("pubkey mismatch")
	ErrInvalidValidatorIndexFormat = errors.New("invalid validator index format")
	ErrInvalidSignature            = errors.New("invalid signature")
	ErrFailedToProcessExit         = errors.New("failed to process exit")
	ErrExitNotFound                = errors.New("exit not found")
	ErrValidatorNotFound           = errors.New("validator not found in beacon state")
	ErrValidatorNotActive          = errors.New("validator is not active")
	ErrValidatorNotProcessed       = errors.New("validator was not processed")

	// Config errors.
	ErrInvalidBeaconConfig     = errors.New("invalid beacon configuration")
	ErrEmptyGenesisTime        = errors.New("genesis time is empty")
	ErrEmptyConfigName         = errors.New("config name is empty")
	ErrEmptyGenesisForkVersion = errors.New("genesis fork version is empty")

	// Deposit data errors.
	ErrEmptyNetwork            = errors.New("network is empty")
	ErrEmptyWithdrawalCred     = errors.New("withdrawal credential is empty")
	ErrEmptyPath               = errors.New("path is empty")
	ErrNetworkMismatch         = errors.New("network mismatch")
	ErrWithdrawalCredMismatch  = errors.New("withdrawal credential mismatch")
	ErrAmountMismatch          = errors.New("amount mismatch")
	ErrCountMismatch           = errors.New("count mismatch")
	ErrInvalidPublicKey        = errors.New("invalid public key")
	ErrInvalidSignatureDeposit = errors.New("invalid deposit signature")
)
