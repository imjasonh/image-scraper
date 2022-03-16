#!/usr/bin/env bash

count=$(find manifests/ -type f | wc -l)
curl "https://img.shields.io/badge/manifests_cached-$count-green" -o manifests-cached.svg

count=$(wc -l < index.txt | xargs)
curl "https://img.shields.io/badge/layers_indexed-$count-green" -o layers-indexed.svg
