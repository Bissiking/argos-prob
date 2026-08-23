APP_NAME   := argos-prob
VERSION    ?= $(shell grep -oP 'Version\s*=\s*"\K[^"]+' internal/version/version.go 2>/dev/null || echo "1.2.0")
MODULE     := github.com/Bissiking/argos-prob
BUILD_DIR  := dist
CMD_DIR    := cmd/argos-prob

PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

LDFLAGS := -s -w -X $(MODULE)/internal/version.Version=$(VERSION)

# ──────────────────────────────────────────────
# Build
# ──────────────────────────────────────────────

.PHONY: build
build: ## Build for the current platform
	go build -ldflags "$(LDFLAGS)" -o $(APP_NAME) ./$(CMD_DIR)

.PHONY: build-all
build-all: $(addprefix build-,$(subst /,_,$(PLATFORMS))) ## Build for all platforms

build-linux_amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 ./$(CMD_DIR)

build-linux_arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64 ./$(CMD_DIR)

build-darwin_amd64:
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 ./$(CMD_DIR)

build-darwin_arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-darwin-arm64 ./$(CMD_DIR)

build-windows_amd64:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe ./$(CMD_DIR)

# ──────────────────────────────────────────────
# Archives (tar.gz / zip)
# ──────────────────────────────────────────────

.PHONY: archive
archive: $(addprefix archive-,$(subst /,_,$(PLATFORMS))) ## Create archives for all platforms

archive-linux_amd64: build-linux_amd64
	cd $(BUILD_DIR) && tar czf $(APP_NAME)-$(VERSION)-linux-amd64.tar.gz $(APP_NAME)-linux-amd64
	@echo " -> $(BUILD_DIR)/$(APP_NAME)-$(VERSION)-linux-amd64.tar.gz"

archive-linux_arm64: build-linux_arm64
	cd $(BUILD_DIR) && tar czf $(APP_NAME)-$(VERSION)-linux-arm64.tar.gz $(APP_NAME)-linux-arm64
	@echo " -> $(BUILD_DIR)/$(APP_NAME)-$(VERSION)-linux-arm64.tar.gz"

archive-darwin_amd64: build-darwin_amd64
	cd $(BUILD_DIR) && tar czf $(APP_NAME)-$(VERSION)-darwin-amd64.tar.gz $(APP_NAME)-darwin-amd64
	@echo " -> $(BUILD_DIR)/$(APP_NAME)-$(VERSION)-darwin-amd64.tar.gz"

archive-darwin_arm64: build-darwin_arm64
	cd $(BUILD_DIR) && tar czf $(APP_NAME)-$(VERSION)-darwin-arm64.tar.gz $(APP_NAME)-darwin-arm64
	@echo " -> $(BUILD_DIR)/$(APP_NAME)-$(VERSION)-darwin-arm64.tar.gz"

archive-windows_amd64: build-windows_amd64
	cd $(BUILD_DIR) && zip -j $(APP_NAME)-$(VERSION)-windows-amd64.zip $(APP_NAME)-windows-amd64.exe
	@echo " -> $(BUILD_DIR)/$(APP_NAME)-$(VERSION)-windows-amd64.zip"

# ──────────────────────────────────────────────
# DEB packages
# ──────────────────────────────────────────────

.PHONY: package-deb
package-deb: build-linux_amd64 build-linux_arm64 ## Create .deb packages (amd64 + arm64)
	@bash packaging/build-deb.sh $(BUILD_DIR) $(VERSION) amd64
	@bash packaging/build-deb.sh $(BUILD_DIR) $(VERSION) arm64

.PHONY: package-deb-amd64
package-deb-amd64: build-linux_amd64 ## Create .deb package (amd64 only)
	@bash packaging/build-deb.sh $(BUILD_DIR) $(VERSION) amd64

.PHONY: package-deb-arm64
package-deb-arm64: build-linux_arm64 ## Create .deb package (arm64 only)
	@bash packaging/build-deb.sh $(BUILD_DIR) $(VERSION) arm64

# ──────────────────────────────────────────────
# RPM packages
# ──────────────────────────────────────────────

.PHONY: package-rpm
package-rpm: build-linux_amd64 build-linux_arm64 ## Create .rpm packages (requires rpmbuild)
	@bash packaging/build-rpm.sh $(BUILD_DIR) $(VERSION) amd64
	@bash packaging/build-rpm.sh $(BUILD_DIR) $(VERSION) arm64

.PHONY: package-rpm-amd64
package-rpm-amd64: build-linux_amd64 ## Create .rpm package (amd64 only)
	@bash packaging/build-rpm.sh $(BUILD_DIR) $(VERSION) amd64

.PHONY: package-rpm-arm64
package-rpm-arm64: build-linux_arm64 ## Create .rpm package (arm64 only)
	@bash packaging/build-rpm.sh $(BUILD_DIR) $(VERSION) arm64

# ──────────────────────────────────────────────
# MSI packages (Windows)
# ──────────────────────────────────────────────

.PHONY: package-msi
package-msi: build-windows_amd64 ## Create .msi installer (requires wixl from msitools)
	@bash packaging/build-msi.sh $(BUILD_DIR) $(VERSION)

# ──────────────────────────────────────────────
# All
# ──────────────────────────────────────────────

.PHONY: package-all
package-all: package-deb package-rpm package-msi ## Create all packages

.PHONY: dist
dist: build-all archive-all package-all ## Full distribution build

# ──────────────────────────────────────────────
# Clean
# ──────────────────────────────────────────────

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)
	rm -f $(APP_NAME)

# ──────────────────────────────────────────────
# Help
# ──────────────────────────────────────────────

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
