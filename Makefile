GOPATH := $(shell go env GOPATH)
LINT := $(shell PATH="$(PATH):$(GOPATH)/bin" which golangci-lint 2>/dev/null || echo "$(GOPATH)/bin/golangci-lint")

.PHONY: lint lint-core lint-example lint-exampleapp lint-it
.PHONY: fix fix-core fix-example fix-exampleapp fix-it
.PHONY: test test-core test-example test-exampleapp test-it
.PHONY: ddd-lint ddd-lint-core ddd-lint-example ddd-lint-exampleapp ddd-lint-it

lint: lint-core lint-example lint-exampleapp lint-it

LINT_FLAGS := --max-same-issues=50

lint-core:
	$(LINT) run $(LINT_FLAGS) ./...

lint-example:
	cd examples && GOWORK=off $(LINT) run $(LINT_FLAGS) ./...

lint-exampleapp:
	cd exampleapp && GOWORK=off $(LINT) run $(LINT_FLAGS) ./...

lint-it:
	cd integrationtest && $(LINT) run $(LINT_FLAGS) ./...

fix: fix-core fix-example fix-exampleapp fix-it

fix-core:
	$(LINT) run --fix ./...

fix-example:
	cd examples && GOWORK=off $(LINT) run --fix ./...

fix-exampleapp:
	cd exampleapp && GOWORK=off $(LINT) run --fix ./...

fix-it:
	cd integrationtest && $(LINT) run --fix ./...

test:
	go test ./... github.com/ddd-qce/examples/... github.com/ddd-qce/exampleapp/...

test-core:
	go test ./...

test-example:
	go test github.com/ddd-qce/examples/...

test-exampleapp:
	go test github.com/ddd-qce/exampleapp/...

test-it:
	go test ./integrationtest/...

ddd-lint: ddd-lint-core ddd-lint-example ddd-lint-exampleapp ddd-lint-it

ddd-lint-core:
	go run ./lint/cmd/ddd-lint ./...

ddd-lint-example:
	cd examples && go run ../lint/cmd/ddd-lint ./...

ddd-lint-exampleapp:
	cd exampleapp && go run ../lint/cmd/ddd-lint ./...

ddd-lint-it:
	cd integrationtest && go run ../lint/cmd/ddd-lint ./...
