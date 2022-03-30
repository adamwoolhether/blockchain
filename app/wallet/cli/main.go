package main

import (
	"log"
	
	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	err := genKey()
	if err != nil {
		log.Fatalln(err)
	}
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
