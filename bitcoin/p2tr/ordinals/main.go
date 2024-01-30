package main

import (
	"encoding/hex"
	"go-demo/bitcoin/pkg/btcapi/mempool"
	"log"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// 两阶段提交
// 1.1 承诺：创建包含铭刻内容的脚本的 Taproot 输出
// 1.2 揭示：承诺交易创建的输出被消费
// todo
// commitTxAddress交易无法处理
func main() {
	netParams := &chaincfg.SigNetParams
	privateKeyHex01 := "7e3503d37b624a541815ce378f00f5ba01914708d9c8572ddc7057cc649659de"
	privateKeyByte, _ := hex.DecodeString(privateKeyHex01)
	privateKeyBytes01 := privateKeyByte // 用你自己的私钥替换这里的字节
	privateKey01, _ := btcec.PrivKeyFromBytes(privateKeyBytes01)

	sourceAddrPubKey, err := btcutil.NewAddressTaproot(schnorr.SerializePubKey(txscript.ComputeTaprootKeyNoScript(privateKey01.PubKey())), netParams)
	if err != nil {
		log.Fatal("NewAddressTaproot err:", err)
	}
	log.Println("sourceAddress:", sourceAddrPubKey.EncodeAddress())

	inscriptionBuilder := txscript.NewScriptBuilder().
		AddData(schnorr.SerializePubKey(privateKey01.PubKey())).
		AddOp(txscript.OP_CHECKSIG).
		AddOp(txscript.OP_FALSE).
		AddOp(txscript.OP_IF).
		AddData([]byte("ord")).
		// Two OP_DATA_1 should be OP_1. However, in the following link, it's not set as OP_1:
		// https://github.com/casey/ord/blob/0.5.1/src/inscription.rs#L17
		// Therefore, we use two OP_DATA_1 to maintain consistency with ord.
		AddOp(txscript.OP_DATA_1).
		AddOp(txscript.OP_DATA_1).
		AddData([]byte("text/plain;charset=utf-8")).
		AddOp(txscript.OP_0)
	maxChunkSize := 520
	body := []byte("my is test data")
	bodySize := len(body)
	for i := 0; i < bodySize; i += maxChunkSize {
		end := i + maxChunkSize
		if end > bodySize {
			end = bodySize
		}
		// to skip txscript.MaxScriptSize 10000
		inscriptionBuilder.AddFullData(body[i:end])
	}
	inscriptionScript, err := inscriptionBuilder.Script()
	if err != nil {
		log.Fatal("inscriptionBuilder err:", err)
	}
	inscriptionScript = append(inscriptionScript, txscript.OP_ENDIF)

	leafNode := txscript.NewBaseTapLeaf(inscriptionScript)
	proof := &txscript.TapscriptProof{
		TapLeaf:  leafNode,
		RootNode: leafNode,
	}

	controlBlock := proof.ToControlBlock(privateKey01.PubKey())
	controlBlockWitness, err := controlBlock.ToBytes()
	if err != nil {
		log.Fatal("controlBlock.ToBytes err:", err)
	}
	log.Println(controlBlockWitness)
	tapHash := proof.RootNode.TapHash()

	// 这里比较重要，要转给这个地址
	commitTxAddress, err := btcutil.NewAddressTaproot(schnorr.SerializePubKey(txscript.ComputeTaprootOutputKey(privateKey01.PubKey(), tapHash[:])), netParams)
	if err != nil {
		log.Fatal("commitTxAddress NewAddressTaproot  err:", err)
	}
	log.Println("commitTxAddress:", commitTxAddress)

	err = CommitTx(sourceAddrPubKey.EncodeAddress(), commitTxAddress.EncodeAddress(), privateKey01)
	if err != nil {
		log.Fatal("commitTx err:", err)
	}

	//toAddress := "tb1plfmp4605cr79pnrrev8x2pxt3ngwx5xpffsllz7p57mfp57zmupsmkg20n"
	err = RevealTx(commitTxAddress.EncodeAddress(), sourceAddrPubKey.EncodeAddress(), leafNode, inscriptionScript, controlBlockWitness, privateKey01)
	if err != nil {
		log.Fatal("RevealTx err:", err)
	}

}

// 承诺
func CommitTx(sourceAddrress string, destAddress string, privateKey01 *btcec.PrivateKey) error {
	var amount int64 = 2000
	netParams := &chaincfg.SigNetParams
	btcApiClient := mempool.NewClient(netParams)

	sourceAddr, err := btcutil.DecodeAddress(sourceAddrress, &chaincfg.SigNetParams)
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

	var totalSenderAmount int64
	tx := wire.NewMsgTx(wire.TxVersion)
	for _, v := range unspentList {
		// 去除不是PayToTaproot
		txIn := wire.NewTxIn(v.Outpoint, nil, nil)
		tx.AddTxIn(txIn)
		totalSenderAmount += v.Output.Value
		// 钱够了就不继续计算了
		if totalSenderAmount > amount {
			break
		}
	}

	// 计算总金额和交易费用
	var fee int64 = 1000
	changeAmount := totalSenderAmount - amount - fee // 减去交易费用，转账2000

	// 添加目标地址作为交易输出
	// 获取目标地址的比特币脚本
	destAddr, err := btcutil.DecodeAddress(destAddress, &chaincfg.SigNetParams)
	if err != nil {
		log.Fatal(err)
	}
	destinationScript, err := txscript.PayToAddrScript(destAddr)
	if err != nil {
		log.Fatal(err)
	}
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
		prevOutputFetcher := txscript.NewMultiPrevOutFetcher(map[wire.OutPoint]*wire.TxOut{
			*unspentList[k].Outpoint: {
				PkScript: unspentList[k].Output.PkScript,
				Value:    unspentList[k].Output.Value,
			},
		})

		sigHashes := txscript.NewTxSigHashes(tx, prevOutputFetcher)
		witness, err := txscript.TaprootWitnessSignature(tx, sigHashes,
			k, unspentList[k].Output.Value, unspentList[k].Output.PkScript, txscript.SigHashDefault, privateKey01)
		if err != nil {
			log.Fatal("TaprootWitnessSignature err:", err)
		}
		v.Witness = witness
	}
	txHash, err := btcApiClient.BroadcastTx(tx)
	if err != nil {
		log.Fatal("send tx failed:", err)
	}
	if err != nil {
		log.Fatal("send tx failed:", err)
	}
	log.Printf("CommitTx tx_hash：%s\n", txHash.String())
	return nil
}

// 揭示
func RevealTx(sourceAddrress string, destAddress string, leafNode txscript.TapLeaf, inscriptionScript []byte, controlBlockWitness []byte, privateKey01 *btcec.PrivateKey) error {
	netParams := &chaincfg.SigNetParams
	btcApiClient := mempool.NewClient(netParams)

	sourceAddr, err := btcutil.DecodeAddress(sourceAddrress, &chaincfg.SigNetParams)
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
	log.Println("len:", len(unspentList))

	var totalSenderAmount int64
	tx := wire.NewMsgTx(wire.TxVersion)
	for _, v := range unspentList {
		txIn := wire.NewTxIn(v.Outpoint, nil, nil)
		txIn.Sequence = wire.MaxTxInSequenceNum - 10
		tx.AddTxIn(txIn)
		totalSenderAmount += v.Output.Value
	}
	// 计算总金额和交易费用
	var fee int64 = 1000
	changeAmount := totalSenderAmount - fee - 500 // 减去交易费用，转账2000

	// 添加目标地址作为交易输出
	destAddr, err := btcutil.DecodeAddress(destAddress, &chaincfg.SigNetParams)
	if err != nil {
		log.Fatal(err)
	}
	destAddrScript, err := txscript.PayToAddrScript(destAddr)
	if err != nil {
		log.Fatal(err)
	}
	tx.AddTxOut(wire.NewTxOut(500, destAddrScript))

	changeScript, err := txscript.PayToAddrScript(sourceAddr)
	if err != nil {
		log.Fatal(err)
	}
	// 添加找零地址作为交易输出
	tx.AddTxOut(wire.NewTxOut(changeAmount, changeScript))

	for i := range tx.TxIn {
		prevOutputFetcher := txscript.NewCannedPrevOutputFetcher(
			unspentList[i].Output.PkScript, unspentList[i].Output.Value,
		)
		sigHashes := txscript.NewTxSigHashes(tx, prevOutputFetcher)

		witnessArray, err := txscript.CalcTapscriptSignaturehash(sigHashes,
			txscript.SigHashDefault, tx, i, prevOutputFetcher, leafNode)
		if err != nil {
			log.Fatal("CalcTapscriptSignaturehash err:", err)
		}

		signature, err := schnorr.Sign(privateKey01, witnessArray)
		if err != nil {
			log.Fatal("sign err:", err)
		}
		tx.TxIn[i].Witness = wire.TxWitness{signature.Serialize(), inscriptionScript, controlBlockWitness}

	}

	txHash, err := btcApiClient.BroadcastTx(tx)
	if err != nil {
		log.Fatal("send tx failed:", err)
	}
	if err != nil {
		log.Fatal("send tx failed:", err)
	}
	log.Printf("RevealTx tx_hash：%s\n", txHash.String())
	return nil
}
