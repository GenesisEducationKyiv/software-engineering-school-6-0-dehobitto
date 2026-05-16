.PHONY: test test-unit test-integration test-e2e

test: test-unit test-integration test-e2e

test-unit:
	go test ./...

test-integration:
	go test -tags integration ./tests/integration/...

test-e2e:
	docker build -f tests/e2e/Dockerfile -t subber-e2e .
	docker run --rm subber-e2e
