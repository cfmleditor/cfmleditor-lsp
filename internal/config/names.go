package config

import (
	"os"
	"path/filepath"
	"slices"
)

// FileName is the project config file. LegacyFileName is what it was called
// while the project was cfmleditor-lsp. Both are read, the new name first, so
// no project has to rename its config; every new file is written under the new
// name.
const (
	FileName       = ".clif.json"
	LegacyFileName = ".cfmleditor.json"
)

// FileNames are the config file's names, in the order a directory is searched.
var FileNames = []string{FileName, LegacyFileName}

// IsFileName reports whether base, a file's base name, names a config file.
func IsFileName(base string) bool {
	return slices.Contains(FileNames, base)
}

// CandidatesIn returns the paths a config file in dir may have, in the order
// to try them: a directory holding both is configured by .clif.json.
func CandidatesIn(dir string) []string {
	out := make([]string, 0, len(FileNames))
	for _, n := range FileNames {
		out = append(out, filepath.Join(dir, n))
	}

	return out
}

// defaultGenerated is where each kind is written when no knownIssues entry is
// marked to receive it, beside the config file: the new name and the legacy
// one. See defaultReport.
var defaultGenerated = map[string][2]string{
	GenerateUnresolved: {".clif-unresolved.txt", ".cfmleditor-unresolved.txt"},
	GenerateCFLint:     {".clif-cflint.txt", ".cfmleditor-cflint.txt"},
}

// defaultReport is a kind's implicit report file in dir: the legacy name when
// a file of that name exists and none of the new one does, so a project that
// committed its report under the old name keeps it, and the new name
// otherwise. "" for a kind with no default.
func defaultReport(dir, kind string) string {
	names, ok := defaultGenerated[kind]
	if !ok {
		return ""
	}

	current, legacy := filepath.Join(dir, names[0]), filepath.Join(dir, names[1])

	if !exists(current) && exists(legacy) {
		return legacy
	}

	return current
}

// isDefaultReport reports the kind whose default file, under either name, is
// path in dir.
func isDefaultReport(path, dir string) (string, bool) {
	for kind, names := range defaultGenerated {
		for _, n := range names {
			if path == filepath.Join(dir, n) {
				return kind, true
			}
		}
	}

	return "", false
}

func exists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}
