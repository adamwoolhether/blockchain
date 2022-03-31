package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	
	"github.com/ethereum/go-ethereum/crypto"
	
	"github.com/adamwoolhether/blockchain/foundation/blockchain/storage"
)

func main() {
	to := flag.String("t", "", "to")
	nonce := flag.Uint("n", 0, "nonce")
	value := flag.Uint("v", 0, "value")
	tip := flag.Uint("p", 0, "tip")
	flag.Parse()
	
	err := sendTran(*to, *nonce, *value, *tip)
	if err != nil {
		log.Fatalln(err)
	}
}

func sendTran(to string, nonce, value, tip uint) error {
	privateKey, err := crypto.LoadECDSA("zblock/accounts/kennedy.ecdsa")
	if err != nil {
		return err
	}
	
	toAccount, err := storage.ToAccount(to)
	if err != nil {
		log.Fatal(err)
	}
	
	userTx, err := storage.NewUserTx(nonce, toAccount, value, tip, nil)
	if err != nil {
		log.Fatal(err)
	}
	
	walletTx, err := userTx.Sign(privateKey)
	if err != nil {
		log.Fatal(err)
	}
	
	data, err := json.Marshal(walletTx)
	if err != nil {
		log.Fatal(err)
	}
	
	url := "http://localhost:8080"
	resp, err := http.Post(fmt.Sprintf("%s/v1/tx/submit", url), "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	
	return nil
}

func genKey() error {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}
	
	if err := crypto.SaveECDSA("./adam.ecdsa", privateKey); err != nil {
		log.Fatal(err)
	}
	
	return nil
}
