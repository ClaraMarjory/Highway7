VERSION := 0.1.0
BINARY := highway
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build clean

build:
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(BINARY) ./cmd/highway/

linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-linux-amd64 ./cmd/highway/

clean:
	rm -f $(BINARY) $(BINARY)-linux-amd64

run: build
	./$(BINARY) -port 8888 -pass admin123
