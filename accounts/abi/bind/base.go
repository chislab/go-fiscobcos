// Copyright 2015 The go-fiscobcos Authors
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

package bind

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/chislab/go-fiscobcos/common/hexutil"
	"github.com/chislab/go-fiscobcos/rlp"
	"github.com/pborman/uuid"
	"math/big"
	"time"

	"github.com/chislab/go-fiscobcos"
	"github.com/chislab/go-fiscobcos/accounts/abi"
	"github.com/chislab/go-fiscobcos/common"
	"github.com/chislab/go-fiscobcos/core/types"
	"github.com/chislab/go-fiscobcos/event"
)

// SignerFn is a signer function callback when a contract requires a method to
// sign the transaction before submission.
type SignerFn func(types.Signer, common.Address, *types.Transaction) (*types.Transaction, error)

// CallOpts is the collection of options to fine tune a contract call request.
type CallOpts struct {
	Pending     bool            // Whether to operate on the pending state or the last known one
	From        common.Address  // Optional the sender address, otherwise the first account is used
	BlockNumber *big.Int        // Optional the block number on which the call should be performed
	Context     context.Context // Network context to support cancellation and timeouts (nil = no timeout)

	GroupId int64
}

// TransactOpts is the collection of authorization data required to create a
// valid FiscoBcos transaction.
// TransactOpts is the collection of authorization data required to create a
// valid FiscoBcos transaction.

type TransactOpts struct {
	From     common.Address // FiscoBcos account to send the transaction from
	RandomId uint64         // RandomId to use for the transaction execution (nil = use pending state)
	Signer   SignerFn       // Method to use for signing the transaction (mandatory)

	Value    *big.Int // Funds to transfer along along the transaction (nil = 0 = no funds)
	GasPrice *big.Int // Gas price to use for the transaction execution (nil = gas price oracle)
	GasLimit uint64   // Gas limit to set for the transaction execution (0 = estimate)

	Context context.Context // Network context to support cancellation and timeouts (nil = no timeout)

	ChainId    int64
	GroupId    int64
	BlockLimit uint64
}

// FilterOpts is the collection of options to fine tune filtering for events
// within a bound contract.
type FilterOpts struct {
	Start uint64  // Start of the queried range
	End   *uint64 // End of the range (nil = latest)

	Context context.Context // Network context to support cancellation and timeouts (nil = no timeout)
}

// WatchOpts is the collection of options to fine tune subscribing for events
// within a bound contract.
type WatchOpts struct {
	Start   *uint64         // Start of the queried range (nil = latest)
	Context context.Context // Network context to support cancellation and timeouts (nil = no timeout)
}

// BoundContract is the base wrapper object that reflects a contract on the
// FiscoBcos network. It contains a collection of methods that are used by the
// higher level contract bindings to operate.
type BoundContract struct {
	address    common.Address     // Deployment address of the contract on the FiscoBcos blockchain
	abi        abi.ABI            // Reflect based ABI to access the correct FiscoBcos methods
	caller     ContractCaller     // Read interface to interact with the blockchain
	transactor ContractTransactor // Write interface to interact with the blockchain
	filterer   ContractFilterer   // Event filtering to interact with the blockchain
}

// NewBoundContract creates a low level contract interface through which calls
// and transactions may be made through.
func NewBoundContract(address common.Address, abi abi.ABI, caller ContractCaller, transactor ContractTransactor, filterer ContractFilterer) *BoundContract {
	return &BoundContract{
		address:    address,
		abi:        abi,
		caller:     caller,
		transactor: transactor,
		filterer:   filterer,
	}
}

// DeployContract deploys a contract onto the FiscoBcos blockchain and binds the
// deployment address with a Go wrapper.
func DeployContract(opts *TransactOpts, abi abi.ABI, bytecode []byte, backend ContractBackend, params ...interface{}) (common.Address, *types.Transaction, *BoundContract, error) {
	// Otherwise try to deploy the contract
	c := NewBoundContract(common.Address{}, abi, backend, backend, backend)

	input, err := abi.Pack("", params...)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	var nonce *big.Int
	for nonce == nil {
		b, _ := rlp.EncodeToBytes(uuid.NewUUID())
		nonce, _ = hexutil.DecodeBig(fmt.Sprintf("0x%x", md5.Sum(b[:8])))
	}
	opts.RandomId = nonce.Uint64()
	payLoad := append(bytecode, input...)
	rawTx := types.NewContractCreation(opts.RandomId, opts.BlockLimit, opts.Value,
		opts.GasLimit, opts.GasPrice, payLoad, opts.ChainId, opts.GroupId, nil)
	signedTx, err := opts.Signer(types.HomesteadSigner{}, opts.From, rawTx)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if opts.Context == nil {
		opts.Context = context.Background()
	}
	if err = backend.SendTransaction(opts.Context, signedTx); err != nil {
		return common.Address{}, nil, nil, err
	}
	time.Sleep(time.Second * 1)
	receipt, err := backend.TransactionReceipt(opts.Context, signedTx.Hash())
	if err != nil || receipt == nil {
		return common.Address{}, nil, nil, err
	}
	c = NewBoundContract(receipt.ContractAddress, abi, backend, backend, backend)
	return c.address, signedTx, c, nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (c *BoundContract) Call(opts *CallOpts, result interface{}, method string, params ...interface{}) error {
	// Don't crash on a lazy user
	if opts == nil {
		opts = new(CallOpts)
	}
	// Pack the input, call and unpack the results
	input, err := c.abi.Pack(method, params...)
	if err != nil {
		return err
	}
	var (
		msg    = fiscobcos.CallMsg{GroupId: opts.GroupId, Msg: fiscobcos.CallEthMsg{From: opts.From, To: &c.address, Data: input}}
		ctx    = ensureContext(opts.Context)
		code   []byte
		output []byte
	)
	output, err = c.caller.CallContract(ctx, msg, opts.BlockNumber)
	if err == nil && len(output) == 0 {
		// Make sure we have a contract to operate on, and bail out otherwise.
		if code, err = c.caller.CodeAt(ctx, c.address, opts.BlockNumber); err != nil {
			return err
		} else if len(code) == 0 {
			return ErrNoCode
		}
	}
	if err != nil {
		return err
	}
	return c.abi.Unpack(result, method, output)
}

func (c *BoundContract) ReadCall(result interface{}, method string, output []byte) error {
	return c.abi.Unpack(result, method, output)
}

// Transact invokes the (paid) contract method with params as input values.
func (c *BoundContract) Transact(opts *TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	// Otherwise pack up the parameters and invoke the contract
	input, err := c.abi.Pack(method, params...)
	if err != nil {
		return nil, err
	}
	// todo(rjl493456442) check the method is payable or not,
	// reject invalid transaction at the first place
	return c.transact(opts, &c.address, input)
}

// RawTransact initiates a transaction with the given raw calldata as the input.
// It's usually used to initiates transaction for invoking **Fallback** function.
func (c *BoundContract) RawTransact(opts *TransactOpts, calldata []byte) (*types.Transaction, error) {
	// todo(rjl493456442) check the method is payable or not,
	// reject invalid transaction at the first place
	return c.transact(opts, &c.address, calldata)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (c *BoundContract) Transfer(opts *TransactOpts) (*types.Transaction, error) {
	// todo(rjl493456442) check the payable fallback or receive is defined
	// or not, reject invalid transaction at the first place
	return c.transact(opts, &c.address, nil)
}

// transact executes an actual transaction invocation, first deriving any missing
// authorization fields, and then scheduling the transaction for execution.
func (c *BoundContract) transact(opts *TransactOpts, contract *common.Address, input []byte) (*types.Transaction, error) {
	var err error

	// Ensure a valid value field and resolve the account nonce
	value := opts.Value
	if value == nil {
		value = new(big.Int)
	}
	if opts.BlockLimit == 0 {
		return nil, errors.New("Block limit shoud be preseted.")
	}
	var nonce *big.Int
	for nonce == nil {
		b, _ := rlp.EncodeToBytes(uuid.NewUUID())
		nonce, _ = hexutil.DecodeBig(fmt.Sprintf("0x%x", md5.Sum(b[:8])))
	}
	opts.RandomId = nonce.Uint64()

	// Figure out the gas allowance and gas price values
	gasPrice := opts.GasPrice
	gasLimit := opts.GasLimit
	// Create the transaction, sign it and schedule it for execution
	var rawTx *types.Transaction
	rawTx = types.NewTransaction(opts.RandomId, opts.BlockLimit, c.address, value, gasLimit, gasPrice, input, opts.ChainId, opts.GroupId, nil)
	if opts.Signer == nil {
		return nil, errors.New("no signer to authorize the transaction with")
	}
	signedTx, err := opts.Signer(types.HomesteadSigner{}, opts.From, rawTx)
	if err != nil {
		return nil, err
	}
	if err := c.transactor.SendTransaction(opts.Context, signedTx); err != nil {
		return nil, err
	}
	return signedTx, nil
}

// FilterLogs filters contract logs for past blocks, returning the necessary
// channels to construct a strongly typed bound iterator on top of them.
func (c *BoundContract) FilterLogs(opts *FilterOpts, name string, query ...[]interface{}) (chan types.Log, event.Subscription, error) {
	// Don't crash on a lazy user
	if opts == nil {
		opts = new(FilterOpts)
	}
	// Append the event selector to the query parameters and construct the topic set
	query = append([][]interface{}{{c.abi.Events[name].ID}}, query...)

	topics, err := makeTopics(query...)
	if err != nil {
		return nil, nil, err
	}
	// Start the background filtering
	logs := make(chan types.Log, 128)

	config := fiscobcos.FilterQuery{
		Addresses: []common.Address{c.address},
		Topics:    topics,
		FromBlock: new(big.Int).SetUint64(opts.Start),
	}
	if opts.End != nil {
		config.ToBlock = new(big.Int).SetUint64(*opts.End)
	}
	/* TODO(karalabe): Replace the rest of the method below with this when supported
	sub, err := c.filterer.SubscribeFilterLogs(ensureContext(opts.Context), config, logs)
	*/
	buff, err := c.filterer.FilterLogs(ensureContext(opts.Context), config)
	if err != nil {
		return nil, nil, err
	}
	sub, err := event.NewSubscription(func(quit <-chan struct{}) error {
		for _, log := range buff {
			select {
			case logs <- log:
			case <-quit:
				return nil
			}
		}
		return nil
	}), nil

	if err != nil {
		return nil, nil, err
	}
	return logs, sub, nil
}

// WatchLogs filters subscribes to contract logs for future blocks, returning a
// subscription object that can be used to tear down the watcher.
func (c *BoundContract) WatchLogs(opts *WatchOpts, name string, query ...[]interface{}) (chan types.Log, event.Subscription, error) {
	// Don't crash on a lazy user
	if opts == nil {
		opts = new(WatchOpts)
	}
	// Append the event selector to the query parameters and construct the topic set
	query = append([][]interface{}{{c.abi.Events[name].ID}}, query...)

	topics, err := makeTopics(query...)
	if err != nil {
		return nil, nil, err
	}
	// Start the background filtering
	logs := make(chan types.Log, 128)

	config := fiscobcos.FilterQuery{
		Addresses: []common.Address{c.address},
		Topics:    topics,
	}
	if opts.Start != nil {
		config.FromBlock = new(big.Int).SetUint64(*opts.Start)
	}
	sub, err := c.filterer.SubscribeFilterLogs(ensureContext(opts.Context), config, logs)
	if err != nil {
		return nil, nil, err
	}
	return logs, sub, nil
}

// UnpackLog unpacks a retrieved log into the provided output structure.
func (c *BoundContract) UnpackLog(out interface{}, event string, log types.Log) error {
	if len(log.Data) > 0 {
		if err := c.abi.Unpack(out, event, log.Data); err != nil {
			return err
		}
	}
	var indexed abi.Arguments
	for _, arg := range c.abi.Events[event].Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	return parseTopics(out, indexed, log.Topics[1:])
}

// UnpackLogIntoMap unpacks a retrieved log into the provided map.
func (c *BoundContract) UnpackLogIntoMap(out map[string]interface{}, event string, log types.Log) error {
	if len(log.Data) > 0 {
		if err := c.abi.UnpackIntoMap(out, event, log.Data); err != nil {
			return err
		}
	}
	var indexed abi.Arguments
	for _, arg := range c.abi.Events[event].Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	return parseTopicsIntoMap(out, indexed, log.Topics[1:])
}

// ensureContext is a helper method to ensure a context is not nil, even if the
// user specified it as such.
func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.TODO()
	}
	return ctx
}
