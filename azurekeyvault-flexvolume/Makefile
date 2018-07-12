binary := azurekeyvault-flexvolume

VERSION          := v0.0.2

.PHONY: build
build: authors deps
	@echo "Building..."
	$Q GOOS=linux CGO_ENABLED=0 go build . 
	$Q mv $(binary) ../deployment/flexvol-installer/

.PHONY: clean deps

deps: setup
	@echo "Ensuring Dependencies..."
	$Q go env
	$Q dep ensure

clean:
	@echo "Clean..."
	$Q rm -rf $(binary)
	$Q rm -rf ../deployment/flexvol-installer/$(binary)

setup: clean
	@echo "Setup..."
	go get -u github.com/golang/dep/cmd/dep

authors:
	$Q git log --all --format='%aN <%cE>' | sort -u  | sed -n '/github/!p' > GITAUTHORS
	$Q cat AUTHORS GITAUTHORS  | sort -u > NEWAUTHORS
	$Q mv NEWAUTHORS AUTHORS
	$Q rm -f NEWAUTHORS
	$Q rm -f GITAUTHORS