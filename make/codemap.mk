# Code map: package-level routing index for AI agents.
# Regenerates .ai/codemap.md from the current source tree.
CODEMAP_OUT ?= .ai/codemap.md

.PHONY: codemap check-codemap

# Generate the code map.
codemap:
	go run ./hack/codemap -out $(CODEMAP_OUT)

# Fail if the code map is stale (for CI drift detection).
check-codemap:
	@go run ./hack/codemap -out $(CODEMAP_OUT)
	@git diff --exit-code -- $(CODEMAP_OUT) \
		|| (echo "ERROR: $(CODEMAP_OUT) is stale. Run 'make codemap' and commit." >&2; exit 1)
