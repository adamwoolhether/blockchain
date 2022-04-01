SHELL := $(shell echo ${SHELL})

# Wallets
# Adam: 	0xeCCc29987128DEbee767c1Ec6A6fea3507dEF877
# Nikki: 	0xA211f66bD829205102c33cAD3A212D7CaD66025D
# Mantou: 	0x4996b5db6639d7775e410C5A2A0Ada4C1D0042E5
# Arc:		0x118947E5266BF8Cd2E730b22f5E66a7868C6DBbC
# Miner1: 	0xdF3212a524C8f7329970D9a5A227d9D40D8723D9
# Miner2: 	0x2378baB7101cDcE1084A540Fb885D8E3779a1DB2

# curl -X GET http://localhost:8080/v1/genesis | jq
# curl -X GET http://localhost:8080/v1/accounts/list | jq
# curl -X GET http://localhost:8080/v1/tx/uncommitted/list | jq

# ######################################################################################################################
# Local support
up:
	go run app/services/node/main.go -race | go run app/tooling/logfmt/main.go
up2:
	go run app/services/node/main.go -race --web-public-host 0.0.0.0:8180 --web-private-host 0.0.0.0:9180 --node-miner-name=miner2 --node-db-path zblock/blocks2.db | go run app/tooling/logfmt/main.go

down:
	kill -INT $(shell ps | grep "main -race" | grep -v grep | sed -n 1,1p | cut -c1-5)

clear-db:
	cat /dev/null > zblock/blocks.db
	cat /dev/null > zblock/blocks2.db

key:
	go run app/wallet/cli/main.go generate

load:
	go run app/wallet/cli/main.go send adam --to "0xA211f66bD829205102c33cAD3A212D7CaD66025D" --nonce 1 --value 450 --tip 15
	go run app/wallet/cli/main.go send nikki --to "0x4996b5db6639d7775e410C5A2A0Ada4C1D0042E5" --nonce 2 --value 200 --tip 15
	go run app/wallet/cli/main.go send adam --to "0x118947E5266BF8Cd2E730b22f5E66a7868C6DBbC" --nonce 3 --value 100 --tip 15
	go run app/wallet/cli/main.go send adam --to "0x4996b5db6639d7775e410C5A2A0Ada4C1D0042E5" --nonce 4 --value 230 --tip 15

# ######################################################################################################################
# Modules support

tidy:
	go mod tidy
	go mod vendor
