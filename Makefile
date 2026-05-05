.PHONY: build install clean

build:
	go build -o ehaul .

install:
	go install .

clean:
	rm -f ehaul
