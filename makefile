.PHONY: test
test:
	@ go test ./...

.PHONY: test-all
test-all:
	@ docker build -t roots-test-action actions/test
	@ docker run -v $(PWD):/github/workspace --rm -it roots-test-action run-tests

.PHONY: test-release
test-release:
	@ docker build -t roots-release-action actions/release
	@ docker run -e "GITHUB_REF=refs/tags/v0.0.0" -v $(PWD):/github/workspace --rm -it roots-release-action run-release --skip-publish --snapshot --rm-dist

.PHONY: release
release:
	@ docker build -t roots-release-action actions/release
	@ docker run -e GITHUB_TOKEN=$(GITHUB_TOKEN) -e GITHUB_REF=refs/tags/$(VERSION) -v $(PWD):/github/workspace --rm -it roots-release-action run-release --rm-dist
