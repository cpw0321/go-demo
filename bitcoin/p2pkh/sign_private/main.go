package main

import (
	"encoding/hex"
	"go-demo/bitcoin/pkg/btcapi/mempool"
	"log"

	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"

	"github.com/btcsuite/btcd/chaincfg"
)

func main() {
	destAddress := "mtzz8i68GchHyfWYso9j2PFjGCC3t9rC3H"
	var amount int64 = 1000

	netParams := &chaincfg.SigNetParams
	btcApiClient := mempool.NewClient(netParams)

	privateKeyHex01 := "7e3503d37b624a541815ce378f00f5ba01914708d9c8572ddc7057cc649659de"
	privateKeyByte, _ := hex.DecodeString(privateKeyHex01)
	privateKeyBytes01 := privateKeyByte // 用你自己的私钥替换这里的字节
	privateKey01, publicKey01 := btcec.PrivKeyFromBytes(privateKeyBytes01)

	sourceAddrPubKey, err := btcutil.NewAddressPubKey(publicKey01.SerializeCompressed(), &chaincfg.SigNetParams)
	if err != nil {
		log.Fatal("NewAddressPubKey:", err)
	}
	log.Println("sourceAddress:", sourceAddrPubKey.EncodeAddress())
	sourceAddr, err := btcutil.DecodeAddress(sourceAddrPubKey.EncodeAddress(), &chaincfg.SigNetParams)
	if err != nil {
		log.Fatal(err)
	}
	unspentList, err := btcApiClient.ListUnspent(sourceAddr)
	if err != nil {
		log.Fatal("ListUnspent failed:", err)
	}

	if len(unspentList) == 0 {
		log.Fatal("no unspentTxs")
	}
	// 创建比特币交易输入和输出
	var totalSenderAmount int64
	tx := wire.NewMsgTx(wire.TxVersion)
	for _, v := range unspentList {
		txIn := wire.NewTxIn(v.Outpoint, nil, nil)
		tx.AddTxIn(txIn)
		totalSenderAmount += v.Output.Value
		// 钱够了就不继续计算了
		if totalSenderAmount > amount {
			break
		}
	}

	// 获取目标地址的比特币脚本
	destAddr, err := btcutil.DecodeAddress(destAddress, &chaincfg.SigNetParams)
	if err != nil {
		log.Fatal(err)
	}
	destinationScript, err := txscript.PayToAddrScript(destAddr)
	if err != nil {
		log.Fatal(err)
	}

	// 计算总金额和交易费用
	var fee int64 = 1000
	changeAmount := totalSenderAmount - amount - fee // 减去交易费用，转账2000

	// 添加目标地址作为交易输出
	tx.AddTxOut(wire.NewTxOut(amount, destinationScript))

	changeScript, err := txscript.PayToAddrScript(sourceAddr)
	if err != nil {
		log.Fatal(err)
	}
	if changeAmount > 0 {
		// 添加找零地址作为交易输出
		tx.AddTxOut(wire.NewTxOut(changeAmount, changeScript))
	}

	for k, v := range tx.TxIn {
		signScriptTx, err := txscript.SignatureScript(tx, k, changeScript, txscript.SigHashAll, privateKey01, true)
		if err != nil {
			log.Fatal("SignatureScript failed:", err)
		}
		v.SignatureScript = signScriptTx
	}

	txHash, err := btcApiClient.BroadcastTx(tx)
	if err != nil {
		log.Fatal("send tx failed:", err)
	}
	if err != nil {
		log.Fatal("send tx failed:", err)
	}
	log.Printf("tx_hash：%s\n", txHash.String())
}
