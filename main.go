package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"

	"github.com/ethanefung/mail/internal/auth"
	"github.com/ethanefung/mail/internal/cache"
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
	case "flag":
		if err := runFlag(os.Args[2:]); err != nil {
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
	fmt.Fprintln(w, "       mail flag <email> <operation> <flag> <uid...>")
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

	cacheDir, cacheDirErr := cache.Dir()
	if cacheDirErr != nil {
		fmt.Fprintf(os.Stderr, "mail: warning: cache dir: %v\n", cacheDirErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	tok, err := auth.GetToken(ctx, email, prov)
	if err != nil {
		return err
	}

	headers, uidValidity, err := mailer.ListInbox(ctx, email, prov, tok, *limit, *page, *unread)
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

	if uidValidity > 0 && cacheDirErr == nil {
		if err := cache.SaveUIDValidity(cacheDir, email, "INBOX", uidValidity); err != nil {
			fmt.Fprintf(os.Stderr, "mail: warning: save uidvalidity: %v\n", err)
		}
	}

	return nil
}

func runFlag(args []string) error {
	fs := flag.NewFlagSet("mail flag", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 4 {
		return fmt.Errorf("not enough arguments\nusage: mail flag <email> <operation> <flag> <uid...>")
	}

	email := strings.TrimSpace(fs.Arg(0))
	op := fs.Arg(1)
	flagAlias := fs.Arg(2)

	// Validate operation before any network call.
	switch op {
	case "add", "rm", "remove", "set":
	default:
		return fmt.Errorf("unknown operation %q (must be add, rm, remove, or set)", op)
	}

	// Parse UID arguments.
	uidArgs := fs.Args()[3:]
	uids := make([]imap.UID, 0, len(uidArgs))
	for _, arg := range uidArgs {
		n, err := strconv.ParseUint(arg, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid UID %q: %w", arg, err)
		}
		if n == 0 {
			return fmt.Errorf("invalid UID %q: IMAP UIDs must be greater than 0", arg)
		}
		uids = append(uids, imap.UID(uint32(n)))
	}

	imapFlag := mailer.ResolveFlag(flagAlias)

	prov, err := provider.Lookup(email)
	if err != nil {
		return err
	}

	cacheDir, cacheDirErr := cache.Dir()
	if cacheDirErr != nil {
		fmt.Fprintf(os.Stderr, "mail: warning: cache dir: %v\n", cacheDirErr)
	}

	var cachedValidity uint32
	var cachedOK bool
	if cacheDirErr == nil {
		var loadErr error
		cachedValidity, cachedOK, loadErr = cache.LoadUIDValidity(cacheDir, email, "INBOX")
		if loadErr != nil {
			fmt.Fprintf(os.Stderr, "mail: warning: load uidvalidity: %v\n", loadErr)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	tok, err := auth.GetToken(ctx, email, prov)
	if err != nil {
		return err
	}

	if err := mailer.FlagMessages(ctx, &mailer.FlagParams{
		Email:          email,
		Provider:       prov,
		Token:          tok,
		Op:             op,
		Flag:           imapFlag,
		UIDs:           uids,
		CachedValidity: cachedValidity,
		CachedOK:       cachedOK,
	}); err != nil {
		return err
	}

	// Confirmation to stdout.
	var displayOp string
	switch op {
	case "add":
		displayOp = "added"
	case "rm", "remove":
		displayOp = "removed"
	case "set":
		displayOp = "set"
	}
	fmt.Printf("%s %s on %d message(s)\n", displayOp, flagAlias, len(uids))

	return nil
}
