bindmapfuse
===========

bindmapfuse is a FUSE filesystem that exposes an underlying filesystem according to a potentially very large set of bind mounts. It was developed to solve a problem wherein a very large number (hundreds of thousands) of bind mounts were desired in a docker container, but the docker daemon did not support this because the container configuration was too large.

Example Usage
-------------
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

If the underlying files do not exist, that will cause a "No such file or directory"
error, though at present they will still be listed in the directory listing but
without any stat information:
```
$ ls -lR /tmp/mnt
/tmp/mnt:
ls: cannot access '/tmp/mnt/dir1': No such file or directory
ls: cannot access '/tmp/mnt/file1.txt': No such file or directory
ls: cannot access '/tmp/mnt/file2.txt': No such file or directory
total 0
?????????? ? ? ? ?            ? dir1
?????????? ? ? ? ?            ? file1.txt
?????????? ? ? ? ?            ? file2.txt
```

You can create suitable example files and directories on the underlying filesystem:
```
$ mkdir /tmp/foo /tmp/foo/bar /tmp/baz /tmp/qux
$ echo foobar > /tmp/foo/bar/foobarfile.txt
$ echo baz > /tmp/baz/bazfile.txt
$ echo qux > /tmp/qux/quxfile.txt
```

After which the specified bind mounts will be available under the mount point:
```
$ ls -R /tmp/mnt
/tmp/mnt:
dir1  file1.txt  file2.txt

/tmp/mnt/dir1:
bazfile.txt  qux

/tmp/mnt/dir1/qux:
quxfile.txt
```

