/*******************************************************************************
 * Copyright (c) 2018 Genome Research Ltd.
 *
 * Author: Joshua C. Randall <jcrandall@alum.mit.edu>
 *
 * This file is part of bindmapfuse.
 *
 * bindmapfuse is free software: you can redistribute it and/or modify it under
 * the terms of the GNU Affero General Public License as published by the Free
 * Software Foundation; either version 3 of the License, or (at your option) any
 * later version.
 *
 * This program is distributed in the hope that it will be useful, but WITHOUT
 * ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS
 * FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License for more
 * details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 ******************************************************************************/

/*
 * Portions are based on examples from the Cgofuse project:
 *   https://github.com/billziss-gh/cgofuse
 *
 * Copyright 2017-2018 Bill Zissimopoulos
 *
 * Licensed under the MIT license:
 *
 * MIT License
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/billziss-gh/cgofuse/fuse"
	"github.com/ghodss/yaml"
)

var (
	TracePattern = os.Getenv("BINDMAPFUSE_TRACE")
)

func traceJoin(deref bool, vals []interface{}) string {
	rslt := ""
	for _, v := range vals {
		if deref {
			switch i := v.(type) {
			case *bool:
				rslt += fmt.Sprintf(", %#v", *i)
			case *int:
				rslt += fmt.Sprintf(", %#v", *i)
			case *int8:
				rslt += fmt.Sprintf(", %#v", *i)
			case *int16:
				rslt += fmt.Sprintf(", %#v", *i)
			case *int32:
				rslt += fmt.Sprintf(", %#v", *i)
			case *int64:
				rslt += fmt.Sprintf(", %#v", *i)
			case *uint:
				rslt += fmt.Sprintf(", %#v", *i)
			case *uint8:
				rslt += fmt.Sprintf(", %#v", *i)
			case *uint16:
				rslt += fmt.Sprintf(", %#v", *i)
			case *uint32:
				rslt += fmt.Sprintf(", %#v", *i)
			case *uint64:
				rslt += fmt.Sprintf(", %#v", *i)
			case *uintptr:
				rslt += fmt.Sprintf(", %#v", *i)
			case *float32:
				rslt += fmt.Sprintf(", %#v", *i)
			case *float64:
				rslt += fmt.Sprintf(", %#v", *i)
			case *complex64:
				rslt += fmt.Sprintf(", %#v", *i)
			case *complex128:
				rslt += fmt.Sprintf(", %#v", *i)
			case *string:
				rslt += fmt.Sprintf(", %#v", *i)
			default:
				rslt += fmt.Sprintf(", %#v", v)
			}
		} else {
			rslt += fmt.Sprintf(", %#v", v)
		}
	}
	if len(rslt) > 0 {
		rslt = rslt[2:]
	}
	return rslt
}

func Trace(skip int, prfx string, vals ...interface{}) func(vals ...interface{}) {
	if "" == TracePattern {
		return func(vals ...interface{}) {
		}
	}
	pc, _, _, ok := runtime.Caller(skip + 1)
	name := "<UNKNOWN>"
	if ok {
		fn := runtime.FuncForPC(pc)
		name = fn.Name()
		if m, _ := filepath.Match(TracePattern, name); !m {
			return func(vals ...interface{}) {
			}
		}
	}
	if "" != prfx {
		prfx = prfx + ": "
	}
	args := traceJoin(false, vals)
	return func(vals ...interface{}) {
		form := "%v%v(%v) = %v"
		rslt := ""
		rcvr := recover()
		if nil != rcvr {
			rslt = fmt.Sprintf("!PANIC:%v", rcvr)
		} else {
			if len(vals) != 1 {
				form = "%v%v(%v) = (%v)"
			}
			rslt = traceJoin(true, vals)
		}
		log.Printf(form, prfx, name, args, rslt)
		if nil != rcvr {
			panic(rcvr)
		}
	}
}

func trace(vals ...interface{}) func(vals ...interface{}) {
	uid, gid, _ := fuse.Getcontext()
	return Trace(1, fmt.Sprintf("[uid=%v,gid=%v]", uid, gid), vals...)
}

func errno(err error) int {
	if nil != err {
		return -int(err.(syscall.Errno))
	} else {
		return 0
	}
}

var (
	_host *fuse.FileSystemHost
)

type Node struct {
	name string
	path string
	realPath string
	parent *Node
	mounts map[string]*Node
}

func NewNode(name string, realPath string, parent *Node) (n *Node) {
	n = &Node{}
	n.name = name
	n.parent = parent
	if n.parent != nil {
		n.path = filepath.Join(parent.path, name)
	}
	n.realPath = realPath
	n.mounts = make(map[string]*Node)
	return
}

func (n *Node) AddMount(c *Node) {
	_, exists := n.mounts[c.name]
	if !exists {
		n.mounts[c.name] = c
	} else {
		log.Fatalf("bindmapfuse: attempted to add mount over existing mount %s at %s", c.name, n.path)
	}
}

func (n *Node) HasMount(name string) bool {
	_, exists := n.mounts[name]
	return exists
}

func (n *Node) GetMount(name string) *Node {
	return n.mounts[name]
}

func (n *Node) ListMountNames() (names []string) {
	names = make([]string, len(n.mounts))
	i := 0
	for mount := range n.mounts {
		names[i] = mount
		i++
	}
	return
}

func (n *Node) EnsureDescendentNode(mountPath string, realPath string) {
	childName, relPath := n.splitPathFirstRest(mountPath)
	if relPath != "" {
		if !n.HasMount(childName) {
			n.AddMount(NewNode(childName, "", n))
		}
		n.GetMount(childName).EnsureDescendentNode(relPath, realPath)
	} else {
		if !n.HasMount(childName) {
			n.AddMount(NewNode(childName, realPath, n))
		} else {
			child := n.GetMount(childName)
			oldRealPath := child.RealPath()
			if oldRealPath != "" {
				log.Printf("bindmapfuse: overriding real path '%s' with '%s'", oldRealPath, realPath)
			}
			child.SetRealPath(realPath)
		}
	}
}

func (n *Node) IsRoot() bool {
	return n.parent == nil
}

func (n *Node) IsVirtual() bool {
	return n.realPath == ""
}

func (n *Node) SetRealPath(realPath string) {
	n.realPath = realPath
}

func (n *Node) RealPath() (realPath string) {
	if n.realPath != "" {
		return n.realPath
	} else {
		if n.IsRoot() {
			return ""
		} else {
			return filepath.Join(n.parent.RealPath(), n.name)
		}
	}
}

func (n *Node) LookupPath(path string) *Node {
	if path == "/" && n.IsRoot() {
		return n
	}
	childName, relPath := n.splitPathFirstRest(path)
	if n.HasMount(childName) {
		if relPath == "" {
			return n.GetMount(childName)
		} else {
			return n.GetMount(childName).LookupPath(relPath)
		}
	} else {
		return nil
	}
}

func (n *Node) splitPathFirstRest(path string) (first string, rest string) {
	if filepath.IsAbs(path) {
		path = path[1:]
	}
	pathSegments := strings.SplitN(filepath.ToSlash(path), "/", 2)
	first = pathSegments[0]
	rest = ""
	if len(pathSegments) > 1 {
		rest = filepath.Join(pathSegments[1:]...)
	}
	return
}

func (n *Node) ResolvePath(path string) (realPath string) {
	childName, relPath := n.splitPathFirstRest(path)
	if n.HasMount(childName) {
		return n.GetMount(childName).ResolvePath(relPath)
	} else {
		return filepath.Join(n.RealPath(), childName, relPath)
	}
}

type Bmfs struct {
	fuse.FileSystemBase
	initReady chan bool
	root *Node
	debug bool
}

func (bmfs *Bmfs) Init() {
	defer trace()()
	ready := <-bmfs.initReady
	if !ready {
		log.Fatalf("bindmapfuse: initialization failed")
	}
}

func (bmfs *Bmfs) resolvePath(path string) (resolvedPath string) {
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) {
		if len(cleanPath) > 1 {
			cleanPath = cleanPath[1:]
		} else {
			cleanPath = ""
		}
	}
	resolvedPath = bmfs.root.ResolvePath(cleanPath)
	bmfs.debugf("bindmapfuse resolvePath: path=%s resolvedPath=%s", path, resolvedPath)
	return
}

func (bmfs *Bmfs) Statfs(path string, stat *fuse.Statfs_t) (errc int) {
	defer trace(path)(&errc, stat)
	resolvedPath := bmfs.resolvePath(path)
	stgo := syscall.Statfs_t{}
	errc = errno(syscall_Statfs(resolvedPath, &stgo))
	copyFusestatfsFromGostatfs(stat, &stgo)
	return
}

func (bmfs *Bmfs) Mknod(path string, mode uint32, dev uint64) (errc int) {
	defer trace(path, mode, dev)(&errc)
	defer setuidgid()()
	resolvedPath := bmfs.resolvePath(path)
	return errno(syscall.Mknod(resolvedPath, mode, int(dev)))
}

func (bmfs *Bmfs) Mkdir(path string, mode uint32) (errc int) {
	defer trace(path, mode)(&errc)
	defer setuidgid()()
	resolvedPath := bmfs.resolvePath(path)
	return errno(syscall.Mkdir(resolvedPath, mode))
}

func (bmfs *Bmfs) Unlink(path string) (errc int) {
	defer trace(path)(&errc)
	resolvedPath := bmfs.resolvePath(path)
	return errno(syscall.Unlink(resolvedPath))
}

func (bmfs *Bmfs) Rmdir(path string) (errc int) {
	defer trace(path)(&errc)
	resolvedPath := bmfs.resolvePath(path)
	return errno(syscall.Rmdir(resolvedPath))
}

func (bmfs *Bmfs) Link(oldpath string, newpath string) (errc int) {
	defer trace(oldpath, newpath)(&errc)
	defer setuidgid()()
	oldResolvedPath := bmfs.resolvePath(oldpath)
	newResolvedPath := bmfs.resolvePath(newpath)
	return errno(syscall.Link(oldResolvedPath, newResolvedPath))
}

func (bmfs *Bmfs) Symlink(target string, newpath string) (errc int) {
	defer trace(target, newpath)(&errc)
	defer setuidgid()()
	newResolvedPath := bmfs.resolvePath(newpath)
	return errno(syscall.Symlink(target, newResolvedPath))
}

func (bmfs *Bmfs) Readlink(path string) (errc int, target string) {
	defer trace(path)(&errc, &target)
	resolvedPath := bmfs.resolvePath(path)
	buff := [1024]byte{}
	n, e := syscall.Readlink(resolvedPath, buff[:])
	if nil != e {
		return errno(e), ""
	}
	return 0, string(buff[:n])
}

func (bmfs *Bmfs) Rename(oldpath string, newpath string) (errc int) {
	defer trace(oldpath, newpath)(&errc)
	defer setuidgid()()
	oldResolvedPath := bmfs.resolvePath(oldpath)
	newResolvedPath := bmfs.resolvePath(newpath)
	return errno(syscall.Rename(oldResolvedPath, newResolvedPath))
}

func (bmfs *Bmfs) Chmod(path string, mode uint32) (errc int) {
	defer trace(path, mode)(&errc)
	resolvedPath := bmfs.resolvePath(path)
	return errno(syscall.Chmod(resolvedPath, mode))
}

func (bmfs *Bmfs) Chown(path string, uid uint32, gid uint32) (errc int) {
	defer trace(path, uid, gid)(&errc)
	resolvedPath := bmfs.resolvePath(path)
	return errno(syscall.Lchown(resolvedPath, int(uid), int(gid)))
}

func (bmfs *Bmfs) Utimens(path string, tmsp1 []fuse.Timespec) (errc int) {
	defer trace(path, tmsp1)(&errc)
	resolvedPath := bmfs.resolvePath(path)
	tmsp := [2]syscall.Timespec{}
	tmsp[0].Sec, tmsp[0].Nsec = tmsp1[0].Sec, tmsp1[0].Nsec
	tmsp[1].Sec, tmsp[1].Nsec = tmsp1[1].Sec, tmsp1[1].Nsec
	return errno(syscall.UtimesNano(resolvedPath, tmsp[:]))
}

func (bmfs *Bmfs) Create(path string, flags int, mode uint32) (errc int, fh uint64) {
	defer trace(path, flags, mode)(&errc, &fh)
	defer setuidgid()()
	return bmfs.open(path, flags, mode)
}

func (bmfs *Bmfs) Open(path string, flags int) (errc int, fh uint64) {
	defer trace(path, flags)(&errc, &fh)
	return bmfs.open(path, flags, 0)
}

func (bmfs *Bmfs) open(path string, flags int, mode uint32) (errc int, fh uint64) {
	resolvedPath := bmfs.resolvePath(path)
	f, e := syscall.Open(resolvedPath, flags, mode)
	if nil != e {
		return errno(e), ^uint64(0)
	}
	return 0, uint64(f)
}

func (bmfs *Bmfs) Getattr(path string, stat *fuse.Stat_t, fh uint64) (errc int) {
	defer trace(path, fh)(&errc, stat)
	stgo := syscall.Stat_t{}
	if ^uint64(0) == fh {
		resolvedPath := bmfs.resolvePath(path)
		errc = errno(syscall.Lstat(resolvedPath, &stgo))
	} else {
		errc = errno(syscall.Fstat(int(fh), &stgo))
	}
	if errc != 0 {
		node := bmfs.root.LookupPath(path)
		if node != nil && node.IsVirtual() {
			stgo.Mode = fuse.S_IFDIR | 0755
			stgo.Size = 4096
			stgo.Nlink = 2
			stgo.Uid = uint32(os.Getuid())
			stgo.Gid = uint32(os.Getgid())
			errc = 0
		}
	}
	copyFusestatFromGostat(stat, &stgo)
	return
}

func (bmfs *Bmfs) Truncate(path string, size int64, fh uint64) (errc int) {
	defer trace(path, size, fh)(&errc)
	if ^uint64(0) == fh {
		resolvedPath := bmfs.resolvePath(path)
		errc = errno(syscall.Truncate(resolvedPath, size))
	} else {
		errc = errno(syscall.Ftruncate(int(fh), size))
	}
	return
}

func (bmfs *Bmfs) Read(path string, buff []byte, ofst int64, fh uint64) (n int) {
	defer trace(path, buff, ofst, fh)(&n)
	n, e := syscall.Pread(int(fh), buff, ofst)
	if nil != e {
		return errno(e)
	}
	return n
}

func (bmfs *Bmfs) Write(path string, buff []byte, ofst int64, fh uint64) (n int) {
	defer trace(path, buff, ofst, fh)(&n)
	n, e := syscall.Pwrite(int(fh), buff, ofst)
	if nil != e {
		return errno(e)
	}
	return n
}

func (bmfs *Bmfs) Release(path string, fh uint64) (errc int) {
	defer trace(path, fh)(&errc)
	return errno(syscall.Close(int(fh)))
}

func (bmfs *Bmfs) Fsync(path string, datasync bool, fh uint64) (errc int) {
	defer trace(path, datasync, fh)(&errc)
	return errno(syscall.Fsync(int(fh)))
}

func (bmfs *Bmfs) Opendir(path string) (errc int, fh uint64) {
	defer trace(path)(&errc, &fh)
	resolvedPath := bmfs.resolvePath(path)
	f, e := syscall.Open(resolvedPath, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if e != nil {
		node := bmfs.root.LookupPath(path)
		if node != nil && node.IsVirtual() {
			return 0, ^uint64(0)
		}
		return errno(e), ^uint64(0)
	}
	return 0, uint64(f)
}

func (bmfs *Bmfs) Readdir(path string,
	fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64,
	fh uint64) (errc int) {
	defer trace(path, fill, ofst, fh)(&errc)
	node := bmfs.root.LookupPath(path)
	if node != nil {
		for _, name := range node.ListMountNames() {
			if !fill(name, nil, 0) {
				break
			}
		}
	}
	resolvedPath := bmfs.resolvePath(path)
	file, e := os.Open(resolvedPath)
	if nil != e {
		if node != nil {
			return 0
		}
		return errno(e)
	}
	defer file.Close()
	names, e := file.Readdirnames(0)
	if e != nil {
		if node != nil {
			return 0
		}
		return errno(e)
	}
	names = append([]string{".", ".."}, names...)
	for _, name := range names {
		if node != nil {
			if node.HasMount(name) {
				continue
			}
		}
		if !fill(name, nil, 0) {
			break
		}
	}
	return 0
}

func (bmfs *Bmfs) Releasedir(path string, fh uint64) (errc int) {
	defer trace(path, fh)(&errc)
	return errno(syscall.Close(int(fh)))
}

func (bmfs *Bmfs) debugf(format string, v ...interface{}) {
	if bmfs.debug {
		log.Printf(format, v...)
	}
}

type BindMapConfig struct {
	Mounts map[string]string `json:"mounts"`
	Debug bool
}

func main() {
	syscall.Umask(0)
	bmfs := &Bmfs{}
	var configFileSet bool
	var configFilePath string
	args, err := fuse.OptParse(os.Args[1:], "bind_map_config= bind_map_config", &configFileSet, &configFilePath)
	if err != nil {
		log.Fatalf("bindmapfuse: error parsing command-line options: %v", err)
	}
	bmfs.root = NewNode("/", "", nil)

	go func() {
		bmfs.initReady = make(chan bool)
		var bindMapConfig BindMapConfig
		if configFileSet {
			configFileBytes, err := ioutil.ReadFile(configFilePath)
			if err != nil {
				log.Fatalf("bindmapfuse: error reading bind map config file %s: %v", configFilePath, err)
			}
			err = yaml.Unmarshal(configFileBytes, &bindMapConfig)
			if err != nil {
				log.Fatalf("bindmapfuse: error parsing bind map config file contents as YAML/JSON: %v", err)
			}
			bmfs.debug = bindMapConfig.Debug
			bmfs.debugf("bindmapfuse: have bindMapConfig=%+v", bindMapConfig)
		}
		for mountPath, realPath := range bindMapConfig.Mounts {
			mountPath = filepath.Clean(mountPath)
			if filepath.IsAbs(mountPath) {
				mountPath = mountPath[1:]
			}
			bmfs.root.EnsureDescendentNode(mountPath, realPath)
		}
		bmfs.initReady <- true
	}()
	
	_host = fuse.NewFileSystemHost(bmfs)
	_host.Mount("", args)
}

func setuidgid() func() {
	euid := syscall.Geteuid()
	if 0 == euid {
		uid, gid, _ := fuse.Getcontext()
		egid := syscall.Getegid()
		syscall.Setregid(-1, int(gid))
		syscall.Setreuid(-1, int(uid))
		return func() {
			syscall.Setreuid(-1, int(euid))
			syscall.Setregid(-1, int(egid))
		}
	}
	return func() {
	}
}

func copyFusestatfsFromGostatfs(dst *fuse.Statfs_t, src *syscall.Statfs_t) {
	*dst = fuse.Statfs_t{}
	dst.Bsize = uint64(src.Bsize)
	dst.Frsize = 1
	dst.Blocks = uint64(src.Blocks)
	dst.Bfree = uint64(src.Bfree)
	dst.Bavail = uint64(src.Bavail)
	dst.Files = uint64(src.Files)
	dst.Ffree = uint64(src.Ffree)
	dst.Favail = uint64(src.Ffree)
	dst.Namemax = 255 //uint64(src.Namelen)
}

func copyFusestatFromGostat(dst *fuse.Stat_t, src *syscall.Stat_t) {
	*dst = fuse.Stat_t{}
	dst.Dev = uint64(src.Dev)
	dst.Ino = uint64(src.Ino)
	dst.Mode = uint32(src.Mode)
	dst.Nlink = uint32(src.Nlink)
	dst.Uid = uint32(src.Uid)
	dst.Gid = uint32(src.Gid)
	dst.Rdev = uint64(src.Rdev)
	dst.Size = int64(src.Size)
	dst.Atim.Sec, dst.Atim.Nsec = src.Atim.Sec, src.Atim.Nsec
	dst.Mtim.Sec, dst.Mtim.Nsec = src.Mtim.Sec, src.Mtim.Nsec
	dst.Ctim.Sec, dst.Ctim.Nsec = src.Ctim.Sec, src.Ctim.Nsec
	dst.Blksize = int64(src.Blksize)
	dst.Blocks = int64(src.Blocks)
}

func syscall_Statfs(path string, stat *syscall.Statfs_t) error {
	return syscall.Statfs(path, stat)
}
