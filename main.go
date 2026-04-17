package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ethanefung/mail/internal/auth"
	"github.com/ethanefung/mail/internal/mailer"
	"github.com/ethanefung/mail/internal/provider"
)

func main() {
	// Fail fast on a misbuilt binary.
	_, _ = auth.MustLoad()

	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "list":
		if err := runList(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "mail:", err)
			os.Exit(1)
		}
	case "-h", "--help", "help":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "mail: unknown command %q\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(2)
	}
}

func usage(w *os.File) {
	fmt.Fprintln(w, "usage: mail list [--unread] [--limit=N] [--page=N] <email>")
}

func runList(args []string) error {
	fs := flag.NewFlagSet("mail list", flag.ContinueOnError)
	unread := fs.Bool("unread", false, "show only unread messages")
	limit := fs.Int("limit", 20, "number of messages per page")
	page := fs.Int("page", 1, "page number (1-indexed)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *limit < 1 {
		return fmt.Errorf("--limit must be >= 1")
	}
	if *page < 1 {
		return fmt.Errorf("--page must be >= 1")
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("missing email argument\nusage: mail list [--unread] [--limit=N] [--page=N] <email>")
	}
	email := strings.TrimSpace(fs.Arg(0))

	prov, err := provider.Lookup(email)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	tok, err := auth.GetToken(ctx, email, prov)
	if err != nil {
		return err
	}

	headers, err := mailer.ListInbox(ctx, email, prov, tok, *limit, *page, *unread)
	if err != nil {
		return err
	}

	for _, h := range headers {
		fmt.Printf("%d\t%s\t%s\t%s\n",
			h.UID,
			mailer.Sanitize(h.From),
			mailer.Sanitize(h.Subject),
			h.Date.Format(time.RFC3339),
		)
	}
	return nil
}
