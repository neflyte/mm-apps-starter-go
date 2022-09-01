# Makefile for mm-apps-starter-go
#

BACKEND_DOCKER_COMPOSE_FILE="testdata/backend/docker-compose.yaml"

.PHONY: dist
dist:
	mkdir -p dist
	CGO_ENABLED=0 go build -ldflags "-s -w" -o dist/mm-apps-starter-go

.PHONY: clean
clean:
	if [ -f dist/mm-apps-starter-go ]; then rm -f dist/mm-apps-starter-go; fi

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: start-server
start-server:
	docker-compose -f $(BACKEND_DOCKER_COMPOSE_FILE) up -d

.PHONY: stop-server
stop-server:
	docker-compose -f $(BACKEND_DOCKER_COMPOSE_FILE) stop

.PHONY: clean-server
clean-server:
	docker-compose -f $(BACKEND_DOCKER_COMPOSE_FILE) down -v --remove-orphans

.PHONY: logs
logs:
	docker-compose -f $(BACKEND_DOCKER_COMPOSE_FILE) logs -f mm-apps-starter-go

.PHONY: logs-server
logs-server:
	docker-compose -f $(BACKEND_DOCKER_COMPOSE_FILE) logs -f mattermost
