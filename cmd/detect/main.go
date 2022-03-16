package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func main() {
	flag.Parse()
	ctx := context.Background()

	if len(os.Args) != 2 {
		log.Fatal("only accepts one arg, got ", os.Args[1:])
	}

	ref, err := name.ParseReference(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	// TODO: support indexes by finding matches for all contituent images and intersecting tags.
	img, err := remote.Image(ref,
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		log.Fatal(err)
	}

	index, err := setupIndex()
	if err != nil {
		log.Fatal(err)
	}

	layers, err := img.Layers()
	if err != nil {
		log.Fatal(err)
	}
	prev := []entry{}
	for i, l := range layers {
		h, err := l.Digest()
		if err != nil {
			log.Fatal(err)
		}
		k := fmt.Sprintf("%s-%d", h.String(), i)

		matches := index[k]
		for _, m := range matches {
			if m.top {
				log.Printf("possible match at layer %d: %s %s", i, m.tag, m.plat)
			}
		}

		if len(matches) == 0 {

			// This latest layer isn't part of any base image, so we're done.
			break
		}

		prev = matches
	}

	// We previously found exactly one match, so let's print it.
	if len(prev) == 1 {
		log.Println("single matching base image found!")
		fmt.Println(prev[0].tag)
	}
}

type entry struct {
	top       bool
	tag, plat string
}

func setupIndex() (map[string][]entry, error) {
	f, err := os.Open("index.txt")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m := map[string][]entry{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), " ")
		if len(parts) != 5 {
			log.Println("malformed line:", parts)
			continue
		}
		if parts[2] != "*" && parts[2] != "-" {
			log.Println("malformed line:", parts)
			continue
		}
		e := entry{
			top:  parts[2] == "*",
			tag:  parts[3],
			plat: parts[4],
		}
		k := fmt.Sprintf("%s-%s", parts[0], parts[1])
		m[k] = append(m[k], e)
	}
	return m, nil
}
