BIN := gogogot
COMPOSE := docker compose -f deploy/docker-compose.yml

.PHONY: run build clean lint up down logs deploy

run:
	go run ./cmd $(ARGS)

build:
	go build -o $(BIN) ./cmd

clean:
	rm -f $(BIN)

lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run ./...

up:
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f

deploy:
	./deploy/deploy.sh
