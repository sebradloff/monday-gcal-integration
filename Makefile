.PHONY: help

default: help


help: ## Show this help
	@echo "monday-gcal-integration"
	@echo "======================"
	@echo
	@echo "cli tool to integrate Monday.com and Gcal"
	@echo
	@fgrep -h " ## " $(MAKEFILE_LIST) | fgrep -v fgrep | sed -Ee 's/([a-z.]*):[^#]*##(.*)/\1##\2/' | column -t -s "##"

build: ## build the binary
	go build -o mgint main.go

