package keeper_test

import (
	"math"
	"sync"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/sei-protocol/sei-chain/testutil/keeper"
	evmkeeper "github.com/sei-protocol/sei-chain/x/evm/keeper"
	"github.com/sei-protocol/sei-chain/x/evm/types"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"
	tmtypes "github.com/tendermint/tendermint/types"
)

func TestGetChainID(t *testing.T) {
	k, ctx := keeper.MockEVMKeeper()
	require.Equal(t, types.DefaultChainID.Int64(), k.ChainID(ctx).Int64())
}

func TestGetVMBlockContext(t *testing.T) {
	k, ctx := keeper.MockEVMKeeper()
	moduleAddr := k.AccountKeeper().GetModuleAddress(authtypes.FeeCollectorName)
	evmAddr, _ := k.GetEVMAddress(ctx, moduleAddr)
	k.DeleteAddressMapping(ctx, moduleAddr, evmAddr)
	_, err := k.GetVMBlockContext(ctx, 0)
	require.NotNil(t, err)
}

func TestGetHashFn(t *testing.T) {
	k, ctx := keeper.MockEVMKeeper()
	f := k.GetHashFn(ctx)
	require.Equal(t, common.Hash{}, f(math.MaxInt64+1))
	require.Equal(t, common.BytesToHash(ctx.HeaderHash()), f(uint64(ctx.BlockHeight())))
	require.Equal(t, common.Hash{}, f(uint64(ctx.BlockHeight())+1))
	require.Equal(t, common.Hash{}, f(uint64(ctx.BlockHeight())-1))
}

func TestKeeper_CalculateNextNonce(t *testing.T) {
	address1 := common.BytesToAddress([]byte("addr1"))
	key1 := tmtypes.TxKey(rand.NewRand().Bytes(32))
	key2 := tmtypes.TxKey(rand.NewRand().Bytes(32))
	tests := []struct {
		name          string
		address       common.Address
		pending       bool
		setup         func(ctx sdk.Context, k *evmkeeper.Keeper)
		expectedNonce uint64
	}{
		{
			name:          "latest block, no latest stored",
			address:       address1,
			pending:       false,
			expectedNonce: 0,
		},
		{
			name:    "latest block, latest stored",
			address: address1,
			pending: false,
			setup: func(ctx sdk.Context, k *evmkeeper.Keeper) {
				k.SetNonce(ctx, address1, 50)
			},
			expectedNonce: 50,
		},
		{
			name:    "latest block, latest stored with pending nonces",
			address: address1,
			pending: false,
			setup: func(ctx sdk.Context, k *evmkeeper.Keeper) {
				k.SetNonce(ctx, address1, 50)
				// because pending:false, these won't matter
				k.AddPendingNonce(key1, address1, 50)
				k.AddPendingNonce(key2, address1, 51)
			},
			expectedNonce: 50,
		},
		{
			name:    "pending block, nonce should follow the last pending",
			address: address1,
			pending: true,
			setup: func(ctx sdk.Context, k *evmkeeper.Keeper) {
				k.SetNonce(ctx, address1, 50)
				k.AddPendingNonce(key1, address1, 50)
				k.AddPendingNonce(key2, address1, 51)
			},
			expectedNonce: 52,
		},
		{
			name:    "pending block, nonce should be the value of hole",
			address: address1,
			pending: true,
			setup: func(ctx sdk.Context, k *evmkeeper.Keeper) {
				k.SetNonce(ctx, address1, 50)
				k.AddPendingNonce(key1, address1, 50)
				// missing 51, so nonce = 51
				k.AddPendingNonce(key2, address1, 52)
			},
			expectedNonce: 51,
		},
		{
			name:    "pending block, completed nonces should also be skipped",
			address: address1,
			pending: true,
			setup: func(ctx sdk.Context, k *evmkeeper.Keeper) {
				k.SetNonce(ctx, address1, 50)
				k.AddPendingNonce(key1, address1, 50)
				k.AddPendingNonce(key2, address1, 51)
				k.CompletePendingNonce(key1)
				k.CompletePendingNonce(key2)
			},
			expectedNonce: 52,
		},
		{
			name:    "pending block, hole created by expiration",
			address: address1,
			pending: true,
			setup: func(ctx sdk.Context, k *evmkeeper.Keeper) {
				k.SetNonce(ctx, address1, 50)
				k.AddPendingNonce(key1, address1, 50)
				k.AddPendingNonce(key2, address1, 51)
				k.ExpirePendingNonce(key1)
			},
			expectedNonce: 50,
		},
		{
			name:    "pending block, skipped nonces all in pending",
			address: address1,
			pending: true,
			setup: func(ctx sdk.Context, k *evmkeeper.Keeper) {
				// next expected for latest is 50, but 51,52 were sent
				k.SetNonce(ctx, address1, 50)
				k.AddPendingNonce(key1, address1, 51)
				k.AddPendingNonce(key2, address1, 52)
			},
			expectedNonce: 50,
		},
		{
			name:    "try 1000 nonces concurrently",
			address: address1,
			pending: true,
			setup: func(ctx sdk.Context, k *evmkeeper.Keeper) {
				// next expected for latest is 50, but 51,52 were sent
				k.SetNonce(ctx, address1, 50)
				wg := sync.WaitGroup{}
				for i := 50; i < 1000; i++ {
					wg.Add(1)
					go func(nonce int) {
						defer wg.Done()
						key := tmtypes.TxKey(rand.NewRand().Bytes(32))
						// call this just to exercise locks
						k.CalculateNextNonce(ctx, address1, true)
						k.AddPendingNonce(key, address1, uint64(nonce))
					}(i)
				}
				wg.Wait()
			},
			expectedNonce: 1000,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			k, ctx := keeper.MockEVMKeeper()
			if test.setup != nil {
				test.setup(ctx, k)
			}
			next := k.CalculateNextNonce(ctx, test.address, test.pending)
			require.Equal(t, test.expectedNonce, next)
		})
	}
}
