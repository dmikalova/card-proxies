//go:build mage

// card-proxies tasks. Run `mage -l` to list them, and
// `mage ci:fix && mage ci:check` before calling work done.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	// mage:import ci
	_ "github.com/dmikalova/project-standards/ci"
	"github.com/magefile/mage/mg"

	"github.com/dmikalova/card-proxies/internal/sorcery"
)

const (
	// cookieEnv holds the sorcerytcg.com session cookie, over the Keychain.
	cookieEnv = "SORCERYTCG_COOKIE"
	// keychainService is the macOS Keychain item of the session cookie.
	keychainService = "card-proxies-sorcerytcg"
	// silhouetteEnv is the path of the silhouette-card-maker checkout.
	silhouetteEnv = "SILHOUETTE_CARD_MAKER"
	// runIDFormat names run directories so that the newest sorts last.
	runIDFormat = "2006-01-02-1504"
)

// Sorcery holds the Sorcery: Contested Realm proxy tasks.
type Sorcery mg.Namespace

// Catalog downloads the public card catalog into data/sorcery/cards.json.
func (Sorcery) Catalog(ctx context.Context) error {
	p, err := projectPaths()
	if err != nil {
		return err
	}
	if err := sorcery.UpdateCatalog(ctx, httpClient(), p); err != nil {
		return err
	}
	fmt.Printf("Wrote %s. Commit it.\n", p.Catalog())
	return nil
}

// Collection downloads the owned quantities from sorcerytcg.com into
// data/sorcery/collection.json. It needs the session cookie of a signed-in
// browser, from SORCERYTCG_COOKIE or the macOS Keychain.
func (Sorcery) Collection(ctx context.Context) error {
	p, err := projectPaths()
	if err != nil {
		return err
	}
	cookie, err := sessionCookie(ctx)
	if err != nil {
		return err
	}
	if err := sorcery.UpdateCollection(ctx, httpClient(), p, cookie); err != nil {
		return err
	}
	fmt.Printf("Wrote %s. Commit it.\n", p.Collection())
	return nil
}

// Plan prints what the next run prints. It downloads and changes nothing.
func (Sorcery) Plan() error {
	p, err := projectPaths()
	if err != nil {
		return err
	}
	planned, err := sorcery.Plan(p)
	if err != nil {
		return err
	}
	report := sorcery.Report{Entries: planned.Entries, OwnedFrom: planned.OwnedFrom}
	return report.Write(os.Stdout)
}

// Stage downloads the card images of the next run and writes the run to
// runs/sorcery/<date-time>/.
func (Sorcery) Stage(ctx context.Context) error {
	p, err := projectPaths()
	if err != nil {
		return err
	}
	id := time.Now().Format(runIDFormat)
	staged, err := sorcery.StageRun(ctx, httpClient(), p, id)
	if err != nil {
		return err
	}
	report := sorcery.Report{
		Entries:   staged.Entries,
		Images:    staged.Images,
		OwnedFrom: staged.OwnedFrom,
		Staged:    true,
		Backs:     staged.Backs,
	}
	if err := report.Write(os.Stdout); err != nil {
		return err
	}
	fmt.Printf("\nStaged run %s in %s. Next: mage sorcery:pdf\n", id, staged.Dir)
	return nil
}

// PDF runs silhouette-card-maker's create_pdf.py on the newest staged run.
func (Sorcery) PDF(ctx context.Context) error {
	p, err := projectPaths()
	if err != nil {
		return err
	}
	id, err := sorcery.LatestRun(p)
	if err != nil {
		return err
	}
	cfg, err := sorcery.LoadConfig(p)
	if err != nil {
		return err
	}
	args, err := sorcery.PDFArgs(p, id, cfg)
	if err != nil {
		return err
	}
	dir, python, err := silhouetteCardMaker()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, python, append([]string{"create_pdf.py"}, args...)...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	fmt.Printf("%s %s\n", python, strings.Join(cmd.Args[1:], " "))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create_pdf.py: %w", err)
	}
	fmt.Printf("\nPrint and cut it, then run mage sorcery:record.\n")
	return nil
}

// Run stages the next run and makes its PDF.
func (Sorcery) Run(ctx context.Context) {
	mg.SerialCtxDeps(ctx, Sorcery.Stage, Sorcery.PDF)
}

// Record adds the newest staged run to data/sorcery/printed.json. Run it
// after the run is printed and cut, so that the next run skips those cards.
func (Sorcery) Record() error {
	p, err := projectPaths()
	if err != nil {
		return err
	}
	id, err := sorcery.LatestRun(p)
	if err != nil {
		return err
	}
	if err := sorcery.RecordRun(p, id); err != nil {
		return err
	}
	fmt.Printf("Recorded run %s in %s. Commit it.\n", id, p.Ledger())
	return nil
}

func projectPaths() (sorcery.Paths, error) {
	root, err := os.Getwd()
	if err != nil {
		return sorcery.Paths{}, fmt.Errorf("find the project root: %w", err)
	}
	return sorcery.NewPaths(root), nil
}

func httpClient() *http.Client {
	return &http.Client{Timeout: time.Minute}
}

// sessionCookie reads the sorcerytcg.com session cookie from cookieEnv, or
// else from the macOS Keychain.
func sessionCookie(ctx context.Context) (string, error) {
	if cookie := os.Getenv(cookieEnv); cookie != "" {
		return cookie, nil
	}
	out, err := exec.CommandContext(ctx, "security", "find-generic-password", "-s", keychainService, "-w").
		Output()
	if err != nil {
		return "", fmt.Errorf("no sorcerytcg.com session cookie in %s or the Keychain. "+
			"Copy the Cookie request header from a signed-in sorcerytcg.com tab, then run: "+
			"security add-generic-password -a \"$USER\" -s %s -w", cookieEnv, keychainService)
	}
	return string(bytes.TrimSpace(out)), nil
}

// silhouetteCardMaker finds the silhouette-card-maker checkout and the Python
// of its venv.
func silhouetteCardMaker() (dir, python string, err error) {
	dir = os.Getenv(silhouetteEnv)
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", fmt.Errorf("find the home directory: %w", err)
		}
		dir = filepath.Join(home, "Code", "github.com", "Alan-Cha", "silhouette-card-maker")
	}
	python = filepath.Join(dir, "venv", "bin", "python")
	if _, err := os.Stat(python); errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf(
			"%s does not exist: set %s to the silhouette-card-maker checkout, "+
				"and make its venv as its README says",
			python,
			silhouetteEnv,
		)
	}
	return dir, python, nil
}
