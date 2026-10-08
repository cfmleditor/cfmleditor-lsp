package cflint

import (
	"bytes"
	// The standard library's v1 decoder on purpose: it is as strict as CFLint's
	// own parser — no comments, no trailing commas, no single quotes, unknown
	// keys allowed — which is what makes "clif rejects it" and "CFLint ignores
	// it" the same set of files.
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ConfigFile is CFLint's per-folder configuration file.
const ConfigFile = ".cflintrc"

// CheckConfigs parses every .cflintrc CFLint reads for files and returns an
// error naming the first it cannot parse.
//
// CFLint does not report a .cflintrc it cannot parse: it lints with its
// default rules and exits as if nothing were wrong. A comment, a trailing
// comma or a half-saved file therefore turns a project's rule set into a
// different one, and a gate built on the result passes or fails code against
// rules nobody chose. Checking first makes that a configuration error.
//
// The files checked are the ones CFLint reads: from each file's folder
// upwards, every .cflintrc, stopping after one that sets "inheritParent":
// false. A broken file above that point is never read, so it is not an error
// here either.
func CheckConfigs(files []string) error {
	checked := map[string]bool{}

	for _, f := range files {
		abs, err := filepath.Abs(f)
		if err != nil {
			return err
		}

		for dir := filepath.Dir(abs); ; {
			if checked[dir] {
				break
			}

			checked[dir] = true

			stop, err := checkConfigIn(dir)
			if err != nil {
				return err
			}

			parent := filepath.Dir(dir)
			if stop || parent == dir {
				break
			}

			dir = parent
		}
	}

	return nil
}

// checkConfigIn parses dir's .cflintrc, if it has one, and reports whether
// CFLint stops looking upwards after it.
func checkConfigIn(dir string) (bool, error) {
	path := filepath.Join(dir, ConfigFile)

	data, err := os.ReadFile(path) //nolint:gosec // a .cflintrc beside a file being linted
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}

	// CFLint accepts a UTF-8 byte-order mark; the decoder does not.
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	var cfg struct {
		InheritParent *bool `json:"inheritParent"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return false, fmt.Errorf("%s is not valid JSON (%w); CFLint would ignore it and lint with its default rules", path, err)
	}

	return cfg.InheritParent != nil && !*cfg.InheritParent, nil
}
