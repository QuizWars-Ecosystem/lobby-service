buf-gen:
	cd ./protobuf && make buf-gen-server

proto-pull:
	git submodule update --remote --force protobuf

go-fmt:
	gofumpt -l -w .

go-lint:
	golangci-lint run ./...

test:
	go test -v -coverpkg=./... -coverprofile=cover.out ./tests/integration_tests

cover-svg:
	go-cover-treemap -percent=true -w=1080 -h=360 -coverprofile cover.out > cover.svg

before-push:
	go mod tidy
	gofumpt -l -w .
	golangci-lint run ./...