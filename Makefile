.PHONY: dev test build lint compose-up compose-down helm

dev:
	docker compose up --build

test:
	go test ./...
	cd web && npm test --if-present && npm run lint

build:
	go build ./cmd/suppervisor
	cd web && npm run build

lint:
	go vet ./...
	cd web && npm run lint

compose-up:
	docker compose up --build

compose-down:
	docker compose down

helm:
	helm lint deploy/helm/suppervisor --strict
	helm template suppervisor deploy/helm/suppervisor >/dev/null
