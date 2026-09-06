.PHONY: drg test loc

test:
	go test ./...

loc:
	@printf 'Go production  %7s\n' "$$(find . -name '*.go' -not -name '*_test.go' -not -path './.git/*' -exec cat {} + | wc -l)"
	@printf 'Go tests       %7s\n' "$$(find . -name '*_test.go' -not -path './.git/*' -exec cat {} + | wc -l)"
	@printf 'Markdown       %7s\n' "$$(find . -name '*.md' -not -path './.git/*' -exec cat {} + | wc -l)"
	@echo
	@echo 'Production by package:'
	@for dir in $$(find internal -type d | sort); do \
		count=$$(find $$dir -maxdepth 1 -name '*.go' -not -name '*_test.go' -exec cat {} + | wc -l); \
		[ "$$count" -gt 0 ] && printf '%7s  %s\n' "$$count" "$$dir" || true; \
	done

%:
	@:

drg:
	@go run main.go $(filter-out drg,$(MAKECMDGOALS))
