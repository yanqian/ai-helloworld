SHELL := /usr/bin/env bash

.PHONY: help init test validate smoke unit contract go-example work dry-run summarize clean ci

FEATURE ?= F001

help:
	@printf '%s\n' 'Targets:'
	@printf '  %-14s %s\n' 'init' 'run the full harness verification entry point'
	@printf '  %-14s %s\n' 'test' 'run init'
	@printf '  %-14s %s\n' 'validate' 'validate one feature with FEATURE=Fxxx'
	@printf '  %-14s %s\n' 'unit' 'run unit tests'
	@printf '  %-14s %s\n' 'contract' 'run contract tests'
	@printf '  %-14s %s\n' 'smoke' 'run smoke tests'
	@printf '  %-14s %s\n' 'go-example' 'run the Go server example tests'
	@printf '  %-14s %s\n' 'work' 'run one orchestrator round for the next unfinished feature'
	@printf '  %-14s %s\n' 'dry-run' 'preview the next orchestrator round'
	@printf '  %-14s %s\n' 'summarize' 'print progress and run summaries'
	@printf '  %-14s %s\n' 'clean' 'reset project-specific harness state for a fresh project'
	@printf '  %-14s %s\n' 'ci' 'run CI verification'

init:
	./init.sh

test: init

validate:
	scripts/validate-feature.sh $(FEATURE)

unit:
	python3 -m unittest discover -s test/unit -p 'test_*.py'

contract:
	python3 -m unittest discover -s test/contract -p 'test_*.py'

smoke:
	python3 -m unittest discover -s test/smoke -p 'test_*.py'

go-example:
	mkdir -p "$${HARNESS_GOCACHE:-$${TMPDIR:-/tmp}/ai-agent-harness-go-build}"
	cd examples/go-server && GOCACHE="$${HARNESS_GOCACHE:-$${TMPDIR:-/tmp}/ai-agent-harness-go-build}" go test ./...

work:
	python3 orchestrator.py --max-rounds 1

dry-run:
	python3 orchestrator.py --dry-run

summarize:
	scripts/summarize-progress.sh
	scripts/summarize-runs.sh

clean:
	python3 scripts/clean-state.py

ci:
	$(MAKE) init
	$(MAKE) validate FEATURE=F001
	$(MAKE) dry-run
