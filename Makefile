.PHONY: test lint docs docs-serve

test:
	go test ./...

lint:
	go vet ./...
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi
	go tool staticcheck ./...

docs:
	mdbook build docs

docs-serve:
	mdbook serve --open docs

fetch-catppuccin:
	wget -O ./docs/theme/catppuccin.css https://github.com/catppuccin/mdBook/releases/latest/download/catppuccin.css
