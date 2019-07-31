.PHONY: deploy rm-deploy \
	container container-build container-push

MAKE ?= make

ORG ?= kscout
APP ?= auto-cluster

CONTAINER_BIN ?= podman

CONTAINER_VERSION ?= ${ENV}-latest
CONTAINER_TAG ?= quay.io/${ORG}/${APP}:${CONTAINER_VERSION}

KUBE_LABELS ?= app=${APP},env=${ENV}
KUBE_TYPES ?= $(shell grep -h -r kind deploy/charts | awk '{ print $2 }' | uniq | paste -sd "," -)
KUBE_APPLY ?= oc apply -f -

# deploy to ENV
deploy:
	@if [ -z "${ENV}" ]; then echo "ENV must be set"; exit 1; fi
	helm template \
		--values deploy/values.yaml \
		--values deploy/values.secrets.${ENV}.yaml \
		--set global.env=${ENV} \
		--set global.app=${APP} \
		--set image.tag=${CONTAINER_VERSION} \
		${SET_ARGS} deploy \
	| ${KUBE_APPLY}

# remove deployment for ENV
rm-deploy:
	@if [ -z "${ENV}" ]; then echo "ENV must be set"; exit 1; fi
	@echo "Remove ${ENV} ${APP} deployment"
	@echo "Hit any key to confirm"
	@read confirm
	oc get \
		--ignore-not-found \
		-l ${KUBE_LABELS} \
		${KUBE_TYPES} \
		-o yaml \
	| oc delete -f -

# build and push container image
container:
	${MAKE} container-build
	${MAKE} container-push

# build container image
container-build:
	@if [ -z "${ENV}" ]; then echo "ENV must be set"; exit 1; fi
	${CONTAINER_BIN} build -t ${CONTAINER_TAG} .

# push container image
container-push:
	@if [ -z "${ENV}" ]; then echo "ENV must be set"; exit 1; fi
	${CONTAINER_BIN} push ${CONTAINER_TAG}
