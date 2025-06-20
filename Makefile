.PHONY: build clean install uninstall reinstall release test test-verbose test-coverage test-race help

help:
	@echo "Usage:"
	@echo "  make build   - Build the monres executable"
	@echo "  make clean   - Clean up build artifacts"
	@echo "  make test    - Run all tests"
	@echo "  make test-verbose - Run tests with verbose output"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make test-race - Run tests with race detection"
	@echo "  make install - Install monres and its systemd service"
	@echo "  make uninstall - Uninstall monres and remove user/group"
	@echo "  make reinstall - Reinstall monres"
	@echo "  make release name=<release_name> - Create a release with the specified name"
build:
	go build -ldflags="-s -w" -o monres cmd/monres/main.go
	@echo "Build complete. Executable: ./monres"
clean:
	rm -f monres coverage.out coverage.html
	@echo "Cleaned up build artifacts."
create-user:
	sudo useradd -r -s /sbin/nologin monres
	sudo mkdir -p /etc/monres
	sudo chown root:monres /etc/monres
	sudo chmod 750 /etc/monres
	sudo cp config.example.yaml /etc/monres/config.yaml
	sudo chown root:monres /etc/monres/config.yaml
	sudo chmod 640 /etc/monres/config.yaml
	sudo touch /etc/monres/monres.env
	sudo chown monres:monres /etc/monres/monres.env
	sudo chmod 600 /etc/monres/monres.env
	@echo "User and group 'monres' created."
	@echo "Configuration file created at /etc/monres/config.yaml"
	@echo "Environment file created at /etc/monres/monres.env"
del-user:
	sudo userdel -r monres
	sudo rm -rf /etc/monres
	@echo "User and group 'monres' deleted, along with configuration files."
install: build create-user
	sudo mv monres /usr/local/bin
	sudo cp deploy/systemd/monres.service /etc/systemd/system/
	sudo systemctl daemon-reload
uninstall: clean del-user
	sudo rm -f /usr/local/bin/monres
	sudo rm -f /etc/systemd/system/monres.service
	sudo systemctl daemon-reload
	@echo "Monres uninstalled successfully."
reinstall: uninstall install
	@echo "Monres reinstalled successfully."
test:
	go test ./...
	@echo "All tests passed."
test-verbose:
	go test -v ./...
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
test-race:
	go test -race ./...
	@echo "Race detection tests passed."
release:
	gh release create $(name) --title "$(name)" --generate-notes ./monres
	@echo "Release created successfully."
	git push --tags
	@echo "Changes pushed to remote repository."
