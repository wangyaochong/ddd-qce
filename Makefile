GOPATH := $(shell go env GOPATH)
LINT := $(shell PATH="$(PATH):$(GOPATH)/bin" which golangci-lint 2>/dev/null || echo "$(GOPATH)/bin/golangci-lint")

.PHONY: lint lint-core lint-example lint-exampleapp lint-it
.PHONY: fix fix-core fix-example fix-exampleapp fix-it
.PHONY: test test-core test-example test-exampleapp test-it

lint: lint-core lint-example lint-exampleapp lint-it

LINT_FLAGS := --max-same-issues=50

lint-core:
	$(LINT) run $(LINT_FLAGS) ./...

lint-example:
	cd example && GOWORK=off $(LINT) run $(LINT_FLAGS) ./...

lint-exampleapp:
	cd exampleapp && GOWORK=off $(LINT) run $(LINT_FLAGS) ./...

lint-it:
	cd it && $(LINT) run $(LINT_FLAGS) ./...

fix: fix-core fix-example fix-exampleapp fix-it

fix-core:
	$(LINT) run --fix ./...

fix-example:
	cd example && GOWORK=off $(LINT) run --fix ./...

fix-exampleapp:
	cd exampleapp && GOWORK=off $(LINT) run --fix ./...

fix-it:
	cd it && $(LINT) run --fix ./...

test:
	go test ./... github.com/ddd-qce/example/... github.com/ddd-qce/exampleapp/...

test-core:
	go test ./...

test-example:
	go test github.com/ddd-qce/example/...

test-exampleapp:
	go test github.com/ddd-qce/exampleapp/...

test-it:
	go test ./it/...
