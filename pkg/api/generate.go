package api

// Global installation of the OpenAPI CLI is required: https://openapi-generator.tech/docs/installation

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config cfg.yaml ./openapi.yaml
//go:generate rm -r ../../clients
//go:generate openapi-generator-cli generate -i ./openapi.yaml -g typescript-axios -o ../../clients/ts
