package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"os/user"
	"path"
	"runtime"
	"strings"

	cli "github.com/jawher/mow.cli"
	"github.com/seantis/roots/pkg/image"
	_ "github.com/seantis/roots/pkg/provider" // to register providers
)

var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)

func main() {
	app := cli.App("roots", "Download and extract containers")
	ctx := newInterruptableContext()

	// disable datetime output
	log.SetFlags(0)

	app.Command("version", "Show version", func(cmd *cli.Cmd) {
		cmd.Action = func() {
			fmt.Printf("roots %s, commit %s, built at %s\n", version, commit, date)
		}
	})

	app.Command("digest", "Show the latest digest", func(cmd *cli.Cmd) {
		cmd.Spec = "CONTAINER [--auth] [--arch] [--os]"

		var (
			url  = newURLArg(cmd)
			auth = newAuthOpt(cmd)
			arch = newArchOpt(cmd)
			ops  = newOSOpt(cmd)
		)

		cmd.Action = func() {
			digest, err := newRemote(ctx, url, auth, arch, ops).Digest()

			if err != nil {
				log.Fatal(err)
			}

			fmt.Println(digest)
		}
	})

	app.Command("purge", "Purge unused files from the cache", func(cmd *cli.Cmd) {
		cmd.Spec = "[--cache]"

		var (
			cache = newCacheOpt(cmd)
		)

		cmd.Action = func() {
			// setup the cache
			if *cache == "" {
				*cache = os.Getenv("ROOTS_CACHE")
			}

			if *cache == "" {
				*cache = defaultCache()
			}

			entries, err := os.ReadDir(*cache)
			if err != nil {
				log.Fatalf("error accessing %s: %v", *cache, err)
			}

			if len(entries) == 0 {
				log.Fatalf("not a cache directory: %s", *cache)
			}

			valid := false
			for _, info := range entries {
				if info.Name() == "layers" {
					valid = true
					break
				}
			}

			if !valid {
				log.Fatalf("not a cache directory: %s", *cache)
			}

			store, err := image.NewStore(*cache)
			if err != nil {
				log.Fatalf("could not create store at %s: %v", *cache, err)
			}

			if err := store.Purge(); err != nil {
				log.Fatalf("error during purge of %s: %v", *cache, err)
			}
		}
	})

	app.Command("pull", "Download and extract", func(cmd *cli.Cmd) {
		cmd.Spec = "CONTAINER DEST [--auth] [--arch] [--os] [--cache] [--force]"

		var (
			url   = newURLArg(cmd)
			dest  = newDestArg(cmd)
			auth  = newAuthOpt(cmd)
			arch  = newArchOpt(cmd)
			ops   = newOSOpt(cmd)
			cache = newCacheOpt(cmd)
			force = newForceOpt(cmd)
		)

		cmd.Action = func() {

			// setup the cache
			if *cache == "" {
				*cache = os.Getenv("ROOTS_CACHE")
			}

			if strings.ToLower(*cache) == "no" {
				temp, err := os.MkdirTemp("", "store")
				if err != nil {
					log.Fatal(err)
				}
				defer os.RemoveAll(temp)

				*cache = temp
			}

			if *cache == "" {
				*cache = defaultCache()
			}

			if err := os.MkdirAll(*cache, 0755); err != nil {
				log.Fatalf("could not create cache at %s: %v", *cache, err)
			}

			store, err := image.NewStore(*cache)
			if err != nil {
				log.Fatalf("could not create store at %s: %v", *cache, err)
			}

			// create the destination
			if *force {

				// let's not be responsible for wiping out an actual root fs
				if strings.Count(*dest, "/") <= 2 {
					log.Fatalf("not enough path separators to force-remove: %s", *dest)
				}

				if err := os.RemoveAll(*dest); err != nil {
					log.Fatalf("could note force-remove %s: %v", *dest, err)
				}

			}

			if err := os.MkdirAll(*dest, 0755); err != nil {
				log.Fatalf("could not create destination at %s: %v", *dest, err)
			}

			// pull & extract the image
			remote := newRemote(ctx, url, auth, arch, ops)

			if err := store.Extract(ctx, remote, *dest); err != nil {
				log.Fatalf("error during pull: %v", err)
			}
		}
	})

	err := app.Run(os.Args)
	if err != nil {
		log.Fatalf("error running command: %v", err)
	}
}

func defaultCache() string {
	usr, err := user.Current()

	if err != nil {
		log.Fatalf("error looking up current user: %v", err)
	}

	if usr.Uid == "0" || usr.HomeDir == "" {
		return "/var/cache/roots"
	}

	return path.Join(usr.HomeDir, ".cache", "seantis", "roots")
}

func newInterruptableContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		signal.Stop(c)
		cancel()
	}()

	return ctx
}

func newRemote(ctx context.Context, urlstring, auth, arch, ops *string) *image.Remote {

	if *auth == "" {
		*auth = os.Getenv("ROOTS_AUTH")
	}

	if *arch == "" {
		*arch = os.Getenv("ROOTS_ARCH")
	}

	if *ops == "" {
		*ops = os.Getenv("ROOTS_OS")
	}

	url, err := image.Parse(*urlstring)
	if err != nil {
		log.Fatalf("failed to parse image url %s: %v", *urlstring, err)
	}

	remote, err := image.NewRemote(ctx, *url, *auth)
	if err != nil {
		log.Fatalf("failed to connect to %s: %v", *urlstring, err)
	}

	if len(*arch) > 0 || len(*ops) > 0 {
		if len(*arch) == 0 {
			*arch = runtime.GOARCH
		}

		if len(*ops) == 0 {
			*ops = "linux"
		}

		remote.WithPlatform(&image.Platform{
			Architecture: *arch,
			OS:           *ops,
		})
	}

	return remote
}

func newURLArg(cmd *cli.Cmd) *string {
	return cmd.StringArg("CONTAINER", "",
		`The url of the container, example values:

               - ubuntu:latest
               - gcr.io/google-containers/etcd:3.3.10
	`)
}

func newDestArg(cmd *cli.Cmd) *string {
	return cmd.StringArg("DEST", "", "The destination folder")
}

func newAuthOpt(cmd *cli.Cmd) *string {
	return cmd.StringOpt("auth", "",
		`Authentication for the following providers:

               * Google Container Registry:
                 Path to service worker json file, with the following scope:
                 <https://www.googleapis.com/auth/devstorage.read_only>

               This value can also be set through the env var ROOTS_AUTH,
               though the flag takes precedence.
	`)
}

func newArchOpt(cmd *cli.Cmd) *string {
	return cmd.StringOpt("arch", "",
		`Force the given architecture, example values:

               * amd64
               * arm

               See https://github.com/golang/go/blob/master/src/go/build/syslist.go

               Requires multi-arch support by the container.

               This value can also be set through the env var ROOTS_ARCH,
               though the flag takes precedence.
	`)
}

func newOSOpt(cmd *cli.Cmd) *string {
	return cmd.StringOpt("os", "",
		`Force the given OS, example values:

               * linux
               * windows

               See https://github.com/golang/go/blob/master/src/go/build/syslist.go

               Requires multi-arch support by the container.

               This value can also be set through the env var ROOTS_OS,
               though the flag takes precedence.
	`)
}

func newCacheOpt(cmd *cli.Cmd) *string {
	return cmd.StringOpt("cache", "",
		`Sets the cache folder that should be used. Defaults:

               * For non-root users:
                 ~/.cache/seantis/roots

               * For root users:
                 /var/cache/roots

               If the special value 'no' is given, a temporary folder will
               be used during the lifetime of the process.

               This value can also be set through the env var ROOTS_CACHE,
               though the flag takes precedence.
	`)
}

func newForceOpt(cmd *cli.Cmd) *bool {
	return cmd.BoolOpt("force", false, `Remove the destination before pulling

               Note that this only works if there are at least two path
               separatores in the destination. So you can force remove
               /var/roots/ubuntu, but not / or /var/lib.
	`)
}
