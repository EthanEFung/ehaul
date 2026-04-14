-include .credentials
export GMAIL_CLIENT_ID GMAIL_CLIENT_SECRET

LDFLAGS := -X 'github.com/ethanefung/mail/internal/auth.clientID=$(GMAIL_CLIENT_ID)' \
           -X 'github.com/ethanefung/mail/internal/auth.clientSecret=$(GMAIL_CLIENT_SECRET)'

.PHONY: build clean

build:
	@if [ ! -f .credentials ]; then \
		echo "ERROR: .credentials not found."; \
		echo "       Copy .credentials.example to .credentials and populate values."; \
		exit 1; \
	fi
	@if [ -z "$(GMAIL_CLIENT_ID)" ] || [ -z "$(GMAIL_CLIENT_SECRET)" ]; then \
		echo "ERROR: .credentials is missing GMAIL_CLIENT_ID or GMAIL_CLIENT_SECRET."; \
		exit 1; \
	fi
	go build -ldflags "$(LDFLAGS)" -o mail .

clean:
	rm -f mail
