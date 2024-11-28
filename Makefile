##run app
run:
	@go run main.go

##build app
build:
	@go build -o bin/main main.go

##test app
test:
	@go test -v ./...

##clean app
clean:
	@rm -rf bin