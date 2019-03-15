

build_arm: clean
	GOARCH=arm GOARM=5 GOOS=linux  go build -o server_gin.arm examples/server_gin.go

build_x86: clean
	go build -o server_gorilla examples/server_gorilla.go
	go build -o server_gin examples/server_gin.go

clean:
	@rm -f server_gorilla
	@rm -f server_gin