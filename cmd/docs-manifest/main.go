// docs-manifest validates and packs versioned product-docs subtrees for
// the Oxidean org site contract (manifest_version 1).
//
// Usage:
//
//	docs-manifest validate <dir>
//	docs-manifest pack <dir> --version <x.y.z> [--out <file>]
//	docs-manifest schema
//	docs-manifest version
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/oxidean/docs-manifest/internal/buildinfo"
	"github.com/oxidean/docs-manifest/internal/pack"
	"github.com/oxidean/docs-manifest/internal/validate"
	"github.com/oxidean/docs-manifest/schema"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "validate":
		err = cmdValidate(os.Args[2:])
	case "pack":
		err = cmdPack(os.Args[2:])
	case "schema":
		_, err = os.Stdout.Write(schema.ManifestV1)
	case "version", "--version", "-v":
		fmt.Println(buildinfo.Version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "docs-manifest: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "docs-manifest: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `docs-manifest — versioned product-docs contract tooling

  validate <dir>                    validate a docs/site subtree
  pack <dir> --version X.Y.Z        validate, gate version, emit docs-X.Y.Z.tar.gz + .sha256
      --out FILE                    output path (default docs-<version>.tar.gz)
  schema                            print the manifest_version 1 JSON schema
  version                           print binary version
`)
}

// reorderArgs moves flag arguments ahead of positionals so flags work in
// any position (stdlib flag stops parsing at the first positional).
func reorderArgs(args []string) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			pos = append(pos, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "-") && !strings.Contains(a, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-"):
			flags = append(flags, a, args[i+1])
			i++
		case strings.HasPrefix(a, "-"):
			flags = append(flags, a)
		default:
			pos = append(pos, a)
		}
	}
	return append(flags, pos...)
}

func cmdValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	fs.Usage = usage
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("validate: expected exactly one directory")
	}
	if _, err := validate.Dir(fs.Arg(0)); err != nil {
		return err
	}
	fmt.Printf("%s: valid (docs-manifest %s)\n", fs.Arg(0), buildinfo.Version)
	return nil
}

func cmdPack(args []string) error {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	version := fs.String("version", "", "expected manifest.version (tag minus prefix)")
	out := fs.String("out", "", "output path (default docs-<version>.tar.gz)")
	fs.Usage = usage
	if err := fs.Parse(reorderArgs(args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("pack: expected exactly one directory")
	}
	if *version == "" {
		return errors.New("pack: --version is required")
	}
	p, err := pack.Pack(pack.Options{Dir: fs.Arg(0), Version: *version, Out: *out})
	if err != nil {
		return err
	}
	fmt.Printf("wrote %s + %s.sha256\n", p, p)
	return nil
}
