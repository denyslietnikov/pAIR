APP=$(shell basename $(shell git remote get-url origin))
REGISTRY=ghcr.io/denyslietnikov
VERSION=$(shell git describe --tags --abbrev=0)-$(shell git rev-parse --short HEAD)
TARGETOS=linux
TARGETARCH=arm
TARGETARM=7

format:
		gofmt -s -w ./

lint:
		golint

test:
		go test -v
get:
		go get

build: format get
		CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETARM} go build -v -o pair

image:
		docker build --push -t ${REGISTRY}/${APP}:${VERSION}-linux-arm .

push:
		docker push ${REGISTRY}/${APP}:${VERSION}-${TARGETOS}-${TARGETARCH}

clean:
		rm -rf pair
		docker rmi ${REGISTRY}/${APP}:${VERSION}-${TARGETOS}-${TARGETARCH}|| true