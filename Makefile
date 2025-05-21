build:
	go build -ldflags="-s -w" -o monres cmd/monres/main.go
install: build
	sudo mv monres /usr/local/bin
	sudo cp deploy/systemd/monres.service /etc/systemd/system/
	sudo systemctl daemon-reload
