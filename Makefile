all: test

run:
	go build && ./vbox_ec2_md

test:
	mkdir -p build
	go vet -n
	golint
	go test -v -timeout 1s -covermode=count -coverprofile=build/coverage.out
	go tool cover -func=build/coverage.out
