.PHONY: deploy rm-deploy \
	docker docker-build docker-push

MAKE ?= make

ORG ?= kscout
APP ?= auto-cluster

DOCKER_VERSION ?= ${ENV}-latest
DOCKER_TAG ?= ${ORG}/${APP}:${DOCKER_VERSION}

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
		${SET_ARGS} deploy \
	| ${KUBE_APPLY}
	oc rollout status "dc/${ENV}-${APP}"

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

# build and push docker image
docker:
	@if [ -eq "$LOGIN" "true" ]; then echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_USERNAME" --password-stdin &> /dev/null
	${MAKE} docker-build
	${MAKE} docker-push

# build docker image
docker-build:
	@if [ -z "${ENV}" ]; then echo "ENV must be set"; exit 1; fi
	docker build -t ${DOCKER_TAG} .

# push docker image
docker-push:
	@if [ -z "${ENV}" ]; then echo "ENV must be set"; exit 1; fi
	docker push ${DOCKER_TAG}
