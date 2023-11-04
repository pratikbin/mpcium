WINDOW_NAME := $(shell tmux list-windows -F "#{window_name}" | grep "mpcium")


generate-id:
	go run cmd/generate-id/main.go
genmock:
	mockgen -source=./pkg/messaging/point2point.go -destination=artifacts/mocks/point2point.go -package=mock

run:
	go run cmd/main.go

build:
	go build -o tmp/main cmd/main.go


clean:
ifeq ($(WINDOW_NAME),mpcium)
	tmux kill-window -t mpcium;
endif

new:
	make clean && tmuxifier load-window mpcium

