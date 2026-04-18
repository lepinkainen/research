.PHONY: build dev test clean docker up down

build:
	go build -o anki .

dev:
	go run .

test:
	go test ./...

clean:
	rm -f anki anki.db

docker:
	docker build -t anki .

up:
	docker-compose up -d

down:
	docker-compose down
