
PWD := $(shell pwd)
GOENV := $(shell go env GOPATH)
GOBIN := $(GOENV)/bin
GOKRAZY := $(PWD)/gokrazy

test:
	./scripts/test.sh

run:
	go run . --port=/dev/tty.usbserial-FTDJA4U0

run_bin:
	go run . --file=meet.bin

web:
	cd js && npm run dev

setup:
	go install github.com/gokrazy/tools/cmd/gok@main
	mkdir -p $(GOKRAZY)
	$(GOBIN)/gok new -i cstapi --parent_dir=$(GOKRAZY)


dump_usb:
	python3 -m serial.tools.miniterm /dev/tty.usbserial-FTDJA4U0 9600