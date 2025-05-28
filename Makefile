.PHONY: api-clients
api-docs:
	@echo ">>> Generating OpenAPI documentation..."
	@go generate ./pkg/api
