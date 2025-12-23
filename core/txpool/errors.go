// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package txpool

import (
	"errors"

	corepkg "github.com/ethereum/go-ethereum/core"
)

var (
	// ErrAlreadyKnown is returned if the transactions is already contained
	// within the pool.
	ErrAlreadyKnown = errors.New("already known")

	// ErrInvalidSender is returned if the transaction contains an invalid signature.
	ErrInvalidSender = errors.New("invalid sender")

	// ErrUnderpriced is returned if a transaction's gas price is below the minimum
	// configured for the transaction pool.
	ErrUnderpriced = errors.New("transaction underpriced")

	// ErrReplaceUnderpriced is returned if a transaction is attempted to be replaced
	// with a different one without the required price bump.
	ErrReplaceUnderpriced = errors.New("replacement transaction underpriced")

	// ErrAccountLimitExceeded is returned if a transaction would exceed the number
	// allowed by a pool for a single account.
	ErrAccountLimitExceeded = errors.New("account limit exceeded")

	// ErrGasLimit is returned if a transaction's requested gas limit exceeds the
	// maximum allowance of the current block.
	ErrGasLimit = errors.New("exceeds block gas limit")

	// ErrNegativeValue is a sanity error to ensure no one is able to specify a
	// transaction with a negative value.
	ErrNegativeValue = errors.New("negative value")

	// ErrUnderMinAmount is returned if a transaction's value is below the
	// configured minimum amount.
	ErrUnderMinAmount = errors.New("transaction value below minimum")

	// ErrOversizedData is returned if the input data of a transaction is greater
	// than some meaningful limit a user might use. This is not a consensus error
	// making the transaction invalid, rather a DOS protection.
	ErrOversizedData = errors.New("oversized data")

	// ErrFutureReplacePending is returned if a future transaction replaces a pending
	// one. Future transactions should only be able to replace other future transactions.
	ErrFutureReplacePending = errors.New("future transaction tries to replace pending")

	// ErrAlreadyReserved is returned if the sender address has a pending transaction
	// in a different subpool. For example, this error is returned in response to any
	// input transaction of non-blob type when a blob transaction from this sender
	// remains pending (and vice-versa).
	ErrAlreadyReserved = errors.New("address already reserved")

	// ErrRateLimit is returned if an account exceeds the allowed number of
	// transactions within the configured time window.
	ErrRateLimit = corepkg.ErrRateLimit

	// ErrOverMaxAmount is returned if a transaction value exceeds the configured
	// per-transaction maximum during off-session periods.
	ErrOverMaxAmount = corepkg.ErrOverMaxOffSession

	// ErrManagementUnauthorized is returned if a management contract transaction
	// originates from a non-administrator account.
	ErrManagementUnauthorized = corepkg.ErrUnauthorizedManagementTx

	// ErrDividendNotEligible is returned if a dividend claim transaction is not
	// eligible at the current time (no active round, outside window, or already claimed).
	ErrDividendNotEligible = errors.New("dividend claim not eligible")

	// ErrDividendRoundTooSoon is returned when a new dividend round is triggered
	// before the cooldown completes.
	ErrDividendRoundTooSoon = errors.New("dividend round still cooling down")
)
