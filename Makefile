.PHONY: build test lint run clean

build:
	cd web && npm run build
	cd server && go build -o ../bin/muddys ./cmd/server

test:
	cd server && go test ./...
	cd web && npm run build

lint:
	cd server && go vet ./...
	cd web && npm run lint

run: build
	./bin/muddys -data-dir=./data

clean:
	rm -rf bin server/internal/webui/dist/assets
	git checkout -- server/internal/webui/dist
