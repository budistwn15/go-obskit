package configenv

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

type InjectOptions struct {
	FilePath        string
	CreateIfMissing bool
	CommentHeader   bool
	Profile         Profile
}

type Result struct {
	Updated bool
	Skipped bool
	Added   []string
}

func UpsertEnvExample(opts InjectOptions) (Result, error) {
	path := strings.TrimSpace(opts.FilePath)
	if path == "" {
		path = ".env.example"
	}
	path = filepath.Clean(path)

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if !opts.CreateIfMissing {
				return Result{Skipped: true}, nil
			}
			return writeNew(path, opts.CommentHeader, opts.Profile)
		}
		return Result{}, err
	}

	existing := parseExistingKeys(b)
	out := bytes.NewBuffer(nil)
	out.Write(b)
	if len(b) > 0 && b[len(b)-1] != '\n' {
		out.WriteByte('\n')
	}

	added := make([]string, 0)
	if opts.CommentHeader {
		out.WriteString("\n# Added by go-obskit configenv\n")
	}
	for _, e := range DefaultsByProfile(opts.Profile) {
		if _, ok := existing[e.Key]; ok {
			continue
		}
		out.WriteString(e.Key)
		out.WriteByte('=')
		out.WriteString(e.Value)
		out.WriteByte('\n')
		added = append(added, e.Key)
	}
	if len(added) == 0 {
		return Result{}, nil
	}
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		return Result{}, err
	}
	return Result{Updated: true, Added: added}, nil
}

func writeNew(path string, withHeader bool, profile Profile) (Result, error) {
	out := bytes.NewBuffer(nil)
	if withHeader {
		out.WriteString("# Added by go-obskit configenv\n")
	}
	defaults := DefaultsByProfile(profile)
	added := make([]string, 0, len(defaults))
	for _, e := range defaults {
		out.WriteString(e.Key)
		out.WriteByte('=')
		out.WriteString(e.Value)
		out.WriteByte('\n')
		added = append(added, e.Key)
	}
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		return Result{}, err
	}
	return Result{Updated: true, Added: added}, nil
}

func parseExistingKeys(b []byte) map[string]struct{} {
	out := map[string]struct{}{}
	s := bufio.NewScanner(bytes.NewReader(b))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:i])
		if k == "" {
			continue
		}
		out[k] = struct{}{}
	}
	return out
}
