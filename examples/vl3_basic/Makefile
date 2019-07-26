NAME = vl3_basic
CONTAINERS = vl3-nse
PODS = simple-client vl3-nse
CHECK = export ITERATIONS=10 && export BATCHES=1 && scripts/check_vl3.sh

include $(TOP)/mk/targets.mk

# If GOPATH is made up of several paths, use the first one for our targets in this Makefile
GO_TOP := $(shell echo ${GOPATH} | cut -d ':' -f1)
export GO_TOP
export OUT_DIR=$(GO_TOP)/out
BUILDTYPE_DIR:=debug
export VL3_OUT_LINUX:=$(OUT_DIR)/linux_amd64/$(BUILDTYPE_DIR)
# scratch dir for building isolated images
DOCKER_BUILD_TOP:=${VL3_OUT_LINUX}/docker_build

build-vl3: ${GOPATH}/bin/linux_amd64/vl3_nse
	GO111MODULE=on CGO_ENABLED=0 GOOS=linux go build -ldflags '-extldflags "-static"' -o ${GOPATH}/bin/linux_amd64/vl3_nse ./examples/vl3_basic/vl3-nse/cmd/...

.PHONY: docker-vl3
docker-vl3: build-vl3
	mkdir -p ${DOCKER_BUILD_TOP}/vl3
	cp -p ./examples/vl3_basic/vl3-nse/Dockerfile.no_go_build ${DOCKER_BUILD_TOP}/vl3/
	cp -p ${GOPATH}/bin/linux_amd64/vl3_nse ${DOCKER_BUILD_TOP}/vl3/
	cd ${DOCKER_BUILD_TOP}/vl3 && docker build -t ${ORG}/vl3_basic-vl3-nse -f Dockerfile.no_go_build .
