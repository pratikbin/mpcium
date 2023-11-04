generate-id:
	go run cmd/generate-id/main.go
genmock:
	mockgen -source=./pkg/messaging/point2point.go -destination=artifacts/mocks/point2point.go -package=mock

clean: 
	rm -rf tmp
