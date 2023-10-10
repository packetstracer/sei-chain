package bank

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	pcommon "github.com/sei-protocol/sei-chain/precompiles/common"
	"github.com/tendermint/tendermint/libs/log"
)

const (
	SendMethod    = "send"
	BalanceMethod = "balance"
)

const (
	BankAddress = "0x0000000000000000000000000000000000001001"
)

var _ vm.PrecompiledContract = &Precompile{}

// Embed abi json file to the executable binary. Needed when importing as dependency.
//
//go:embed abi.json
var f embed.FS

type Precompile struct {
	pcommon.Precompile
	bankKeeper pcommon.BankKeeper
	evmKeeper  pcommon.EVMKeeper
	address    common.Address
}

// RequiredGas returns the required bare minimum gas to execute the precompile.
func (p Precompile) RequiredGas(input []byte) uint64 {
	methodID := input[:4]

	method, err := p.ABI.MethodById(methodID)
	if err != nil {
		// This should never happen since this method is going to fail during Run
		return 0
	}

	return p.Precompile.RequiredGas(input, p.IsTransaction(method.Name))
}

func NewPrecompile(bankKeeper pcommon.BankKeeper, evmKeeper pcommon.EVMKeeper) (*Precompile, error) {
	abiBz, err := f.ReadFile("abi.json")
	if err != nil {
		return nil, fmt.Errorf("error loading the staking ABI %s", err)
	}

	newAbi, err := abi.JSON(bytes.NewReader(abiBz))
	if err != nil {
		return nil, err
	}

	return &Precompile{
		Precompile: pcommon.Precompile{ABI: newAbi},
		bankKeeper: bankKeeper,
		evmKeeper:  evmKeeper,
		address:    common.HexToAddress(BankAddress),
	}, nil
}

func (p Precompile) Address() common.Address {
	return p.address
}

func (p Precompile) Run(evm *vm.EVM, input []byte) (bz []byte, err error) {
	ctx, method, args, err := p.Prepare(evm, input)
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case SendMethod:
		return p.send(ctx, method, args)
	case BalanceMethod:
		return p.balance(ctx, method, args)
	}
	return
}

func (p Precompile) send(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	if len(args) != 4 {
		return nil, errors.New("send requires exactly 4 arguments")
	}
	denom, ok := args[2].(string)
	if !ok || denom == "" {
		return nil, errors.New("invalid denom")
	}
	amount, ok := args[3].(*big.Int)
	if !ok {
		return nil, errors.New("invalid amount")
	}
	if amount.Cmp(big.NewInt(0)) == 0 {
		return method.Outputs.Pack(true)
	}
	// TODO: it's possible to extend evm module's balance to handle non-usei tokens as well
	senderSeiAddr, err := p.accAddressFromArg(ctx, args[0])
	if err != nil {
		return nil, err
	}
	receiverSeiAddr, err := p.accAddressFromArg(ctx, args[1])
	if err != nil {
		return nil, err
	}
	if err := p.bankKeeper.SendCoins(ctx, senderSeiAddr, receiverSeiAddr, sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromBigInt(amount)))); err != nil {
		return nil, err
	}
	return method.Outputs.Pack(true)
}

func (p Precompile) balance(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	addr, err := p.accAddressFromArg(ctx, args[0])
	if err != nil {
		return nil, err
	}
	denom, ok := args[1].(string)
	if !ok || denom == "" {
		return nil, errors.New("invalid denom")
	}
	return method.Outputs.Pack(p.bankKeeper.GetBalance(ctx, addr, denom).Amount.BigInt())
}

func (p Precompile) accAddressFromArg(ctx sdk.Context, arg interface{}) (sdk.AccAddress, error) {
	addr, ok := arg.(common.Address)
	if !ok || addr == (common.Address{}) {
		return nil, errors.New("invalid addr")
	}
	seiAddr, found := p.evmKeeper.GetSeiAddress(ctx, addr)
	if !found {
		return nil, errors.New("address does not have association")
	}
	return seiAddr, nil
}

func (Precompile) IsTransaction(method string) bool {
	switch method {
	case SendMethod:
		return true
	default:
		return false
	}
}

func (p Precompile) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("precompile", "bank")
}
