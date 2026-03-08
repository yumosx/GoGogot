BIN   := gogogot
IMAGE := octagonlab/gogogot
TAG   := latest

.PHONY: run build clean lint docker-build docker-push docker-release deploy

run:
	go run ./cmd $(ARGS)

build:
	go build -o $(BIN) ./cmd

clean:
	rm -f $(BIN)

lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run ./...

docker-build:
	docker buildx build --platform linux/amd64,linux/arm64 \
		-t $(IMAGE):$(TAG) -f deploy/Dockerfile .

docker-push:
	docker push $(IMAGE):$(TAG)

docker-release:
	docker buildx build --platform linux/amd64,linux/arm64 \
		-t $(IMAGE):$(TAG) -f deploy/Dockerfile --push .

deploy:
	./deploy/deploy.sh
