// Copyright 2018 The go-fiscobcos Authors
// This file is part of the go-fiscobcos library.
//
// The go-fiscobcos library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-fiscobcos library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-fiscobcos library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"reflect"

	"github.com/chislab/go-fiscobcos/accounts"
	"github.com/chislab/go-fiscobcos/common"
	"github.com/chislab/go-fiscobcos/common/hexutil"
	"github.com/chislab/go-fiscobcos/log"
)

const (
	// numberOfAccountsToDerive For hardware wallets, the number of accounts to derive
	numberOfAccountsToDerive = 10
	// ExternalAPIVersion -- see extapi_changelog.md
	ExternalAPIVersion = "6.0.0"
	// InternalAPIVersion -- see intapi_changelog.md
	InternalAPIVersion = "7.0.1"
)

// Validator defines the methods required to validate a transaction against some
// sanity defaults as well as any underlying 4byte method database.
//
// Use fourbyte.Database as an implementation. It is separated out of this package
// to allow pieces of the signer package to be used without having to load the
// 7MB embedded 4byte dump.
type Validator interface {
	// ValidateTransaction does a number of checks on the supplied transaction, and
	// returns either a list of warnings, or an error (indicating that the transaction
	// should be immediately rejected).
	ValidateTransaction(selector *string, tx *SendTxArgs) (*ValidationMessages, error)
}

// Metadata about a request
type Metadata struct {
	Remote    string `json:"remote"`
	Local     string `json:"local"`
	Scheme    string `json:"scheme"`
	UserAgent string `json:"User-Agent"`
	Origin    string `json:"Origin"`
}

// MetadataFromContext extracts Metadata from a given context.Context
func MetadataFromContext(ctx context.Context) Metadata {
	m := Metadata{"NA", "NA", "NA", "", ""} // batman

	if v := ctx.Value("remote"); v != nil {
		m.Remote = v.(string)
	}
	if v := ctx.Value("scheme"); v != nil {
		m.Scheme = v.(string)
	}
	if v := ctx.Value("local"); v != nil {
		m.Local = v.(string)
	}
	if v := ctx.Value("Origin"); v != nil {
		m.Origin = v.(string)
	}
	if v := ctx.Value("User-Agent"); v != nil {
		m.UserAgent = v.(string)
	}
	return m
}

// String implements Stringer interface
func (m Metadata) String() string {
	s, err := json.Marshal(m)
	if err == nil {
		return string(s)
	}
	return err.Error()
}

// types for the requests/response types between signer and UI
type (
	// SignTxRequest contains info about a Transaction to sign
	SignTxRequest struct {
		Transaction SendTxArgs       `json:"transaction"`
		Callinfo    []ValidationInfo `json:"call_info"`
		Meta        Metadata         `json:"meta"`
	}
	// SignTxResponse result from SignTxRequest
	SignTxResponse struct {
		//The UI may make changes to the TX
		Transaction SendTxArgs `json:"transaction"`
		Approved    bool       `json:"approved"`
	}
	SignDataRequest struct {
		ContentType string                  `json:"content_type"`
		Address     common.MixedcaseAddress `json:"address"`
		Rawdata     []byte                  `json:"raw_data"`
		Hash        hexutil.Bytes           `json:"hash"`
		Meta        Metadata                `json:"meta"`
	}
	SignDataResponse struct {
		Approved bool `json:"approved"`
	}
	NewAccountRequest struct {
		Meta Metadata `json:"meta"`
	}
	NewAccountResponse struct {
		Approved bool `json:"approved"`
	}
	ListRequest struct {
		Accounts []accounts.Account `json:"accounts"`
		Meta     Metadata           `json:"meta"`
	}
	ListResponse struct {
		Accounts []accounts.Account `json:"accounts"`
	}
	Message struct {
		Text string `json:"text"`
	}
	StartupInfo struct {
		Info map[string]interface{} `json:"info"`
	}
	UserInputRequest struct {
		Title      string `json:"title"`
		Prompt     string `json:"prompt"`
		IsPassword bool   `json:"isPassword"`
	}
	UserInputResponse struct {
		Text string `json:"text"`
	}
)

var ErrRequestDenied = errors.New("request denied")

// logDiff logs the difference between the incoming (original) transaction and the one returned from the signer.
// it also returns 'true' if the transaction was modified, to make it possible to configure the signer not to allow
// UI-modifications to requests
func logDiff(original *SignTxRequest, new *SignTxResponse) bool {
	modified := false
	if f0, f1 := original.Transaction.From, new.Transaction.From; !reflect.DeepEqual(f0, f1) {
		log.Info("Sender-account changed by UI", "was", f0, "is", f1)
		modified = true
	}
	if t0, t1 := original.Transaction.To, new.Transaction.To; !reflect.DeepEqual(t0, t1) {
		log.Info("Recipient-account changed by UI", "was", t0, "is", t1)
		modified = true
	}
	if g0, g1 := original.Transaction.Gas, new.Transaction.Gas; g0 != g1 {
		modified = true
		log.Info("Gas changed by UI", "was", g0, "is", g1)
	}
	if g0, g1 := big.Int(original.Transaction.GasPrice), big.Int(new.Transaction.GasPrice); g0.Cmp(&g1) != 0 {
		modified = true
		log.Info("GasPrice changed by UI", "was", g0, "is", g1)
	}
	if v0, v1 := big.Int(original.Transaction.Value), big.Int(new.Transaction.Value); v0.Cmp(&v1) != 0 {
		modified = true
		log.Info("Value changed by UI", "was", v0, "is", v1)
	}
	if d0, d1 := original.Transaction.Data, new.Transaction.Data; d0 != d1 {
		d0s := ""
		d1s := ""
		if d0 != nil {
			d0s = hexutil.Encode(*d0)
		}
		if d1 != nil {
			d1s = hexutil.Encode(*d1)
		}
		if d1s != d0s {
			modified = true
			log.Info("Data changed by UI", "was", d0s, "is", d1s)
		}
	}
	if n0, n1 := original.Transaction.RandomId, new.Transaction.RandomId; n0 != n1 {
		modified = true
		log.Info("Nonce changed by UI", "was", n0, "is", n1)
	}
	return modified
}
