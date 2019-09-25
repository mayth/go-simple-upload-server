build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o server

darwin:
	go build -o server

docker: build
	docker build -t surenpi/simple-server -f Dockerfile.simple .

push: docker
	docker push surenpi/simple-server:latest

test:
	docker run -p 5678:25478 surenpi/simple-server