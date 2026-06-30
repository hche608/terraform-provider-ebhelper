HOSTNAME=registry.terraform.io
NAMESPACE=hche608
NAME=ebhelper
BINARY=terraform-provider-${NAME}
VERSION=0.1.0
OS_ARCH=$(shell go env GOOS)_$(shell go env GOARCH)

default: build

build:
	go build -o ${BINARY}

install: build
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}
	cp ${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}/

test:
	go test ./... -count=1 -v

testacc:
	TF_ACC=1 go test ./... -count=1 -v -run TestAcc

clean:
	rm -f ${BINARY}

.PHONY: build install test testacc clean
