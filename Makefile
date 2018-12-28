all: clean
	@go build -mod=vendor

clean:
	@go clean

test:
	@go test

