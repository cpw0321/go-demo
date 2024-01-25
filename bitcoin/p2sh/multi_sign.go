package main

import (
	"encoding/hex"
	"go-demo/bitcoin/pkg/btcapi/mempool"
	"log"

	"github.com/btcsuite/btcd/wire"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

func main() {
	// 2-3多签
	destAddress := "mtzz8i68GchHyfWYso9j2PFjGCC3t9rC3H"
	var amount int64 = 1000

	netParams := &chaincfg.SigNetParams
	btcApiClient := mempool.NewClient(netParams)

	privateKeyHex01 := "7e3503d37b624a541815ce378f00f5ba01914708d9c8572ddc7057cc649659de"
	privateKeyByte01, _ := hex.DecodeString(privateKeyHex01)
	privateKey01, publicKey01 := btcec.PrivKeyFromBytes(privateKeyByte01)

	addressPubKey01, err := btcutil.NewAddressPubKey(publicKey01.SerializeCompressed(), &chaincfg.SigNetParams)
	if err != nil {
		log.Fatal("NewAddressPubKey err:", err)
	}

	privateKeyHex02 := "ddcc69506d5c7088078315386fac523d36d2d88c37ab0d56fa6e24501f1a4463"
	privateKeyByte02, _ := hex.DecodeString(privateKeyHex02)
	privateKey02, publicKey02 := btcec.PrivKeyFromBytes(privateKeyByte02)
	addressPubKey02, err := btcutil.NewAddressPubKey(publicKey02.SerializeCompressed(), &chaincfg.SigNetParams)
	if err != nil {
		log.Fatal("NewAddressPubKey err:", err)
	}

	privateKeyHex03 := "4ef5cb1fd5afd08ca9acbe5d077d89f1e095f67616a326d368851e57a92d1bab"
	privateKeyByte03, _ := hex.DecodeString(privateKeyHex03)
	_, publicKey03 := btcec.PrivKeyFromBytes(privateKeyByte03)
	addressPubKey03, err := btcutil.NewAddressPubKey(publicKey03.SerializeCompressed(), &chaincfg.SigNetParams)
	if err != nil {
		log.Fatal("NewAddressPubKey err:", err)
	}

	multiSigScript, err := txscript.MultiSigScript([]*btcutil.AddressPubKey{addressPubKey01, addressPubKey02, addressPubKey03}, 2)
	if err != nil {
		log.Fatal("MultiSigScript err:", err)
	}

	multiAddrScript, err := btcutil.NewAddressScriptHash(multiSigScript, &chaincfg.SigNetParams)
	if err != nil {
		log.Fatal("Failed to create address from script:", err)
	}

	log.Println("Multi-sig address:", multiAddrScript.EncodeAddress())

	multiAddr, err := btcutil.DecodeAddress(multiAddrScript.EncodeAddress(), &chaincfg.SigNetParams)
	if err != nil {
		log.Fatal("DecodeAddress failed:", err)
	}
	unspentList, err := btcApiClient.ListUnspent(multiAddr)
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
	log.Println("multiaddr amount:", totalSenderAmount)
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

	if changeAmount > 0 {
		// 添加找零地址作为交易输出
		changeScript, err := txscript.PayToAddrScript(multiAddr)
		if err != nil {
			log.Fatal(err)
		}
		tx.AddTxOut(wire.NewTxOut(changeAmount, changeScript))
	}

	for k, v := range tx.TxIn {
		sig01, err := txscript.RawTxInSignature(tx, k, multiSigScript, txscript.SigHashAll, privateKey01)
		if err != nil {
			log.Fatal("SignatureScript failed:", err)
		}
		sig02, err := txscript.RawTxInSignature(tx, k, multiSigScript, txscript.SigHashAll, privateKey02)
		if err != nil {
			log.Fatal("SignatureScript failed:", err)
		}
		signatureScript, err := txscript.NewScriptBuilder().AddOp(txscript.OP_FALSE).AddData(sig01).AddData(sig02).AddData(multiSigScript).Script()
		if err != nil {
			log.Fatal("NewScriptBuilder err:", err)
		}
		v.SignatureScript = signatureScript
	}

	txHash, err := btcApiClient.BroadcastTx(tx)
	if err != nil {
		log.Fatal("send tx failed:", err)
	}
	if err != nil {
		log.Fatal("send tx failed:", err)
	}
	log.Printf("tx_hash：%s\n", txHash.String())
	// https: //mempool.space/zh/signet/tx/38eaceccc8c8aaf8f1030026dd8a7fa3115752d12692a01cadab345cf5f302eb
}
