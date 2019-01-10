package maddy

import (
	"io"
	"strings"
	"time"

	pgppubkey "github.com/emersion/go-pgp-pubkey"
	pgppubkeylocal "github.com/emersion/go-pgp-pubkey/local"
	pgplocal "github.com/emersion/go-pgpmail/local"
	smtppgp "github.com/emersion/go-pgpmail/smtp"
	"github.com/emersion/go-smtp"
	smtpproxy "github.com/emersion/go-smtp-proxy"
	"github.com/emersion/go-smtp/backendutil"
	"github.com/mholt/caddy/caddyfile"
	"golang.org/x/crypto/openpgp"
)

func newSMTPServer(tokens map[string][]caddyfile.Token) (server, error) {
	var be smtp.Backend
	if tokens, ok := tokens["proxy"]; ok {
		var err error
		be, err = newSMTPProxy(caddyfile.NewDispenserTokens("", tokens))
		if err != nil {
			return nil, err
		}
	}

	if tokens, ok := tokens["pgp"]; ok {
		var err error
		be, err = newSMTPPGP(caddyfile.NewDispenserTokens("", tokens), be)
		if err != nil {
			return nil, err
		}
	}

	s := smtp.NewServer(newRelay(be))
	return s, nil
}

func newSMTPProxy(d caddyfile.Dispenser) (smtp.Backend, error) {
	for d.Next() {
		args := d.RemainingArgs()

		if len(args) == 1 {
			addr, err := standardizeAddress(args[0])
			if err != nil {
				return nil, err
			}

			if addr.Scheme == "lmtp" {
				return smtpproxy.NewLMTP(addr.Path, "localhost"), nil
			}

			if addr.IsTLS() {
				return smtpproxy.NewTLS(addr.Address(), nil), nil
			} else {
				return smtpproxy.New(addr.Address()), nil
			}
		}
	}

	return nil, nil
}

type keyRing struct {
	pgppubkey.Source
}

func (kr *keyRing) Unlock(username, password string) (openpgp.EntityList, error) {
	return pgplocal.Unlock(username, password)
}

func newSMTPPGP(d caddyfile.Dispenser, be smtp.Backend) (smtp.Backend, error) {
	kr := &keyRing{pgppubkeylocal.New()}
	pgpbe := smtppgp.New(be, kr)
	return pgpbe, nil
}

func newRelay(be smtp.Backend) smtp.Backend {
	hostname := "" // TODO

	return &backendutil.TransformBackend{
		Backend: be,
		Transform: func(from string, to []string, r io.Reader) (string, []string, io.Reader) {
			received := "Received:"
			if hostname != "" {
				received += " by " + hostname
			}
			received += " with ESMTP"
			received += "; " + time.Now().Format(time.RFC1123Z) + "\r\n"

			r = io.MultiReader(strings.NewReader(received), r)
			return from, to, r
		},
	}
}
