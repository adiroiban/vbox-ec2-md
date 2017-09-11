all: test

run:
	go build && ./vbox_ec2_md

get-deps:
	go get -v github.com/Masterminds/glide github.com/golang/lint/golint
	cd $(GOPATH)/src/github.com/Masterminds/glide && git checkout v0.12.3 && go install && cd -
	glide install

test:
	mkdir -p build
	go vet -n
	golint
	go test -v -race -timeout 1s -covermode=atomic -coverprofile=build/coverage.txt
	go tool cover -func=build/coverage.txt

codecov:
	# Append an empty first line to the report as discussed in the
	# codecov example from go.
	# http://stackoverflow.com/a/21142256/2055281
	echo '' | cat /dev/stdin build/coverage.txt > build/coverage.codecov
	./tools/codecov.sh -f build/coverage.codecov
