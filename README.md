bindmapfuse
===========

bindmapfuse is a FUSE filesystem that exposes an underlying filesystem according to a potentially very large set of bind mounts. It was developed to solve a problem wherein a very large number (hundreds of thousands) of bind mounts were desired in a docker container, but the docker daemon did not support this because the container configuration was too large.

Usage
-----
Create a YAML (or JSON) configuration file that contains a `mounts` key, which is a map of "mountpoints" and real paths (i.e. paths on the underlying filesystem). Like bind mounts with docker, both files and directories can be bind mounted.

Example YAML configuration (`example.yaml`):
```yaml
mounts:
  file1.txt: "/tmp/foo/bar/foobarfile.txt"
  dir1: "/tmp/baz"
  file2.txt: "/tmp/qux/quxfile.txt"
  "dir1/qux": "/tmp/qux"
```

This can then be mounted at a `/tmp/mnt` mountpoint by running bindmapfuse:
```
$ bindmapfuse /tmp/mnt -o bind_map_config=example.yaml &
```

After which the specified binds will be available under the mount point:
```
$ ls -R /tmp/mnt
/tmp/mnt:
dir1  file1.txt  file2.txt

/tmp/mnt/dir1:
bazfile.txt  qux

/tmp/mnt/dir1/qux:
quxfile.txt
```