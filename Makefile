WINDOW_NAME := $(shell tmux list-windows -F "#{window_name}" | grep -w "^mpcium$$")


generate-id:
	go run cmd/generate-id/main.go
genmock:
	mockgen -source=./pkg/messaging/point2point.go -destination=artifacts/mocks/point2point.go -package=mock

run:
	go run cmd/main.go

build:
	go build -o tmp/main cmd/main.go

node0: 
	pm2 start ./tmp/main --name=mpcium0 -- --name=node0
node1:
	pm2 start ./tmp/main --name=mpcium1 -- --name=node1
node3:
	pm2 start ./tmp/main --name=mpcium2 -- --name=node2

node0-prod: 
	pm2 start ./tmp/main --name=mpcium0-prod -- --name=node0-prod
node1-prod:
	pm2 start ./tmp/main --name=mpcium1-prod -- --name=node1-prod
node2-prod:
	pm2 start ./tmp/main --name=mpcium2-prod -- --name=node2-prod
clean:

ifeq ($(WINDOW_NAME),mpcium)
	tmux kill-window -t mpcium;
endif

new:
	make clean && tmuxifier load-window mpcium

