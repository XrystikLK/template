ifneq (,$(wildcard .env))
    include .env
    export
endif

MIGRATIONS_DIR = ./migrations

.PHONY: create migrate-up run test

generate: 
	go tool oapi-codegen -generate types,chi-server -package api -o internal/generated/api.gen.go contracts/openapi/trip-service.openapi.yaml

create:
	goose -dir ${MIGRATIONS_DIR} create $(NAME) sql

migrate-up:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" up
