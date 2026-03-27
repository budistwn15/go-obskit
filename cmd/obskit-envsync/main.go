package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/budistwn15/go-obskit/configenv"
)

func main() {
	file := flag.String("file", ".env.example", "target .env.example path")
	create := flag.Bool("create", false, "create file if missing")
	header := flag.Bool("header", true, "add comment header when appending")
	profile := flag.String("profile", "minimal", "env profile: minimal|full")
	quiet := flag.Bool("quiet", false, "quiet mode")
	flag.Parse()

	p := configenv.ProfileMinimal
	if strings.EqualFold(strings.TrimSpace(*profile), "full") {
		p = configenv.ProfileFull
	}

	res, err := configenv.UpsertEnvExample(
		configenv.InjectOptions{
			FilePath:        *file,
			CreateIfMissing: *create,
			CommentHeader:   *header,
			Profile:         p,
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "obskit-envsync error: %v\n", err)
		os.Exit(1)
	}
	if *quiet {
		return
	}
	if res.Skipped {
		fmt.Printf("obskit-envsync: skip (file not found): %s\n", *file)
		return
	}
	if !res.Updated {
		fmt.Printf("obskit-envsync: no changes: %s\n", *file)
		return
	}
	fmt.Printf("obskit-envsync: updated %s (added %d keys)\n", *file, len(res.Added))
}
