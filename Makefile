.PHONY: test test-integration lint docs docs-serve fetch-catppuccin db-up db-down db-psql migrate-platform migrate-check create-tenant

# .env is optional: test, lint and docs don't need it. Its variables are
# exported to every recipe, so goose sees DATABASE_URL.
-include .env
export

GOOSE_PLATFORM = go tool goose -dir migrations/platform postgres "$$DATABASE_URL"

test:
	go test -race ./...

# Needs the compose database with the platform migration: make db-up migrate-platform.
test-integration:
	go test -race -tags integration ./...

lint:
	go vet -tags integration ./...
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi
	go tool staticcheck -tags integration ./...

docs:
	mdbook build docs

docs-serve:
	mdbook serve --open docs

fetch-catppuccin:
	wget -O ./docs/theme/catppuccin.css https://github.com/catppuccin/mdBook/releases/latest/download/catppuccin.css

db-up:
	docker compose up -d --wait

db-down:
	docker compose down

db-psql:
	docker compose exec db psql -U cowbook -d cowbook

migrate-platform:
	$(GOOSE_PLATFORM) up
	$(GOOSE_PLATFORM) status

# Up, all the way down, and up again: proves every Down migration works.
migrate-check:
	$(GOOSE_PLATFORM) up
	$(GOOSE_PLATFORM) reset
	$(GOOSE_PLATFORM) up

# make create-tenant SLUG=north-hub NAME="North Hub"
create-tenant:
	go run ./cmd/cowbook create-tenant "$(SLUG)" "$(NAME)"
