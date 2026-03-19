.PHONY: test test-integration test-fast build install lint release-dry e2e-user e2e-user-docker

test:
	go test ./... -race -count=1

test-integration:
	go test -tags integration ./... -race -count=1 -timeout 120s

test-fast:
	go test ./... -short -count=1

build:
	go build -o ./bin/sqlshift .

# Full CLI path a human uses: init → create → edit migration → up/status/validate/down (SQLite file DB).
e2e-user:
	./scripts/e2e-user-journey.sh

# Same as e2e-user, plus docker-compose.real-db.yml Postgres + MySQL “fake prod” databases.
e2e-user-docker:
	./scripts/e2e-user-journey.sh --with-docker

install:
	go install .

lint:
	golangci-lint run

release-dry:
	goreleaser release --snapshot --clean
