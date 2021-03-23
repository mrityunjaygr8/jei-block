package main

import (
	"encoding/hex"
	"log"

	"github.com/boltdb/bolt"
)

const utxoBucket = "chainstate"

type UTXOSet struct {
	Blockchain *Blockchain
}

func (u UTXOSet) Reindex() {
	db := u.Blockchain.db
	bucketName := []byte(utxoBucket)

	err := db.Update(func(t *bolt.Tx) error {
		err := t.DeleteBucket(bucketName)
		if err != nil && err != bolt.ErrBucketNotFound {
			log.Panic(err)
		}
		_, err = t.CreateBucket(bucketName)
		if err != nil {
			log.Panic(err)
		}

		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	UTXO := u.Blockchain.FindUTXO()
	err = db.Update(func(t *bolt.Tx) error {
		b := t.Bucket(bucketName)

		for txID, outs := range UTXO {
			key, err := hex.DecodeString(txID)
			if err != nil {
				log.Panic(err)
			}
			err = b.Put(key, outs.Serialize())
			if err != nil {
				log.Panic(err)
			}
		}

		return nil
	})

	if err != nil {
		log.Panic(err)
	}
}

func (u UTXOSet) FindSpendableOutputs(publickeyHash []byte, amount int) (int, map[string][]int) {
	unspentOutputs := make(map[string][]int)
	accumulated := 0
	db := u.Blockchain.db

	err := db.View(func(t *bolt.Tx) error {
		b := t.Bucket([]byte(utxoBucket))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			txID := hex.EncodeToString(k)
			outs := DeserializeOutputs(v)

			for outIdx, out := range outs.Outputs {
				if out.IsLockedWithKey(publickeyHash) && accumulated < amount {
					accumulated += out.Value
					unspentOutputs[txID] = append(unspentOutputs[txID], outIdx)
				}
			}
		}

		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	return accumulated, unspentOutputs
}

func (u UTXOSet) FindUTXO(pubKeyHash []byte) []TXOutput {
	var UTXOs []TXOutput
	db := u.Blockchain.db

	err := db.View(func(t *bolt.Tx) error {
		b := t.Bucket([]byte(utxoBucket))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			outs := DeserializeOutputs(v)

			for _, out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) {
					UTXOs = append(UTXOs, out)
				}
			}
		}

		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	return UTXOs
}

func (u UTXOSet) Update(block *Block) {
	db := u.Blockchain.db

	err := db.Update(func(t *bolt.Tx) error {
		b := t.Bucket([]byte(utxoBucket))

		for _, t := range block.Transactions {
			if !t.IsCoinbase() {
				for _, vin := range t.Vin {
					updatedOuts := TXOutputs{}
					outsBytes := b.Get(vin.Txid)
					outs := DeserializeOutputs(outsBytes)

					for outIdx, out := range outs.Outputs {
						if outIdx != vin.Vout {
							updatedOuts.Outputs = append(updatedOuts.Outputs, out)
						}
					}

					if len(updatedOuts.Outputs) == 0 {
						err := b.Delete(vin.Txid)
						if err != nil {
							log.Panic(err)
						}
					} else {
						err := b.Put(vin.Txid, updatedOuts.Serialize())
						if err != nil {
							log.Panic(err)
						}
					}
				}
			}

			newOutputs := TXOutputs{}
			newOutputs.Outputs = append(newOutputs.Outputs, t.Vout...)

			err := b.Put(t.ID, newOutputs.Serialize())
			if err != nil {
				log.Panic(err)
			}
		}

		return nil
	})

	if err != nil {
		log.Panic(err)
	}
}

func (u UTXOSet) CountTransactions() int {
	db := u.Blockchain.db
	count := 0

	err := db.View(func(t *bolt.Tx) error {
		b := t.Bucket([]byte(utxoBucket))
		c := b.Cursor()

		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			count++
		}

		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	return count
}
