# Roots

Pulls containers from registries and extracts them into a folder. The resulting
root tree can be used to inspect all the files of a container and it can be run
directly using [systemd-nspawn](https://www.freedesktop.org/software/systemd/man/systemd-nspawn.html):

There are other tools that can accomplish the same thing, but they all do
more than roots does. Roots fetches image layers, extracts them and calls it a day.

```bash
roots pull ubuntu:18.04 ./ubuntu
sudo systemd-nspawn -D ./ubuntu
```

Existing directories can be overwritten using `--force`:

```bash
roots pull ubuntu:18.04 ./ubuntu --force
```

Roots supports checking the digest of images, which is useful to check if
an image has had an update:

```bash
roots digest ubuntu:18.04
```

Roots keeps downloaded layers in a cache. This cache can be purged periodically:

```bash
roots purge
```

Private registries are supported, though currently only the Google Container
Registry has been implemented (pull requests welcome!):

```bash
roots pull gcr.io/google-containers/etcd:3.3.10 ./etcd --auth account.json
```

It is possible to select a specific architecture/os for the image if it supports
multi-arch manifests (a.k.a fat manifests):

```bash
roots pull gcr.io/google-containers/etcd:3.3.10 --arch arm --os linux
```

If the image does not support multiple platforms, using --arch/--os will result
in an error. If the image does support multiple platforms and --arch/--os is
ommitted, the default manifest defined by the registry is used.

## Requirements / Limitations

Roots has only been tested on Linux/MacOS.

## Installation

To install the roots commandline run the following command:

```bash
go get github.com/seantis/roots
```

## Multiple Processes

It is possible to run multiple roots processes at the same time, however its
use is quite limited as cache and destination are locked during pull/purge.

That only leaves the digest operation, which doesn't write anything as well as
the option to use no cache or separate caches with differing destinations.

Feel free to open an issue if you have a use case for this.
