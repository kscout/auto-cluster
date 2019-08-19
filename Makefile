.PHONY: run container

CONTAINER_VERSION ?= latest
CONTAINER_TAG ?= quay.io/kscout/auto-cluster:${CONTAINER_VERSION}

# run container
run:
	podman run -it --rm ${CONTAINER_TAG}

# container builds the auto cluster container
container: main.go
	podman build -t ${CONTAINER_TAG} .
