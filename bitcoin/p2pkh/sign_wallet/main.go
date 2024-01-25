package main

import (
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"log"
)

func main() {
	// 连接到比特币节点的RPC服务器
	connCfg := &rpcclient.ConnConfig{
		Host:         "127.0.0.1:8332",
		User:         "test",
		Pass:         "123456",
		HTTPPostMode: true,
		DisableTLS:   true, // 如果比特币节点未启用SSL/TLS，需要设置为true
	}

	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		log.Println("Failed to connect to the Bitcoin node:", err)
		return
	}
	defer client.Shutdown()

	sourceAddress := "tb1qr2ssscefkjeehv5kl0alhwj976v6cpxqlskn7n"
	destAddress := "tb1qtzqjsdwskw4r52mzek7jmd609rnmtlwf4ev79g"
	var amount int64 = 1000

	sourceAddr, err := btcutil.DecodeAddress(sourceAddress, &chaincfg.SigNetParams)
	if err != nil {
		log.Fatal(err)
	}

	// list钱包中的账户
	unspentTxs, err := client.ListUnspentMinMaxAddresses(1, 9999999, []btcutil.Address{sourceAddr})
	if err != nil {
		log.Fatal(err)
	}
	if len(unspentTxs) == 0 {
		log.Fatal("no unspentTxs")
	}

	// 创建比特币交易输入和输出
	totalSenderAmount := btcutil.Amount(0)
	tx := wire.NewMsgTx(wire.TxVersion)
	for _, v := range unspentTxs {
		inTxid, _ := chainhash.NewHashFromStr(v.TxID)
		outpoint := wire.NewOutPoint(inTxid, v.Vout)
		txIn := wire.NewTxIn(outpoint, nil, nil)
		tx.AddTxIn(txIn)
		totalSenderAmount += btcutil.Amount(v.Amount * 1e8)
		// 钱够了就不继续计算了
		if int64(totalSenderAmount) > amount {
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
	changeAmount := int64(totalSenderAmount) - amount - fee // 减去交易费用，转账2000

	// 添加目标地址作为交易输出
	tx.AddTxOut(wire.NewTxOut(amount, destinationScript))

	if changeAmount > 0 {
		// 添加找零地址作为交易输出
		changeScript, err := txscript.PayToAddrScript(sourceAddr)
		if err != nil {
			log.Fatal(err)
		}

		tx.AddTxOut(wire.NewTxOut(changeAmount, changeScript))
	}

	signedTx, complete, err := client.SignRawTransactionWithWallet(tx)
	if err != nil {
		log.Fatal("sign tx failed", err)
	}
	if !complete {
		log.Fatal("交易未完成签名")
	}
	// 发送交易
	txHash, err := client.SendRawTransaction(signedTx, true)
	if err != nil {
		log.Fatal("send tx failed:", err)
	}
	// https://mempool.space/zh/signet/tx/7ce384aa43f95e7ee07cace97d053a1aca8316d81d6144b5ee505e6bedd2bd95
	log.Printf("转账成功！交易哈希：%s\n", txHash.String())
}
