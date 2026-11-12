// update-release signs verified native CI ZIPs locally, or verifies a draft
// using only the public key. It never uploads or prints private key material.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"ngareader/internal/appupdate"
)

func main() {
	if len(os.Args) < 2 {
		fail(fmt.Errorf("usage: update-release sign|verify --dir ... --version ... --key ..."))
	}
	mode := os.Args[1]
	f := flag.NewFlagSet(mode, flag.ExitOnError)
	dir := f.String("dir", "dist", "directory containing the three native portable ZIPs")
	version := f.String("version", "", "release version without v")
	keyPath := f.String("key", "", "private key file for sign; public key file for verify")
	notesPath := f.String("notes", "packaging/update-notes.txt", "compact in-app update notes file")
	date := f.String("date", time.Now().UTC().Format(time.RFC3339), "UTC release date")
	target := f.String("target", "windows-amd64", "platform for check")
	f.Parse(os.Args[2:])
	keyText, err := os.ReadFile(*keyPath)
	if err != nil {
		fail(fmt.Errorf("cannot read the required signing/verification key file"))
	}
	if mode == "sign" {
		key, err := appupdate.PrivateKey(string(keyText))
		if err != nil {
			fail(err)
		}
		notes, err := os.ReadFile(*notesPath)
		if err != nil {
			fail(err)
		}
		err = appupdate.SignDirectory(*dir, strings.TrimPrefix(*version, "v"), string(notes), *date, key)
		if err != nil {
			fail(err)
		}
	} else if mode == "verify" {
		pub, err := appupdate.PublicKey(string(keyText))
		if err != nil {
			fail(err)
		}
		if err := appupdate.VerifyDirectory(*dir, strings.TrimPrefix(*version, "v"), pub); err != nil {
			fail(err)
		}
	} else if mode == "check" {
		pub, err := appupdate.PublicKey(string(keyText))
		if err != nil {
			fail(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		offer, err := appupdate.NewClient(&http.Client{Timeout: 40 * time.Second}, pub).Latest(ctx, *version, *target)
		if err != nil {
			fail(err)
		}
		if offer == nil {
			fmt.Println("No higher signed version available")
		} else {
			fmt.Println("Verified GitHub signed update: " + offer.Manifest.Version + " / " + offer.Manifest.Target)
		}
		return
	} else {
		fail(fmt.Errorf("unknown operation"))
	}
	fmt.Println("All three platform update archives and metadata verified")
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
