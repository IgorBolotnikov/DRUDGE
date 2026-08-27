.PHONY: drg test

test:
	go test ./...

%:
	@:

drg:
	@go run main.go $(filter-out drg,$(MAKECMDGOALS))
