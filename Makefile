setup:
	go get -t && npm install
tests:
	go test -v ./... && npm test
run:
	go run geobin.go config.go handlers.go geobinrequest.go util.go socket.go socketmap.go middleware.go
debug:
	go build -o debug.out && ./debug.out -debug=true
tar:
	cd build && go run build.go
