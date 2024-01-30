package main

import (
	"encoding/hex"
	"go-demo/bitcoin/pkg/btcapi/mempool"
	"log"
	"sync"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcec/v2/schnorr/musig2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// https://mempool.space/zh/signet/tx/3cd35114cae482fc1c7c3309d74963b12712e859faf5b73e97f559d3cc988721
func main() {
	const numSigners = 2
	signerKeys := make([]*btcec.PrivateKey, 0)
	signSet := make([]*btcec.PublicKey, 0)
	privateKeyHex01 := "7e3503d37b624a541815ce378f00f5ba01914708d9c8572ddc7057cc649659de"
	privateKeyBytes01, _ := hex.DecodeString(privateKeyHex01)
	privateKey01, pubKey01 := btcec.PrivKeyFromBytes(privateKeyBytes01)

	privateKeyHex02 := "ddcc69506d5c7088078315386fac523d36d2d88c37ab0d56fa6e24501f1a4463"
	privateKeyBytes02, _ := hex.DecodeString(privateKeyHex02)
	privateKey02, pubKey02 := btcec.PrivKeyFromBytes(privateKeyBytes02)

	signerKeys = append(signerKeys, privateKey01, privateKey02)
	signSet = append(signSet, pubKey01, pubKey02)

	var combinedKey *btcec.PublicKey

	var ctxOpts []musig2.ContextOption

	ctxOpts = append(ctxOpts, musig2.WithBip86TweakCtx())

	ctxOpts = append(ctxOpts, musig2.WithKnownSigners(signSet))

	// for example: github.com/btcsuite/btcd/btcec/v2/schnorr/musig2/musig2_test.go
	signers := make([]*musig2.Session, numSigners)
	for i, signerKey := range signerKeys {
		signCtx, err := musig2.NewContext(
			signerKey, false, ctxOpts...,
		)
		if err != nil {
			log.Fatalf("unable to generate context: %v", err)
		}

		if combinedKey == nil {
			combinedKey, err = signCtx.CombinedKey()
			if err != nil {
				log.Fatalf("combined key not available: %v", err)
			}
		}

		session, err := signCtx.NewSession()
		if err != nil {
			log.Fatalf("unable to generate new session: %v", err)
		}
		signers[i] = session
	}

	fromAddr, err := btcutil.NewAddressTaproot(schnorr.SerializePubKey(combinedKey), &chaincfg.SigNetParams)
	if err != nil {
		log.Fatal("NewAddressTaproot:", err)
	}
	log.Println("address:", fromAddr.EncodeAddress())

	// 构造交易
	netParams := &chaincfg.SigNetParams
	btcApiClient := mempool.NewClient(netParams)
	unspentList, err := btcApiClient.ListUnspent(fromAddr)
	if err != nil {
		log.Fatal("ListUnspent err:", err)
	}
	if len(unspentList) == 0 {
		log.Fatal("unspentList len is 0")
	}
	tx := wire.NewMsgTx(2)
	tx.TxIn = []*wire.TxIn{{
		PreviousOutPoint: *unspentList[0].Outpoint,
	}}

	totalSenderAmount := btcutil.Amount(0)
	totalSenderAmount = btcutil.Amount(unspentList[0].Output.Value)
	changeAmount := int64(totalSenderAmount) - 1000

	p2wkhPkScript, err := txscript.PayToAddrScript(fromAddr)
	if err != nil {
		log.Fatal("PayToAddrScript err:", err)
	}
	tx.TxOut = []*wire.TxOut{{
		PkScript: p2wkhPkScript,
		Value:    changeAmount,
	}}

	prevOutputFetcher := txscript.NewCannedPrevOutputFetcher(
		unspentList[0].Output.PkScript, unspentList[0].Output.Value,
	)
	sighashes := txscript.NewTxSigHashes(tx, prevOutputFetcher)
	sigHash, err := txscript.CalcTaprootSignatureHash(
		sighashes, txscript.SigHashDefault, tx, 0, prevOutputFetcher,
	)

	msg := [32]byte(sigHash)

	var wg sync.WaitGroup
	for i, signCtx := range signers {
		signCtx := signCtx

		wg.Add(1)
		go func(idx int, signer *musig2.Session) {
			defer wg.Done()

			for j, otherCtx := range signers {
				if idx == j {
					continue
				}

				nonce := otherCtx.PublicNonce()
				haveAll, err := signer.RegisterPubNonce(nonce)
				if err != nil {
					log.Fatalf("unable to add public nonce")
				}

				if j == len(signers)-1 && !haveAll {
					log.Fatalf("all public nonces should have been detected")
				}
			}
		}(i, signCtx)
	}

	wg.Wait()

	// In the final step, we'll use the first signer as our combiner, and
	// generate a signature for each signer, and then accumulate that with
	// the combiner.
	combiner := signers[0]
	for i := range signers {
		signer := signers[i]
		partialSig, err := signer.Sign(msg)
		if err != nil {
			log.Fatalf("unable to generate partial sig: %v", err)
		}

		// We don't need to combine the signature for the very first
		// signer, as it already has that partial signature.
		if i != 0 {
			haveAll, err := combiner.CombineSig(partialSig)
			if err != nil {
				log.Fatalf("unable to combine sigs: %v", err)
			}

			if i == len(signers)-1 && !haveAll {
				log.Fatalf("final sig wasn't reconstructed")
			}
		}
	}

	// Finally we'll combined all the nonces, and ensure that it validates
	// as a single schnorr signature.
	finalSig := combiner.FinalSig()

	if !finalSig.Verify(msg[:], combinedKey) {
		log.Fatalf("final sig is invalid!")
	}

	tx.TxIn[0].Witness = wire.TxWitness{finalSig.Serialize()}

	//  SendRawTransaction
	txHash, err := btcApiClient.BroadcastTx(tx)
	if err != nil {
		log.Fatal("BroadcastTx err:", err)
	}
	log.Printf("txhash：%s\n", txHash.String())
}
