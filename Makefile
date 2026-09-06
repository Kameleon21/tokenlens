.PHONY: check release-check release-snapshot release-prepare release-tag prices-update

check:
	@test -z "$$(gofmt -l .)"
	go test -race ./...
	go vet ./...
	go build -o bin/tokenlens .
	python3 -m unittest discover -s scripts -p 'test_*.py'
	python3 scripts/release.py check

release-check:
	@goreleaser --version | grep -Eq "^GitVersion: +$$(sed 's/^v//; s/\./[.]/g' .goreleaser-version)$$" || { echo "Install GoReleaser $$(cat .goreleaser-version) first"; exit 1; }
	goreleaser check
	python3 scripts/release.py check

release-snapshot: check release-check
	goreleaser release --snapshot --clean

# Example: make release-prepare BUMP=minor
release-prepare:
	python3 scripts/release.py prepare $(BUMP)

# Run on clean, synchronized main after its Checks workflow passes.
release-tag:
	python3 scripts/release.py tag

# Refresh the committed initial catalog; review and commit its changes.
prices-update:
	python3 scripts/sync_prices.py
