# Les commandes du quotidien. Tout passe par `go`, sans dépendance externe.

BINAIRE := hippo
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
PAQUETS := ./...

.PHONY: aide
aide:
	@echo "compile      construit ./$(BINAIRE)"
	@echo "test         lance les tests avec le détecteur de courses"
	@echo "couverture   détail de couverture par paquet"
	@echo "vet          analyse statique"
	@echo "format       met le code au format"
	@echo "verifie      ce que la CI va faire"
	@echo "captures     régénère docs/captures/"
	@echo "man          régénère les pages de manuel"
	@echo "propre       supprime les artefacts de construction"
	@echo "installer    installe le binaire dans GOBIN"

.PHONY: compile
compile:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BINAIRE) ./cmd/$(BINAIRE)

.PHONY: installer
installer:
	go install ./cmd/$(BINAIRE)

.PHONY: test
test:
	go test -race $(PAQUETS)

.PHONY: couverture
couverture:
	go test -coverprofile=couverture.out $(PAQUETS)
	go tool cover -func=couverture.out | tail -1
	go tool cover -html=couverture.out -o couverture.html
	@echo "couverture.html écrit"

.PHONY: vet
vet:
	go vet $(PAQUETS)

.PHONY: format
format:
	gofmt -w .
	gofmt -l . | (! grep .)

.PHONY: verifie
verifie: format vet test

.PHONY: captures
captures: compile
	./$(BINAIRE) capture
	@echo "docs/captures/ régénéré"

.PHONY: man
man: compile
	./$(BINAIRE) man --dans docs/man

.PHONY: propre
propre:
	rm -f $(BINAIRE) couverture.out couverture.html
	go clean -testcache