test:
	go test ./... -race -count=1

test-integration:
	go test -tags integration ./... -race -count=1 -timeout 120s

test-fast:
	go test ./... -short -count=1

build:
	go build -o ./bin/sqlshift .

install:
	go install .

lint:
	golangci-lint run

release-dry:
	goreleaser release --snapshot --clean
