.PHONY: run container dev

CONTAINER_VERSION ?= latest
CONTAINER_TAG ?= quay.io/kscout/auto-cluster:${CONTAINER_VERSION}

# run container
run:
	podman run -it --rm ${CONTAINER_TAG}

# container builds the auto cluster container
container:
	podman build -t ${CONTAINER_TAG} .

# build and run the container for local development
dev: container run
