OPENAPI_GENERATOR_CLI_VERSION=7.12.0

.PHONY: install
install:
	@npm install @openapitools/openapi-generator-cli -g
	@openapi-generator-cli version-manager set ${OPENAPI_GENERATOR_CLI_VERSION}
	@go install entgo.io/ent/cmd/ent@latest
	@go install ariga.io/atlas/cmd/atlas@latest

.PHONY: rest-api
rest-api:
	@echo ">>> Generating REST API..."
	@go generate ./pkg/api

.PHONY: ent-generate
ent-generate:
	@echo ">>> Generating Ent schema implementation files..."
	@go generate ./pkg/storage/postgres/ent

.PHONY: test
test:
	@echo ">>> Running tests..."
	@go test ./pkg/...

.PHONY: reprocess
reprocess:
	@echo ">>> Running reprocessing tool..."
	@go run ./cmd/reprocess --env-file=./cmd/reprocess/.env $(ARGS)

.PHONY: reprocess-dry-run
reprocess-dry-run:
	@echo ">>> Running reprocessing tool (dry run)..."
	@go run ./cmd/reprocess --dry-run --env-file=./cmd/reprocess/.env $(ARGS)
