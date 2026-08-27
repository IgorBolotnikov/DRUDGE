.PHONY: drg

%:
	@:

drg:
	@go run main.go $(filter-out drg,$(MAKECMDGOALS))
