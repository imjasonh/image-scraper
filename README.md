# `image-scraper`

[![crawl](https://github.com/imjasonh/image-scraper/actions/workflows/crawl.yaml/badge.svg)](https://github.com/imjasonh/image-scraper/actions/workflows/crawl.yaml)

![manifests cached](./manifests-cached.svg)
![layers indexed](./layers-indexed.svg)

This repo periodically crawls popular base images and tracks what tags exist, and what digests they point to.

If you'd like to start tracking another base image, [send a PR to add it here](https://github.com/imjasonh/image-scraper/edit/main/images.txt)!

### How is this useful?

It scrapes and caches image and index manifests. This could be useful if you want to avoid rate limiting on manifest `GET`s, but GitHub also rate-limits, so I doubt it's very useful. The crawl would lag up to an hour anyway (longer if it's broken and I haven't fixed it yet!).

Since history is also preserved here, you could use this repo to go back in time and discovery what a tag pointed to at some point in the repo's very short history.

But the main reason I wrote this is to try to detect base images for built images.

The crawl also generates an [index](./index.txt) of a layer digest and its position in the base image, to the image that contains it (and platform, for multiplatform images).

A line in the index looks like this:

```
...
sha256:29291e31a76a7e560b9b7ad3cada56e8c18d50a96cca8a2573e4f4689d7aca77 0 * index.docker.io/library/alpine:3.14.1 linux/amd64
...
```

This line indicates that layer `sha256:28281e` is the 0th and final layer in `alpine:3.14`'s image for `linux/amd64`.
So if your image's 0th layer is also that, it may be based on `alpine:3.14`!

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

But there's a problem!

If your image was built with a Dockerfile that's `FROM alpine:latest`, at a time when `alpine:latest` also pointed to `alpine:3` and `alpine:3.15` and `alpine:3.15.0`, then it will be impossible to tell which of these tracks you intend to follow when you want to upgrade.

You can use the index to tell that your base layer is `sha256:59bf1c`, and that that matches a whole base image, but there are multiple matches:

```
sha256:59bf1c3509f33515622619af21ed55bbe26d24913cedbca106468a5fb37a50c3 0 * index.docker.io/library/alpine:3 linux/amd64
sha256:59bf1c3509f33515622619af21ed55bbe26d24913cedbca106468a5fb37a50c3 0 * index.docker.io/library/alpine:3.15.0 linux/amd64
sha256:59bf1c3509f33515622619af21ed55bbe26d24913cedbca106468a5fb37a50c3 0 * index.docker.io/library/alpine:latest linux/amd64
sha256:59bf1c3509f33515622619af21ed55bbe26d24913cedbca106468a5fb37a50c3 0 * index.docker.io/library/alpine:3.15 linux/amd64
```

When `:latest` is updated to point to something else, it would presumably also change `:3` and `:3.15` to point to something else, and we'd eventually only be able to tell you were based on `:3.15.0`.

So if you were `FROM :latest`, using this index, you'd eventually only be able to upgrade you along the most specific (i.e., slowest moving) matching base image.

_(This is a good reason not to `FROM :latest`!)_
