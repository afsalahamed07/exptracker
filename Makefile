GO ?= go
PYTHON ?= python3
DIST_DIR ?= dist/lambda
LAMBDA_ARCH ?= arm64

.PHONY: fmt test vet check build package

fmt:
	$(GO) fmt ./...

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

check: vet test

build:
	mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=$(LAMBDA_ARCH) CGO_ENABLED=0 $(GO) build -o $(DIST_DIR)/bootstrap .

package: build
	$(PYTHON) -c 'import pathlib, zipfile; dist = pathlib.Path("$(DIST_DIR)"); archive = dist / "function.zip"; zf = zipfile.ZipFile(archive, "w", compression=zipfile.ZIP_DEFLATED); zf.write(dist / "bootstrap", arcname="bootstrap"); zf.close(); print("wrote", archive)'
