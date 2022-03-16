# `image-scraper`

[![crawl](https://github.com/imjasonh/image-scraper/actions/workflows/crawl.yaml/badge.svg)](https://github.com/imjasonh/image-scraper/actions/workflows/crawl.yaml)

This repo periodically crawls popular base images and tracks what tags exist, and what digests they point to.

### How is this useful?

It scrapes and caches image and index manifests. This could be useful if you want to avoid rate limiting, but GitHub also rate-limits, so I doubt it's very useful. The crawl would lag up to an hour anyway (longer if it's broken and I haven't fixed it yet!)

The main reason I wrote this is to try to detect base images for built images.

The crawl also generates an [index](./index.txt) of a layer digest and its position in the base image, to the image that contains it (and platform, for multiplatform images).

The index looks like this:

```
...
sha256:29291e31a76a7e560b9b7ad3cada56e8c18d50a96cca8a2573e4f4689d7aca77 0 * index.docker.io/library/alpine:3.14.1 linux/amd64
sha256:292e8cd7ee20f1ac032e6ca9196fda517ae1ba62f4c8e6393f69e75c97639701 0 * index.docker.io/library/debian:bullseye-20200908-slim linux/mips64le
sha256:293b44f451623251bf75ce5a72d3cee63706972c88980232217a81026987f63e 2 - index.docker.io/library/ubuntu:xenial-20190610 linux/amd64
sha256:2941ecd8ffcace2a2ed621a00e6c9a63554f2ceff12d1bffc6b488aec74dbd1a 0 * index.docker.io/library/debian:unstable-20210408-slim linux/arm64/v8
...
```

The top line indicates that layer `sha256:28281e` is the 0th and final layer in `alpine:3.14`'s image for `linux/amd64`.
So if your image's 0th layer is also that, it may be based on `alpine:3.14`.

The index also includes these lines (though not adjacent in the file):

```
sha256:58690f9b18fca6469a14da4e212c96849469f9b1be6661d2342a4bf01774aa50 0 - index.docker.io/library/ubuntu:xenial linux/amd64
sha256:b51569e7c50720acf6860327847fe342a1afbe148d24c529fb81df105e3eed01 1 - index.docker.io/library/ubuntu:xenial linux/amd64
sha256:da8ef40b9ecabc2679fe2419957220c0272a965c5cf7e0269fa1aeeb8c56f2e1 2 - index.docker.io/library/ubuntu:xenial linux/amd64
sha256:fb15d46c38dcd1ea0b1990006c3366ecd10c79d374f341687eb2cb23a2c8672e 3 * index.docker.io/library/ubuntu:xenial linux/amd64
```

This shows all four layers of `ubuntu:xenial` for `linux/amd64`.
If those four layers are the bottom-most four layers of your image (until you encounter the `*`), your image may be based on `ubuntu:xenial`.

Detecting an image's base image can be useful for determining if it should be rebuilt or rebased when a newer base image is available.
