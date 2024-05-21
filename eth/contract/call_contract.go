package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// todo test
const (
	PRIVATE_KEY_TEST01 = "8805596b3d1291fa5264d84554bc92670971f0560cab7762df62b2cb904ba404"
	RPC_URL            = "https://haven-rpc.bsquared.network"
)

func main() {
	// 连接到以太坊网络
	client, err := ethclient.Dial(RPC_URL)
	if err != nil {
		log.Fatal("eth prc dial err: ", err)
	}
	// 合约地址
	contractAddressStr := "0x8449ea3a703D8C38F3F02D48348bA46237E9c240"
	contractAddress := common.HexToAddress(contractAddressStr)
	// 创建发送交易的账户
	privateKey, err := crypto.HexToECDSA(PRIVATE_KEY_TEST01)
	if err != nil {
		log.Fatal("get privateKey err:", err)
	}
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("get public key to ECDSA err: ", err)
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	// 构建未签名的交易
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		log.Fatal("get nonce err:", err)
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal("get gasPrice err:", err)
	}

	abiByte, _ := os.ReadFile("/Users/zhigui/code/gopath/src/cpw/go-demo/eth/contract/abi.json")
	// 构造方法调用数据
	contractAbi, err := abi.JSON(bytes.NewReader(abiByte))
	if err != nil {
		fmt.Println("abi.JSON error ,", err)
		return
	}
	// 将data设置为Deposit函数的ABI编码结果
	// 转30000
	value := [32]byte{}
	copy(value[:], "0669270c2e09ae4f35224e13f63601348573c1a2b1c69d3cebd6beec80b65098")

	//amount, _ := new(big.Int).SetString("100000000", 10)
	tokenIds := []string{"30"} // 替换为你要传递的tokenIds
	// 将tokenIds转换为abi.Value
	tokenIdsSlice := make([]*big.Int, len(tokenIds))
	for i, str := range tokenIds {
		uintVal, err := strconv.ParseUint(str, 10, 64)
		if err != nil {

		}
		tokenIdsSlice[i] = big.NewInt(int64(uintVal))
	}
	toAddress := "0x756A6aa43547fA8cCF02ab417E6c4c4747137346"
	data, err := contractAbi.Pack("release", common.HexToAddress(toAddress), tokenIdsSlice)
	if err != nil {
		log.Fatal("contractAbi.Pack error:", err)
	}

	gasLimit, err := client.EstimateGas(context.Background(), ethereum.CallMsg{
		From:     fromAddress,
		To:       &contractAddress,
		Data:     data,
		Value:    big.NewInt(0),
		GasPrice: gasPrice, // 替换为你要设置的 gas price
	})
	if err != nil {
		log.Fatal("get gasLimit err: ", err)
	}

	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &contractAddress,
		Value:    big.NewInt(0),
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     data,
	})

	// 对交易进行签名
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		log.Fatal("get chainID err: ", err)
	}
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		log.Fatal("get SignTx is err: ", err)
	}

	// 发送交易到以太坊网络
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		log.Fatal("send tx err: ", err)
	}
	// 等待交易确认
	receipt, err := bind.WaitMined(context.Background(), client, signedTx)
	if err != nil {
		log.Fatal("tx WaitMined is err: ", err)
	}
	log.Println("Status:", receipt.Status)
	log.Println("TxHash:", receipt.TxHash)
	log.Println("ContractAddress:", receipt.ContractAddress)
}
