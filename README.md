# `image-scraper`

[![crawl](https://github.com/imjasonh/image-scraper/actions/workflows/crawl.yaml/badge.svg)](https://github.com/imjasonh/image-scraper/actions/workflows/crawl.yaml)

![manifests cached](./manifests-cached.svg)
![layers indexed](./layers-indexed.svg)

This repo periodically crawls popular base images and tracks what tags exist, and what digests they point to.

If you'd like to start tracking another base image, [send a PR to add it here](https://github.com/imjasonh/image-scraper/edit/main/images.txt)!

## How is this useful?

It scrapes and caches image and index manifests. This could be useful if you want to avoid rate limiting on manifest `GET`s, but GitHub also rate-limits, so I doubt it's very useful. The crawl would lag up to an hour anyway (longer if it's broken and I haven't fixed it yet!).

Since history is also preserved here, you could use this repo to go back in time and discover what a tag pointed to at some point in the repo's (very short) history.

For example, to see if the digest of `alpine:latest` has changed since a week ago:

```
$ img=index.docker.io/library/alpine:latest
$ diff <(git show "HEAD@{1 week ago}":$img | jq -r .digest) <(cat $img | jq -r .digest)
1c1
< sha256:21a3deaa0d32a8057914f36584b5288d2e5ecc984380bc0118285c70fa8c9300
---
> sha256:6af1b11bbb17f4c311e269db6530e4da2738262af5fd9064ccdf109b765860fb
```

GitHub also provides an undocumented RSS feed feature, which even lets you filter changes in certain paths.
For example, you can subscribe to [changes to tags in `gcr.io/distroless/static`](https://github.com/imjasonh/image-scraper/commits/main.atom?path=gcr.io/distroless/static) in your RSS reader of choice.
Party like it's 2009!

But the main reason I wrote this is to try to detect base images for built images.

### Base Layer Index

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

When `:latest` is updated to point to something else, it might also update `:3` and `:3.15` to point to something else, and if so we'd eventually only be able to tell you were based on `:3.15.0` specifically.

If `:3.15.0` is also the last `:3.15`, or the last `:3`, then it will always be ambiguous with the other equivalent tags, and no rebuild will be needed.

So if you were `FROM :latest`, using this index, you'd eventually only be able to upgrade you along the most specific (i.e., slowest moving) matching base image.

_(This is a good reason not to `FROM :latest`!)_

(All of the above assumes sane semver tagging. I have bad news for you about sane semver tagging...)

---

[`cmd/detect`](./cmd/detect) is an early prototype that uses the index to detect possible base images.

```
$ go run ./cmd/detect gcr.io/imjasonh/alpine-with-git
2022/03/16 10:38:06 possible match at layer 0: index.docker.io/library/alpine:3.14.2 linux/amd64
2022/03/16 10:38:06 single matching base image found!
index.docker.io/library/alpine:3.14.2
```

This image was indeed built using a [Dockerfile `FROM alpine:3.14.2`](./Dockerfile)!

Having detected this, we could attempt to parse the semver tag and find there's a newer `:3.14`/`:3.14.3`, and even a newer `:3`/`:3.15`/`:3.15.0`.
We could then attempt to rebuild the image on those new bases.
