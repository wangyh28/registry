lite:
	go install ./...

all:	protos
	go install ./...

protos:
	cd third_party; sh ./SETUP.sh
	./tools/COMPILE-PROTOS.sh

test:
	go clean -testcache
	go test -v ./...

clean:
	rm -rf \
		cmd/apg/*.go gapic/*.go rpc/*.go \
		third_party/api-common-protos third_party/gnostic \
		envoy/proto.pb

build:
ifndef REGISTRY_PROJECT_IDENTIFIER
	@echo "Error! REGISTRY_PROJECT_IDENTIFIER must be set."; exit 1
endif
	gcloud builds submit --tag gcr.io/${REGISTRY_PROJECT_IDENTIFIER}/registry-backend

deploy:
ifndef REGISTRY_PROJECT_IDENTIFIER
	@echo "Error! REGISTRY_PROJECT_IDENTIFIER must be set."; exit 1
endif
	gcloud run deploy registry-backend --image gcr.io/${REGISTRY_PROJECT_IDENTIFIER}/registry-backend --platform managed

index:
	gcloud datastore indexes create index.yaml

index-cleanup:
	gcloud datastore indexes cleanup index.yaml

