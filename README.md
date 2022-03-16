# `image-scraper`

[![crawl](https://github.com/imjasonh/image-scraper/actions/workflows/crawl.yaml/badge.svg)](https://github.com/imjasonh/image-scraper/actions/workflows/crawl.yaml)

This repo periodically crawls popular base images and tracks what tags exist, and what digests they point to.

It also scrapes and caches image and index manifests.

### How is this useful?

I don't know. _Maybe?_

The scraper also generates an [index](./index.txt) of a layer digest and its position in the base image, to the image that contains it (and platform, for multiplatform images).

This means it might be useful for detecting the base image for a given image, if that image's Nth layer matches the base image's Nth layer, for all layers in the base image -- the base image's topmost layer is marked with a `*`, otherwise `-` if there are more layers.

Detecting an image's base image can be useful for determining if it should be rebuilt or rebased using a newer base image.
