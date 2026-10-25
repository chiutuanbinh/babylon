SHELL := /bin/bash
PG    ?= pg
GO_WANT := $(shell awk '/^go /{print $$2}' go.mod)

.DEFAULT_GOAL := help
.PHONY: help setup check run test test-db test-sim vet pg pg-stop

help: ## list targets
	@awk -F':.*## ' '/^[a-z-]+:.*## /{printf "  make %-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

setup: ## first-time setup (idempotent): hooks, modules, .env, postgres, then check
	git config core.hooksPath .githooks
	go mod download
	@[ -f .env ] || { cp .env.example .env; echo ".env created: fill in GITHUB_CLIENT_ID/SECRET"; }
	@$(MAKE) --no-print-directory pg
	-container exec $(PG) createdb -U postgres babylon_test
	@$(MAKE) --no-print-directory check

pg: ## start local postgres container (creates it if missing)
	-container system start
	@if container ls -a | awk '{print $$1}' | grep -qx $(PG); then container start $(PG); \
	else container run -d --name $(PG) -e POSTGRES_PASSWORD=pg -e POSTGRES_DB=babylon -p 5432:5432 postgres:17; fi
	@for i in $$(seq 20); do container exec $(PG) pg_isready -U postgres >/dev/null 2>&1 && break; sleep 1; done

pg-stop: ## stop postgres container
	container stop $(PG)

# step LABEL, CMD: print ok/FAIL and remember failure in a marker file
# (portable: macOS ships GNU make 3.81, which has no .ONESHELL).
FAILED := .check-failed
step = @if { $(2); } >/dev/null 2>&1; then printf '  ok    %s\n' "$(1)"; else printf '  FAIL  %s\n' "$(1)"; touch $(FAILED); fi

check: ## verify toolchain, modules, postgres, .env, hooks, vet (read-only)
	@rm -f $(FAILED)
	@echo toolchain
	$(call step,go >= $(GO_WANT),[ "$$(printf '%s\n%s\n' go$(GO_WANT) "$$(go env GOVERSION)" | sort -V | head -1)" = go$(GO_WANT) ])
	$(call step,node >= 24,[ "$$(node -p 'process.versions.node.split(".")[0]')" -ge 24 ])
	$(call step,container CLI,command -v container)
	@echo modules
	$(call step,go mod verify,go mod verify)
	@echo services
	$(call step,container '$(PG)' running (make pg),container ls | awk '{print $$1}' | grep -qx $(PG))
	$(call step,postgres reachable :5432,exec 3<>/dev/tcp/127.0.0.1/5432)
	@echo config
	$(call step,.env present (cp .env.example .env),[ -f .env ])
	$(call step,DATABASE_URL set in .env,grep -Eq '^(export )?DATABASE_URL=.+' .env)
	$(call step,GITHUB_CLIENT_ID set in .env,grep -Eq '^(export )?GITHUB_CLIENT_ID=.+' .env)
	$(call step,GITHUB_CLIENT_SECRET set in .env,grep -Eq '^(export )?GITHUB_CLIENT_SECRET=.+' .env)
	$(call step,BASE_URL set in .env,grep -Eq '^(export )?BASE_URL=.+' .env)
	$(call step,git hooks enabled,[ "$$(git config core.hooksPath)" = .githooks ])
	@echo build
	$(call step,go vet,go vet ./...)
	@if [ -f $(FAILED) ]; then rm -f $(FAILED); exit 1; fi

run: ## serve on :8080
	go run .

test: ## all Go tests
	go test ./...

test-db: ## postgres integration tests (needs make pg)
	TEST_DATABASE_URL=postgres://postgres:pg@localhost:5432/babylon_test go test ./internal/adapters/postgres/ -v

test-sim: ## simulator front-end tests
	cd internal/adapters/sim && node --test

vet: ## go vet
	go vet ./...
