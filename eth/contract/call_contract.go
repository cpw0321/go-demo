package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"os"

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
	PRIVATE_KEY_TEST01 = "8833e01ffbb75785ed8bc0fb0f1154aaa162e03deaad61f003d98c2c3cd8aee6"
	RPC_URL            = "https://yolo-winter-breeze.ethereum-goerli.quiknode.pro/"
)

func main() {
	// 连接到以太坊网络
	client, err := ethclient.Dial(RPC_URL)
	if err != nil {
		log.Fatal("eth prc dial err: ", err)
	}
	// 合约地址
	contractAddressStr := ""
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

	abiByte, _ := os.ReadFile("./abi.json")
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

	amount, _ := new(big.Int).SetString("100000000", 10)
	toAddress := "0xa1CEF929b4B9bc780790dfD87430d119AeD61DD0"
	data, err := contractAbi.Pack("deposit", value, common.HexToAddress(toAddress), amount)
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
