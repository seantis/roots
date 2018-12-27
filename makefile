.PHONY: test
test:
	go test ./...

.PHONY: test-all
test-action:
	docker build -t roots-test-action actions/test
	docker run -v $(PWD):/github/workspace --rm -it roots-test-action run-tests

.PHONY: test-release
test-release:
	docker build -t roots-release-action actions/release
	docker run -e "GITHUB_REF=refs/tags/v0.0.0" -v $(PWD):/github/workspace --rm -it roots-release-action run-release --skip-publish --snapshot --rm-dist


.PHONY: release
test-release:
	docker build -t roots-release-action actions/release
	docker run -e "GITHUB_REF=refs/tags/v0.0.0" -v $(PWD):/github/workspace --rm -it roots-release-action run-release --skip-publish --snapshot --rm-dist
