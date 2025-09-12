# Set these to the desired values
ARTIFACT_ID=k8s-ces-assets
ARTIFACT_ID_WARP=${ARTIFACT_ID}-warp
ARTIFACT_ID_MAINTENANCE=${ARTIFACT_ID}-maintenance
VERSION=0.0.2
IMAGE=cloudogu/${ARTIFACT_ID}:${VERSION}

MAKEFILES_VERSION=10.2.0

ADDITIONAL_CLEAN=dist-clean
MOCKERY_VERSION=v2.53.3

K8S_COMPONENT_SOURCE_VALUES = ${HELM_SOURCE_DIR}/values.yaml
K8S_COMPONENT_TARGET_VALUES = ${HELM_TARGET_DIR}/values.yaml
HELM_PRE_GENERATE_TARGETS = helm-values-update-image-version
HELM_POST_GENERATE_TARGETS = helm-values-replace-image-repo template-stage template-log-level template-image-pull-policy
IMAGE_IMPORT_TARGET=images-import

include build/make/variables.mk
include build/make/self-update.mk
include build/make/build.mk
include build/make/test-common.mk
include build/make/test-unit.mk
include build/make/static-analysis.mk
include build/make/clean.mk
include build/make/digital-signature.mk
include build/make/mocks.mk

include build/make/k8s-controller.mk
include build/make/k8s.mk

PACKAGES=$(shell go list ./warp/... ./maintenance/...)
IMAGE_DEV_WARP=$(CES_REGISTRY_HOST)$(CES_REGISTRY_NAMESPACE)/$(ARTIFACT_ID_WARP)/$(GIT_BRANCH)
IMAGE_DEV_MAINTENANCE=$(CES_REGISTRY_HOST)$(CES_REGISTRY_NAMESPACE)/$(ARTIFACT_ID_MAINTENANCE)/$(GIT_BRANCH)

##@ Deployment

.PHONY: template-stage
template-stage: $(BINARY_YQ)
	@if [[ ${STAGE} == "development" ]]; then \
  		echo "Setting STAGE env in deployment to ${STAGE}!" ;\
		$(BINARY_YQ) -i e ".nginx.env.stage=\"${STAGE}\"" ${K8S_COMPONENT_TARGET_VALUES} ;\
	fi

.PHONY: template-log-level
template-log-level: $(BINARY_YQ)
	@echo "Setting LOG_LEVEL env in deployment to ${LOG_LEVEL}!"
	@$(BINARY_YQ) -i e ".nginx.env.logLevel=\"${LOG_LEVEL}\"" ${K8S_COMPONENT_TARGET_VALUES}

.PHONY: template-image-pull-policy
template-image-pull-policy: $(BINARY_YQ)
	@if [[ ${STAGE} == "development" ]]; then \
  		echo "Setting PULL POLICY to always!" ;\
		$(BINARY_YQ) -i e ".nginx.imagePullPolicy=\"Always\"" ${K8S_COMPONENT_TARGET_VALUES} ;\
	fi

.PHONY: helm-values-update-image-version
helm-values-update-image-version: $(BINARY_YQ)
	@echo "Updating the image version in source value.yaml to ${VERSION}..."
	@$(BINARY_YQ) -i e ".nginx.manager.image.tag = \"${VERSION}\"" ${K8S_COMPONENT_SOURCE_VALUES}

.PHONY: helm-values-replace-image-repo
helm-values-replace-image-repo: $(BINARY_YQ)
	@if [[ ${STAGE} == "development" ]]; then \
		echo "Setting dev image repo in target value.yaml!" ;\
		$(BINARY_YQ) -i e ".nginx.manager.image.registry=\"$(shell echo '${IMAGE_DEV}' | sed 's/\([^\/]*\)\/\(.*\)/\1/')\"" ${K8S_COMPONENT_TARGET_VALUES} ;\
		$(BINARY_YQ) -i e ".nginx.manager.image.repository=\"$(shell echo '${IMAGE_DEV}' | sed 's/\([^\/]*\)\/\(.*\)/\2/')\"" ${K8S_COMPONENT_TARGET_VALUES} ;\
		echo "Setting warp dev image repo in target value.yaml!" ;\
		$(BINARY_YQ) -i e ".nginx.warp.image.registry=\"$(shell echo '${IMAGE_DEV_WARP}' | sed 's/\([^\/]*\)\/\(.*\)/\1/')\"" ${K8S_COMPONENT_TARGET_VALUES} ;\
		$(BINARY_YQ) -i e ".nginx.warp.image.repository=\"$(shell echo '${IMAGE_DEV_WARP}' | sed 's/\([^\/]*\)\/\(.*\)/\2/')\"" ${K8S_COMPONENT_TARGET_VALUES} ;\
		echo "Setting maintenance dev image repo in target value.yaml!" ;\
		$(BINARY_YQ) -i e ".nginx.maintenance.image.registry=\"$(shell echo '${IMAGE_DEV_MAINTENANCE}' | sed 's/\([^\/]*\)\/\(.*\)/\1/')\"" ${K8S_COMPONENT_TARGET_VALUES} ;\
		$(BINARY_YQ) -i e ".nginx.maintenance.image.repository=\"$(shell echo '${IMAGE_DEV_MAINTENANCE}' | sed 's/\([^\/]*\)\/\(.*\)/\2/')\"" ${K8S_COMPONENT_TARGET_VALUES} ;\
	fi

# Custom targets:

.PHONY: mocks
mocks: ${MOCKERY_BIN} ## target is used to generate mocks for all interfaces in a project.
	cd ${WORKDIR}/warp && ${MOCKERY_BIN}
	@echo "Mocks successfully created."
	cd ${WORKDIR}/maintenance && ${MOCKERY_BIN}
	echo "Mocks successfully created."

.PHONY: docker-build
docker-build: check-docker-credentials check-k8s-image-env-var ${BINARY_YQ} ## Builds the docker image of the K8s app.
	@echo "Building docker image $(IMAGE) in directory $(IMAGE_DIR)..."
	@DOCKER_BUILDKIT=1 docker build $(IMAGE_DIR) -t $(IMAGE)

.PHONY: images-import
images-import: ## import images from ces-importer and
	@echo "Import k8s-ces-assets image"
	@make image-import \
		IMAGE_DIR=.
	@echo "Import warp assets image"
	@make image-import \
		IMAGE_DIR=./warp \
		IMAGE=${ARTIFACT_ID_WARP}:${VERSION} \
		IMAGE_DEV_VERSION=$(CES_REGISTRY_HOST)$(CES_REGISTRY_NAMESPACE)/$(ARTIFACT_ID_WARP)/$(GIT_BRANCH):${VERSION}
	@echo "Import maintenance assets image"
	@make image-import \
		IMAGE_DIR=./maintenance \
		IMAGE=${ARTIFACT_ID_MAINTENANCE}:${VERSION} \
		IMAGE_DEV_VERSION=$(CES_REGISTRY_HOST)$(CES_REGISTRY_NAMESPACE)/$(ARTIFACT_ID_MAINTENANCE)/$(GIT_BRANCH):${VERSION}

.PHONY: vendor
vendor: # no prerequisites
	@echo "Installing dependencies using go modules..."
	${GO_CALL} work vendor


compile-generic:
	@echo "Compiling..."
# here is go called without mod capabilities because of error "go: error loading module requirements"
# see https://github.com/golang/go/issues/30868#issuecomment-474199640
	@$(GO_ENV_VARS) go build $(GO_BUILD_FLAGS)-warp ./warp
	@$(GO_ENV_VARS) go build $(GO_BUILD_FLAGS)-maintenance ./maintenance

$(STATIC_ANALYSIS_DIR)/static-analysis.log: $(STATIC_ANALYSIS_DIR)
	@echo ""
	@echo "complete static analysis:"
	@echo ""
	@$(LINT) $(LINTFLAGS) run ./warp/... ./maintenance/... $(ADDITIONAL_LINTER) > $@

$(STATIC_ANALYSIS_DIR)/static-analysis-cs.log: $(STATIC_ANALYSIS_DIR)
	@echo "run static analysis with export to checkstyle format"
	@$(LINT) $(LINTFLAGS) --output.checkstyle.path stdout run ./warp/... ./maintenance/... $(ADDITIONAL_LINTER) > $@
