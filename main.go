package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

var (
	commitSHARE = regexp.MustCompile(`\b[0-9a-f]{40}\b`)
	attRE       = regexp.MustCompile(`\.(sig|att|sbom|cosign)$`)
)

var (
	full      = flag.Bool("full", false, "if true, crawl manifests (may incur registry GETs")
	indexonly = flag.Bool("indexonly", false, "if true, only regenerate index from local files")
)

func main() {
	flag.Parse()
	ctx := context.Background()

	f, err := os.Open("images.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if !*indexonly {
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
	if err := indexRepo(ctx); err != nil {
		log.Fatal(err)
	}
}

type entry struct {
	tag, plat string
	layers    []string
}

func indexRepo(ctx context.Context) error {
	entries := []entry{}

	indexImage := func(path string, desc v1.Descriptor) error {
		f, err := os.Open(filepath.Join("manifests", desc.Digest.String()))
		if err != nil {
			return err
		}
		defer f.Close()

		mf, err := v1.ParseManifest(f)
		if err != nil {
			return err
		}
		ls := make([]string, 0, len(mf.Layers))
		for _, l := range mf.Layers {
			ls = append(ls, l.Digest.String())
		}
		var platstr string
		if plat := desc.Platform; plat != nil {
			platstr = plat.String()
		}
		entries = append(entries, entry{path, platstr, ls})
		return nil
	}
	indexIndex := func(path string, desc v1.Descriptor) error {
		f, err := os.Open(filepath.Join("manifests", desc.Digest.String()))
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
				if err := indexImage(path, desc); err != nil {
					return err
				}
			case desc.MediaType.IsIndex():
				log.Println("BUG: cowardly refusing to index recursive index: ", desc.Digest)
			default:
				log.Println("BUG:", desc.Digest, "is not image or index:", desc.MediaType)
			}
		}
		return nil
	}

	if err := filepath.Walk(".", func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		if strings.HasPrefix(path, "manifests/") {
			return filepath.SkipDir
		}
		if !strings.HasPrefix(path, "index.docker.io") &&
			!strings.HasPrefix(path, "gcr.io") {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		var desc v1.Descriptor
		if err := json.NewDecoder(f).Decode(&desc); err != nil {
			return err
		}

		switch {
		case desc.MediaType.IsImage():
			if err := indexImage(path, desc); err != nil {
				return err
			}
		case desc.MediaType.IsIndex():
			if err := indexIndex(path, desc); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// Reverse the entries from tag->[]layer, to ~ layer->tag, with idx and top-*
	bylayer := [][]string{}
	for _, e := range entries {
		for idx, l := range e.layers {
			top := "-"
			if idx == len(e.layers)-1 {
				top = "*"
			}
			bylayer = append(bylayer, []string{l, fmt.Sprintf("%d", idx), top, e.tag, e.plat})
		}
	}
	sort.Slice(bylayer, func(i, j int) bool {
		return bylayer[i][0] < bylayer[j][0]
	})
	f, err := os.OpenFile("index.txt", os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, bl := range bylayer {
		fmt.Fprintln(f, strings.Join(bl, " "))
	}
	return nil
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
			log.Println("cowardly refusing to crawl commit-tagged image:", tag)
			continue
		}
		if attRE.MatchString(t) {
			log.Println("cowardly refusing to crawl attachment:", tag)
			continue
		}

		log.Println("HEAD", tag)
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
		if err := os.WriteFile(fn, rdesc.Manifest, 0644); err != nil {
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
