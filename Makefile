

build_arm:
	GOARCH=arm GOARM=5 GOOS=linux  go build

build_x86: clean
	go build -o server_gorilla examples/server_gorilla.go
	go build -o server_gin examples/server_gin.go

clean:
	@rm -f server_gorilla
	@rm -f server_gin