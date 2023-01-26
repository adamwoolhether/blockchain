SHELL := $(shell echo ${SHELL})

# Wallets
# Adam: 	0xeCCc29987128DEbee767c1Ec6A6fea3507dEF877
# Nikki: 	0xA211f66bD829205102c33cAD3A212D7CaD66025D
# Mantou: 	0x4996b5db6639d7775e410C5A2A0Ada4C1D0042E5
# Arc:		0x118947E5266BF8Cd2E730b22f5E66a7868C6DBbC
# Goku 		0x26814dA49253798250D6c00270f2A8A6BC0424b7
# Miner1: 	0xdF3212a524C8f7329970D9a5A227d9D40D8723D9
# Miner2: 	0x2378baB7101cDcE1084A540Fb885D8E3779a1DB2

# Bookeeping transactions
# curl -il -X GET http://localhost:8080/v1/genesis/list
# curl -il -X GET http://localhost:9080/v1/node/status
# curl -il -X GET http://localhost:8080/v1/accounts/list
# curl -il -X GET http://localhost:8080/v1/tx/uncommitted/list
# curl -il -X GET http://localhost:8080/v1/blocks/list
# curl -il -X GET http://localhost:9080/v1/node/block/list/1/latest
#
# curl -X GET http://localhost:8080/v1/genesis/list | jq
# curl -X GET http://localhost:9080/v1/node/status | jq
# curl -X GET http://localhost:8080/v1/accounts/list | jq
# curl -X GET http://localhost:8080/v1/tx/uncommitted/list | jq
# curl -X GET http://localhost:8080/v1/blocks/list | jq
# curl -X GET http://localhost:9080/v1/node/block/list/1/latest | jq

# ######################################################################################################################
# Local support
up:
	go run app/services/node/main.go -race | go run app/tooling/logfmt/main.go
up2:
	go run app/services/node/main.go -race --web-debug-host 0.0.0.0:7281 --web-public-host 0.0.0.0:8280 --web-private-host 0.0.0.0:9280 --state-beneficiary=miner2 --state-db-path zblock/miner2/ | go run app/tooling/logfmt/main.go

down:
	kill -INT $(shell ps | grep "main -race" | grep -v grep | sed -n 1,1p | cut -c1-5)

#clear-db:
	#cat /dev/null > zblock/blocks.db
	#cat /dev/null > zblock/blocks2.db

key:
	go run app/wallet/cli/main.go generate

# ######################################################################################################################
# Docker support

docker-up:
	docker compose -f zarf/docker/docker-compose.yaml up
docker-down:
	docker compose -f zarf/docker/docker-compose.yaml down
docker-logs:
	docker compose -f zarf/docker/docker-compose.yaml logs

load:
	go run app/wallet/cli/main.go send --account adam --from 0xeCCc29987128DEbee767c1Ec6A6fea3507dEF877 --to 0xA211f66bD829205102c33cAD3A212D7CaD66025D --nonce 1 --value 450 --tip 15
	go run app/wallet/cli/main.go send --account nikki --from 0xA211f66bD829205102c33cAD3A212D7CaD66025D --to 0x4996b5db6639d7775e410C5A2A0Ada4C1D0042E5 --nonce 2 --value 200 --tip 15
	go run app/wallet/cli/main.go send --account adam --from 0xeCCc29987128DEbee767c1Ec6A6fea3507dEF877 --to 0x118947E5266BF8Cd2E730b22f5E66a7868C6DBbC --nonce 3 --value 100 --tip 15
	go run app/wallet/cli/main.go send --account adam --from 0xeCCc29987128DEbee767c1Ec6A6fea3507dEF877 --to 0x4996b5db6639d7775e410C5A2A0Ada4C1D0042E5 --nonce 4 --value 230 --tip 15
	go run app/wallet/cli/main.go send --account adam --from 0xeCCc29987128DEbee767c1Ec6A6fea3507dEF877 --to 0x26814dA49253798250D6c00270f2A8A6BC0424b7 --nonce 5 --value 450 --tip 15
	go run app/wallet/cli/main.go send --account nikki --from 0xA211f66bD829205102c33cAD3A212D7CaD66025D --to 0x26814dA49253798250D6c00270f2A8A6BC0424b7 --nonce 6 --value 200 --tip 15

load2:
	go run app/wallet/cli/main.go send --account adam --from 0xeCCc29987128DEbee767c1Ec6A6fea3507dEF877 --to 0xA211f66bD829205102c33cAD3A212D7CaD66025D --nonce 1 --value 450 --tip 15
	go run app/wallet/cli/main.go send --account nikki --from 0xA211f66bD829205102c33cAD3A212D7CaD66025D --to 0x4996b5db6639d7775e410C5A2A0Ada4C1D0042E5 --nonce 2 --value 200 --tip 15

load3:
	go run app/wallet/cli/main.go send --account adam --from 0xeCCc29987128DEbee767c1Ec6A6fea3507dEF877 --to 0x118947E5266BF8Cd2E730b22f5E66a7868C6DBbC --nonce 3 --value 100 --tip 15
	go run app/wallet/cli/main.go send --account adam --from 0xeCCc29987128DEbee767c1Ec6A6fea3507dEF877 --to 0x4996b5db6639d7775e410C5A2A0Ada4C1D0042E5 --nonce 4 --value 230 --tip 15
	go run app/wallet/cli/main.go send --account adam --from 0xeCCc29987128DEbee767c1Ec6A6fea3507dEF877 --to 0x26814dA49253798250D6c00270f2A8A6BC0424b7 --nonce 5 --value 450 --tip 15
	go run app/wallet/cli/main.go send --account nikki --from 0xA211f66bD829205102c33cAD3A212D7CaD66025D --to 0x26814dA49253798250D6c00270f2A8A6BC0424b7 --nonce 6 --value 200 --tip 15

# ######################################################################################################################
# Viewer Support
react:
	npm install --prefix app/services/viewer
	npm start --prefix app/services/viewer

viewer:
	open -a "Google Chrome" http://localhost

# ######################################################################################################################
# Modules support

deps-reset:
	git checkout -- go.mod
	go mod tidy
	go mod vendor

tidy:
	go mod tidy
	go mod vendor

deps-upgrade:
	# go get $(go list -f '{{if not (or .Main .Indirect)}}{{.Path}}{{end}}' -m all)
	go get -u -v ./...
	go mod tidy
	go mod vendor

######################################################################################################################
# Tests
# go install honnef.co/go/tools/cmd/staticcheck@latest
# go install golang.org/x/vuln/cmd/govulncheck@latest

test:
	go test -count=1 ./...
	go vet ./...
	staticcheck -checks=all ./...
	govulncheck ./...