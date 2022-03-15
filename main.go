package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

var (
	commitSHARE = regexp.MustCompile(`\b[0-9a-f]{40}\b`)
	attRE       = regexp.MustCompile(`\.(sig|att|sbom|cosign)$`)
)

var (
	full = flag.Bool("full", false, "if true, crawl manifests")
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

	out, err := os.OpenFile(repo.String(), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	ls, err := remote.List(repo, remote.WithContext(ctx))
	if err != nil {
		log.Fatal(err)
	}
	for _, t := range ls {
		tag := repo.Tag(t)
		if commitSHARE.MatchString(t) {
			log.Println("cowardly refusing to crawl commit-tagged image:", tag)
			continue
		}
		if attRE.MatchString(t) {
			log.Println("cowardly refusing to crawl attachment:", tag)
			continue
		}

		log.Println("HEAD", tag)
		desc, err := remote.Head(tag)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Fprintln(out, tag, desc.Digest)

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
				log.Println(desc.Digest, "is not image or index:", desc.MediaType)
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
		if err != nil {
			return err
		}
		mf, err := os.Create(fn)
		if err != nil {
			return err
		}
		defer mf.Close()
		if _, err := io.Copy(mf, bytes.NewReader(rdesc.Manifest)); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		log.Println("already have manifest for image", rd.DigestStr())
	}
	return nil
}

func crawlIndex(ctx context.Context, rd name.Digest) error {
	fn := filepath.Join("manifests", rd.DigestStr())
	if _, err := os.Stat(fn); os.IsNotExist(err) {
		log.Println("GET", rd)
		rdesc, err := remote.Get(rd, remote.WithContext(ctx))
		if err != nil {
			return err
		}
		mf, err := os.Create(fn)
		if err != nil {
			return err
		}
		defer mf.Close()
		if _, err := io.Copy(mf, bytes.NewReader(rdesc.Manifest)); err != nil {
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
			log.Println("index", rd, "has image", ird)
			if err := crawlIndex(ctx, ird); err != nil {
				return err
			}
		case desc.MediaType.IsIndex():
			log.Println("cowardly refusing to crawl recursive index: ", desc.Digest)
			continue
		default:
			log.Println(desc.Digest, "is not image or index:", desc.MediaType)
		}
	}
	return nil
}
