package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

var (
	commitSHARE = regexp.MustCompile(`\b[0-9a-f]{40}\b`)
	attRE       = regexp.MustCompile(`\.(sig|att|sbom|cosign)$`)
)

var (
	full    = flag.Bool("full", false, "if true, crawl manifests (may incur registry GETs")
	verbose = flag.Bool("v", false, "if true, log verbosely")
)

func main() {
	flag.Parse()
	ctx := context.Background()

	f, err := os.Open("images.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}

		repo, err := name.NewRepository(scanner.Text())
		if err != nil {
			log.Fatal(err)
		}

		if err := crawlRepo(ctx, repo); err != nil {
			log.Fatal(err)
		}
	}
}

func crawlRepo(ctx context.Context, repo name.Repository) error {
	fn := filepath.Clean(repo.String())
	if err := os.MkdirAll(filepath.Dir(fn), 0777); err != nil {
		return err
	}

	ls, err := remote.List(repo, remote.WithContext(ctx))
	if err != nil {
		log.Fatal(err)
	}
	for _, t := range ls {
		tag := repo.Tag(t)
		if commitSHARE.MatchString(t) {
			if *verbose {
				log.Println("cowardly refusing to crawl commit-tagged image:", tag)
			}
			continue
		}
		if attRE.MatchString(t) {
			if *verbose {
				log.Println("cowardly refusing to crawl attachment:", tag)
			}
			continue
		}

		if *verbose {
			log.Println("HEAD", tag)
		}
		desc, err := remote.Head(tag, remote.WithContext(ctx))
		if err != nil {
			log.Fatal(err)
		}

		b, err := json.Marshal(desc)
		if err != nil {
			return err
		}
		if err := os.WriteFile(tag.String(), b, 0644); err != nil {
			return err
		}

		if *full {
			rd := repo.Digest(desc.Digest.String())
			switch {
			case desc.MediaType.IsImage():
				if err := crawlImage(ctx, rd); err != nil {
					log.Fatal(err)
				}
			case desc.MediaType.IsIndex():
				if err := crawlIndex(ctx, rd); err != nil {
					log.Fatal(err)
				}
			default:
				if *verbose {
					log.Println(desc.Digest, "is not image or index:", desc.MediaType)
				}
			}
		}
	}
	return nil
}

func crawlImage(ctx context.Context, rd name.Digest) error {
	fn := filepath.Join("manifests", rd.DigestStr())
	if _, err := os.Stat(fn); os.IsNotExist(err) {
		log.Println("GET", rd)
		rdesc, err := remote.Get(rd, remote.WithContext(ctx))
		if terr, ok := err.(*transport.Error); ok {
			if terr.StatusCode == http.StatusTooManyRequests {
				log.Println("got 429 trying to crawl; skipping", rd.DigestStr())
				return nil
			}
			return err

		} else if err != nil {
			return err
		}
		if err := os.WriteFile(fn, rdesc.Manifest, 0644); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if *verbose {
			log.Println("already have manifest for image", rd.DigestStr())
		}
	}
	return nil
}

func crawlIndex(ctx context.Context, rd name.Digest) error {
	fn := filepath.Join("manifests", rd.DigestStr())
	if _, err := os.Stat(fn); os.IsNotExist(err) {
		log.Println("GET", rd)
		rdesc, err := remote.Get(rd, remote.WithContext(ctx))
		if terr, ok := err.(*transport.Error); ok {
			if terr.StatusCode == http.StatusTooManyRequests {
				log.Println("got 429 trying to crawl; skipping", rd.DigestStr())
				return nil
			}
			return err

		} else if err != nil {
			return err
		}
		if err := os.WriteFile(fn, rdesc.Manifest, 0644); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	f, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer f.Close()
	mf, err := v1.ParseIndexManifest(f)
	if err != nil {
		return err
	}

	for _, desc := range mf.Manifests {
		switch {
		case desc.MediaType.IsImage():
			ird := rd.Context().Digest(desc.Digest.String())
			if *verbose {
				log.Println("index", rd, "has image", ird)
			}
			if err := crawlIndex(ctx, ird); err != nil {
				return err
			}
		case desc.MediaType.IsIndex():
			if *verbose {
				log.Println("cowardly refusing to crawl recursive index: ", desc.Digest)
			}
			continue
		default:
			if *verbose {
				log.Println(desc.Digest, "is not image or index:", desc.MediaType)
			}
		}
	}
	return nil
}
