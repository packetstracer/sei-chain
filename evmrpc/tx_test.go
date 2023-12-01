package evmrpc_test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/triedb/hashdb"
	"github.com/sei-protocol/sei-chain/evmrpc/uniswapfactory"
	"github.com/sei-protocol/sei-chain/evmrpc/uniswappool"
	testkeeper "github.com/sei-protocol/sei-chain/testutil/keeper"
	"github.com/sei-protocol/sei-chain/x/evm/types"
	"github.com/stretchr/testify/require"
)

func TestGetTxReceipt(t *testing.T) {
	body := "{\"jsonrpc\": \"2.0\",\"method\": \"eth_getTransactionReceipt\",\"params\":[\"0x78b0bd7fe9ccc8ae8a61eae9315586cf2a406dacf129313e6c5769db7cd14372\"],\"id\":\"test\"}"
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s:%d", TestAddr, TestPort), strings.NewReader(body))
	require.Nil(t, err)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	resBody, err := io.ReadAll(res.Body)
	require.Nil(t, err)
	resObj := map[string]interface{}{}
	require.Nil(t, json.Unmarshal(resBody, &resObj))
	resObj = resObj["result"].(map[string]interface{})
	require.Equal(t, "0x3030303030303030303030303030303030303030303030303030303030303031", resObj["blockHash"].(string))
	require.Equal(t, "0x8", resObj["blockNumber"].(string))
	require.Equal(t, "0x1234567890123456789012345678901234567890", resObj["contractAddress"].(string))
	require.Equal(t, "0x7b", resObj["cumulativeGasUsed"].(string))
	require.Equal(t, "0xa", resObj["effectiveGasPrice"].(string))
	require.Equal(t, "0x1234567890123456789012345678901234567890", resObj["from"].(string))
	require.Equal(t, "0x37", resObj["gasUsed"].(string))
	logs := resObj["logs"].([]interface{})
	require.Equal(t, 1, len(logs))
	log := logs[0].(map[string]interface{})
	require.Equal(t, "0x1111111111111111111111111111111111111111", log["address"].(string))
	topics := log["topics"].([]interface{})
	require.Equal(t, 2, len(topics))
	require.Equal(t, "0x1111111111111111111111111111111111111111111111111111111111111111", topics[0].(string))
	require.Equal(t, "0x1111111111111111111111111111111111111111111111111111111111111112", topics[1].(string))
	require.Equal(t, "0x78797a", log["data"].(string))
	require.Equal(t, "0x8", log["blockNumber"].(string))
	require.Equal(t, "0x1111111111111111111111111111111111111111111111111111111111111113", log["transactionHash"].(string))
	require.Equal(t, "0x2", log["transactionIndex"].(string))
	require.Equal(t, "0x1111111111111111111111111111111111111111111111111111111111111111", log["blockHash"].(string))
	require.Equal(t, "0x1", log["logIndex"].(string))
	require.True(t, log["removed"].(bool))
	require.Equal(t, "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000", resObj["logsBloom"].(string))
	require.Equal(t, "0x0", resObj["status"].(string))
	require.Equal(t, "0x1234567890123456789012345678901234567890", resObj["to"].(string))
	require.Equal(t, "0x0123456789012345678902345678901234567890123456789012345678901234", resObj["transactionHash"].(string))
	require.Equal(t, "0x0", resObj["transactionIndex"].(string))
	require.Equal(t, "0x1", resObj["type"].(string))

	req, err = http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s:%d", TestAddr, TestBadPort), strings.NewReader(body))
	require.Nil(t, err)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	require.Nil(t, err)
	resBody, err = io.ReadAll(res.Body)
	require.Nil(t, err)
	resObj = map[string]interface{}{}
	require.Nil(t, json.Unmarshal(resBody, &resObj))
	resObj = resObj["error"].(map[string]interface{})
	require.Equal(t, float64(-32000), resObj["code"].(float64))
	require.Equal(t, "error block", resObj["message"].(string))

	resObj = sendRequestGood(t, "getTransactionReceipt", common.HexToHash("0x3030303030303030303030303030303030303030303030303030303030303031"))
	require.Nil(t, resObj["result"])
}

func TestGetTransaction(t *testing.T) {
	bodyByBlockNumberAndIndex := "{\"jsonrpc\": \"2.0\",\"method\": \"eth_getTransactionByBlockNumberAndIndex\",\"params\":[\"0x8\",\"0x0\"],\"id\":\"test\"}"
	bodyByBlockHashAndIndex := "{\"jsonrpc\": \"2.0\",\"method\": \"eth_getTransactionByBlockHashAndIndex\",\"params\":[\"0x0000000000000000000000000000000000000000000000000000000000000001\",\"0x0\"],\"id\":\"test\"}"
	bodyByHash := "{\"jsonrpc\": \"2.0\",\"method\": \"eth_getTransactionByHash\",\"params\":[\"0x78b0bd7fe9ccc8ae8a61eae9315586cf2a406dacf129313e6c5769db7cd14372\"],\"id\":\"test\"}"
	for _, body := range []string{bodyByBlockNumberAndIndex, bodyByBlockHashAndIndex, bodyByHash} {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s:%d", TestAddr, TestPort), strings.NewReader(body))
		require.Nil(t, err)
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		resBody, err := io.ReadAll(res.Body)
		require.Nil(t, err)
		resObj := map[string]interface{}{}
		require.Nil(t, json.Unmarshal(resBody, &resObj))
		resObj = resObj["result"].(map[string]interface{})
		require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000001", resObj["blockHash"].(string))
		require.Equal(t, "0x8", resObj["blockNumber"].(string))
		require.Equal(t, "0x1234567890123456789012345678901234567890", resObj["from"].(string))
		require.Equal(t, "0x3e8", resObj["gas"].(string))
		require.Equal(t, "0xa", resObj["gasPrice"].(string))
		require.Equal(t, "0xa", resObj["maxFeePerGas"].(string))
		require.Equal(t, "0x0", resObj["maxPriorityFeePerGas"].(string))
		require.Equal(t, "0x78b0bd7fe9ccc8ae8a61eae9315586cf2a406dacf129313e6c5769db7cd14372", resObj["hash"].(string))
		require.Equal(t, "0x616263", resObj["input"].(string))
		require.Equal(t, "0x1", resObj["nonce"].(string))
		require.Equal(t, "0x0000000000000000000000000000000000010203", resObj["to"].(string))
		require.Equal(t, "0x0", resObj["transactionIndex"].(string))
		require.Equal(t, "0x3e8", resObj["value"].(string))
		require.Equal(t, "0x0", resObj["type"].(string))
		require.Equal(t, 0, len(resObj["accessList"].([]interface{})))
		require.Equal(t, "0x1", resObj["chainId"].(string))
		require.Equal(t, "0x1", resObj["v"].(string))
		require.Equal(t, "0x34125c09c6b1a57f5f571a242572129057b22612dd56ee3519c4f68bece0db03", resObj["r"].(string))
		require.Equal(t, "0x3f4fe6f2512219bac6f9b4e4be1aa11d3ef79c5c2f1000ef6fa37389de0ff523", resObj["s"].(string))
		require.Equal(t, "0x1", resObj["yParity"].(string))
	}

	for _, body := range []string{bodyByBlockNumberAndIndex, bodyByBlockHashAndIndex, bodyByHash} {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s:%d", TestAddr, TestBadPort), strings.NewReader(body))
		require.Nil(t, err)
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		resBody, err := io.ReadAll(res.Body)
		require.Nil(t, err)
		resObj := map[string]interface{}{}
		require.Nil(t, json.Unmarshal(resBody, &resObj))
		require.Nil(t, resObj["result"])
	}
}

func TestGetPendingTransactionByHash(t *testing.T) {
	resObj := sendRequestGood(t, "getTransactionByHash", "0x74452c2b9b4482f34eba843725cc99625bc89fe55d9a67d4a506e584ba1f334b")
	result := resObj["result"].(map[string]interface{})
	require.Equal(t, "0x2", result["nonce"])
}

func TestGetTransactionCount(t *testing.T) {
	// happy path
	bodyByNumber := "{\"jsonrpc\": \"2.0\",\"method\": \"eth_getTransactionCount\",\"params\":[\"0x1234567890123456789012345678901234567890\",\"0x8\"],\"id\":\"test\"}"
	bodyByHash := "{\"jsonrpc\": \"2.0\",\"method\": \"eth_getTransactionCount\",\"params\":[\"0x1234567890123456789012345678901234567890\",\"0x3030303030303030303030303030303030303030303030303030303030303031\"],\"id\":\"test\"}"

	for _, body := range []string{bodyByNumber, bodyByHash} {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s:%d", TestAddr, TestPort), strings.NewReader(body))
		require.Nil(t, err)
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		resBody, err := io.ReadAll(res.Body)
		require.Nil(t, err)
		resObj := map[string]interface{}{}
		require.Nil(t, json.Unmarshal(resBody, &resObj))
		count := resObj["result"].(string)
		require.Equal(t, "0x1", count)
	}

	// address that doesn't have tx
	strangerBody := "{\"jsonrpc\": \"2.0\",\"method\": \"eth_getTransactionCount\",\"params\":[\"0x0123456789012345678902345678901234567891\",\"0x8\"],\"id\":\"test\"}"
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s:%d", TestAddr, TestPort), strings.NewReader(strangerBody))
	require.Nil(t, err)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	resBody, err := io.ReadAll(res.Body)
	require.Nil(t, err)
	resObj := map[string]interface{}{}
	require.Nil(t, json.Unmarshal(resBody, &resObj))
	count := resObj["result"].(string)
	require.Equal(t, "0x0", count) // no tx

	// error cases
	earliestBodyToBadPort := "{\"jsonrpc\": \"2.0\",\"method\": \"eth_getTransactionCount\",\"params\":[\"0x1234567890123456789012345678901234567890\",\"earliest\"],\"id\":\"test\"}"
	for body, errStr := range map[string]string{
		earliestBodyToBadPort: "error genesis",
	} {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s:%d", TestAddr, TestBadPort), strings.NewReader(body))
		require.Nil(t, err)
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		require.Nil(t, err)
		resBody, err := io.ReadAll(res.Body)
		require.Nil(t, err)
		resObj := map[string]interface{}{}
		require.Nil(t, json.Unmarshal(resBody, &resObj))
		errMap := resObj["error"].(map[string]interface{})
		errMsg := errMap["message"].(string)
		require.Equal(t, errStr, errMsg)
	}
}

func TestGetTransactionError(t *testing.T) {
	h := common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
	EVMKeeper.SetReceipt(Ctx, h, &types.Receipt{VmError: "test error"})
	resObj := sendRequestGood(t, "getTransactionErrorByHash", "0x1111111111111111111111111111111111111111111111111111111111111111")
	require.Equal(t, "test error", resObj["result"])
}

func TestDuration(t *testing.T) {
	chainConfig := params.AllDevChainProtocolChanges
	_, author := testkeeper.MockAddressPair()
	privKey := testkeeper.MockPrivateKey()
	_, sender := testkeeper.PrivateKeyToAddresses(privKey)
	testPrivHex := hex.EncodeToString(privKey.Bytes())
	key, _ := crypto.HexToECDSA(testPrivHex)
	gp := core.GasPool(10000000000)
	initialGas := uint64(1000000000)
	genesisConfig := new(core.Genesis)
	genesisConfig.GasLimit = initialGas
	genesisConfig.Config = params.AllDevChainProtocolChanges
	genesisConfig.Coinbase = author
	genesisConfig.Alloc = core.GenesisAlloc{
		author: core.GenesisAccount{
			Balance: big.NewInt(math.MaxInt64),
		},
		sender: core.GenesisAccount{
			Balance: big.NewInt(math.MaxInt64),
		},
	}
	header := ethtypes.Header{Number: big.NewInt(10), Difficulty: big.NewInt(0), BaseFee: big.NewInt(1)}

	db := rawdb.NewMemoryDatabase()
	triedb := trie.NewDatabase(db, &trie.Config{
		Preimages: false,
		HashDB:    hashdb.Defaults,
	})
	defer triedb.Close()
	genesis := genesisConfig.MustCommit(db, triedb)
	sdb := state.NewDatabaseWithNodeDB(db, triedb)
	statedb, _ := state.New(genesis.Root(), sdb, nil)
	usedGas := uint64(0)
	vmConfig := vm.Config{NoBaseFee: true}
	code, err := os.ReadFile("../../v3-core/contracts/bin/factory/UniswapV3Factory.bin")
	require.Nil(t, err)
	bz, err := hex.DecodeString(string(code))
	require.Nil(t, err)
	tx := ethtypes.LegacyTx{
		GasPrice: big.NewInt(100),
		Gas:      20000000,
		To:       nil,
		Value:    big.NewInt(0),
		Data:     bz,
		Nonce:    0,
	}
	blockNum := big.NewInt(10)
	signer := ethtypes.MakeSigner(chainConfig, blockNum, uint64(time.Now().Unix()))
	signedTx, err := ethtypes.SignTx(ethtypes.NewTx(&tx), signer, key)
	receipt, err := core.ApplyTransaction(
		chainConfig,
		nil,
		&author,
		&gp,
		statedb,
		&header,
		signedTx,
		&usedGas,
		vmConfig,
	)
	unifacAddr := receipt.ContractAddress
	require.Nil(t, err)

	code, err = os.ReadFile("../contracts/TokenA.bin")
	require.Nil(t, err)
	bz, err = hex.DecodeString(string(code))
	require.Nil(t, err)
	tx = ethtypes.LegacyTx{
		GasPrice: big.NewInt(100),
		Gas:      50000000,
		To:       nil,
		Value:    big.NewInt(0),
		Data:     bz,
		Nonce:    1,
	}
	signedTx, err = ethtypes.SignTx(ethtypes.NewTx(&tx), signer, key)
	receipt, err = core.ApplyTransaction(
		chainConfig,
		nil,
		&author,
		&gp,
		statedb,
		&header,
		signedTx,
		&usedGas,
		vmConfig,
	)
	require.Nil(t, err)
	TokenA := receipt.ContractAddress

	code, err = os.ReadFile("../contracts/TokenB.bin")
	require.Nil(t, err)
	bz, err = hex.DecodeString(string(code))
	require.Nil(t, err)
	tx = ethtypes.LegacyTx{
		GasPrice: big.NewInt(100),
		Gas:      50000000,
		To:       nil,
		Value:    big.NewInt(0),
		Data:     bz,
		Nonce:    2,
	}
	signedTx, err = ethtypes.SignTx(ethtypes.NewTx(&tx), signer, key)
	receipt, err = core.ApplyTransaction(
		chainConfig,
		nil,
		&author,
		&gp,
		statedb,
		&header,
		signedTx,
		&usedGas,
		vmConfig,
	)
	require.Nil(t, err)
	TokenB := receipt.ContractAddress
	fmt.Println(unifacAddr)
	fmt.Println(TokenA)
	fmt.Println(TokenB)

	facAbi, err := uniswapfactory.UniswapfactoryMetaData.GetAbi()
	require.Nil(t, err)
	bz, err = facAbi.Pack("createPool", TokenA, TokenB, big.NewInt(500))
	require.Nil(t, err)
	tx = ethtypes.LegacyTx{
		GasPrice: big.NewInt(100),
		Gas:      50000000,
		To:       &unifacAddr,
		Value:    big.NewInt(0),
		Data:     bz,
		Nonce:    3,
	}
	signedTx, err = ethtypes.SignTx(ethtypes.NewTx(&tx), signer, key)
	receipt, err = core.ApplyTransaction(
		chainConfig,
		nil,
		&author,
		&gp,
		statedb,
		&header,
		signedTx,
		&usedGas,
		vmConfig,
	)
	poolAddr := common.BytesToAddress(receipt.ReturnData)

	poolAbi, err := uniswappool.UniswappoolMetaData.GetAbi()
	require.Nil(t, err)
	bz, err = poolAbi.Pack("swap", author, false, big.NewInt(10), big.NewInt(1), []byte{})
	require.Nil(t, err)
	tx = ethtypes.LegacyTx{
		GasPrice: big.NewInt(100),
		Gas:      50000000,
		To:       &poolAddr,
		Value:    big.NewInt(0),
		Data:     bz,
		Nonce:    4,
	}
	signedTx, err = ethtypes.SignTx(ethtypes.NewTx(&tx), signer, key)
	start := time.Now()
	receipt, err = core.ApplyTransaction(
		chainConfig,
		nil,
		&author,
		&gp,
		statedb,
		&header,
		signedTx,
		&usedGas,
		vmConfig,
	)
	end := time.Now()
	fmt.Println(end.UnixNano() - start.UnixNano())
	require.NotNil(t, err)
}
