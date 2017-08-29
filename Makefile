all: test

run:
	go build && ./vbox_ec2_md

deps:
	go get -v github.com/Masterminds/glide github.com/golang/lint/golint
	cd $(GOPATH)/src/github.com/Masterminds/glide && git checkout v0.12.3 && go install && cd -
	glide install

test:
	mkdir -p build
	go vet -n
	golint
	go test -v -race -timeout 1s -covermode=atomic -coverprofile=build/coverage.out
	go tool cover -func=build/coverage.out

codecov:
	./tools/codecov.sh -f build/coverage.out
