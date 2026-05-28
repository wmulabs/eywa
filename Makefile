.PHONY: test test-verbose coverage coverage-html lint fmt

COVERAGE_PACKAGES := \
	./internal/domain/... \
	./internal/implementation/... \
	./internal/helpers/... \
	./internal/infrastructure/...

test:
	go test ./...

test-verbose:
	go test ./... -v

coverage:
	go test $(COVERAGE_PACKAGES) -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -1

coverage-html: coverage
	go tool cover -html=coverage.out -o coverage.html
	open coverage.html

fmt:
	gofmt -w .

lint:
	go vet ./...
