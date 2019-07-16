.PHONY: docker

TAG_VERSION ?= latest
TAG ?= kscout/auto-cluster:${TAG_VERSION}

# build, tag, and push docker image
docker:
	docker build -t ${TAG} .
	docker push ${TAG}
