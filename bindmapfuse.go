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
	realPath string
}

type Bmfs struct {
	fuse.FileSystemBase
	root string
	mounts map[string]*Node
}

func (self *Bmfs) Init() {
	defer trace()()
	e := syscall.Chdir(self.root)
	if nil == e {
		self.root = "./"
	}
}

func (self *Bmfs) resolvePath(path string) string {
	mount := filepath.Clean(path)
	afterMount := ""
	if filepath.IsAbs(mount) {
		if len(mount) > 1 {
			mount = mount[1:]
		} else {
			mount = ""
		}
	}
	mappedMount := ""
	for mount != "" {
		log.Printf("resolvePath: path=%s mount=%s afterMount=%s", path, mount, afterMount)
		node, ok := self.mounts[mount]
		if ok {
			log.Printf("resolvePath: mapped mount=%s to node=%+v", mount, node)
			mappedMount = filepath.Join(node.realPath, afterMount)
			break
		}
		var file string
		mount, file = filepath.Split(mount)
		if mount[len(mount)-1:] == "/" {
			mount = mount[:len(mount)-1]
		}		
		if file == "" {
			log.Printf("resolvePath: failed to map path=%s mounts=%v", path, self.mounts)
			mappedMount = filepath.Join(mount, afterMount)
			break
		} else {
			afterMount = filepath.Join(file, afterMount)
		}
	}
	if !filepath.IsAbs(mappedMount) {
		mappedMount = filepath.Join(self.root, mappedMount)
	}
	log.Printf("resolvePath: path=%s mappedMount=%s", path, mappedMount)
	return mappedMount
}

func (self *Bmfs) Statfs(path string, stat *fuse.Statfs_t) (errc int) {
	defer trace(path)(&errc, stat)
	path = self.resolvePath(path)
	stgo := syscall.Statfs_t{}
	errc = errno(syscall_Statfs(path, &stgo))
	copyFusestatfsFromGostatfs(stat, &stgo)
	return
}

func (self *Bmfs) Mknod(path string, mode uint32, dev uint64) (errc int) {
	defer trace(path, mode, dev)(&errc)
	defer setuidgid()()
	path = self.resolvePath(path)
	return errno(syscall.Mknod(path, mode, int(dev)))
}

func (self *Bmfs) Mkdir(path string, mode uint32) (errc int) {
	defer trace(path, mode)(&errc)
	defer setuidgid()()
	path = self.resolvePath(path)
	return errno(syscall.Mkdir(path, mode))
}

func (self *Bmfs) Unlink(path string) (errc int) {
	defer trace(path)(&errc)
	path = self.resolvePath(path)
	return errno(syscall.Unlink(path))
}

func (self *Bmfs) Rmdir(path string) (errc int) {
	defer trace(path)(&errc)
	path = self.resolvePath(path)
	return errno(syscall.Rmdir(path))
}

func (self *Bmfs) Link(oldpath string, newpath string) (errc int) {
	defer trace(oldpath, newpath)(&errc)
	defer setuidgid()()
	oldpath = self.resolvePath(oldpath)
	newpath = self.resolvePath(newpath)
	return errno(syscall.Link(oldpath, newpath))
}

func (self *Bmfs) Symlink(target string, newpath string) (errc int) {
	defer trace(target, newpath)(&errc)
	defer setuidgid()()
	newpath = self.resolvePath(newpath)
	return errno(syscall.Symlink(target, newpath))
}

func (self *Bmfs) Readlink(path string) (errc int, target string) {
	defer trace(path)(&errc, &target)
	path = self.resolvePath(path)
	buff := [1024]byte{}
	n, e := syscall.Readlink(path, buff[:])
	if nil != e {
		return errno(e), ""
	}
	return 0, string(buff[:n])
}

func (self *Bmfs) Rename(oldpath string, newpath string) (errc int) {
	defer trace(oldpath, newpath)(&errc)
	defer setuidgid()()
	oldpath = self.resolvePath(oldpath)
	newpath = self.resolvePath(newpath)
	return errno(syscall.Rename(oldpath, newpath))
}

func (self *Bmfs) Chmod(path string, mode uint32) (errc int) {
	defer trace(path, mode)(&errc)
	path = self.resolvePath(path)
	return errno(syscall.Chmod(path, mode))
}

func (self *Bmfs) Chown(path string, uid uint32, gid uint32) (errc int) {
	defer trace(path, uid, gid)(&errc)
	path = self.resolvePath(path)
	return errno(syscall.Lchown(path, int(uid), int(gid)))
}

func (self *Bmfs) Utimens(path string, tmsp1 []fuse.Timespec) (errc int) {
	defer trace(path, tmsp1)(&errc)
	path = self.resolvePath(path)
	tmsp := [2]syscall.Timespec{}
	tmsp[0].Sec, tmsp[0].Nsec = tmsp1[0].Sec, tmsp1[0].Nsec
	tmsp[1].Sec, tmsp[1].Nsec = tmsp1[1].Sec, tmsp1[1].Nsec
	return errno(syscall.UtimesNano(path, tmsp[:]))
}

func (self *Bmfs) Create(path string, flags int, mode uint32) (errc int, fh uint64) {
	defer trace(path, flags, mode)(&errc, &fh)
	defer setuidgid()()
	return self.open(path, flags, mode)
}

func (self *Bmfs) Open(path string, flags int) (errc int, fh uint64) {
	defer trace(path, flags)(&errc, &fh)
	return self.open(path, flags, 0)
}

func (self *Bmfs) open(path string, flags int, mode uint32) (errc int, fh uint64) {
	path = self.resolvePath(path)
	f, e := syscall.Open(path, flags, mode)
	if nil != e {
		return errno(e), ^uint64(0)
	}
	return 0, uint64(f)
}

func (self *Bmfs) Getattr(path string, stat *fuse.Stat_t, fh uint64) (errc int) {
	defer trace(path, fh)(&errc, stat)
	stgo := syscall.Stat_t{}
	if ^uint64(0) == fh {
		path = self.resolvePath(path)
		errc = errno(syscall.Lstat(path, &stgo))
	} else {
		errc = errno(syscall.Fstat(int(fh), &stgo))
	}
	copyFusestatFromGostat(stat, &stgo)
	return
}

func (self *Bmfs) Truncate(path string, size int64, fh uint64) (errc int) {
	defer trace(path, size, fh)(&errc)
	if ^uint64(0) == fh {
		path = self.resolvePath(path)
		errc = errno(syscall.Truncate(path, size))
	} else {
		errc = errno(syscall.Ftruncate(int(fh), size))
	}
	return
}

func (self *Bmfs) Read(path string, buff []byte, ofst int64, fh uint64) (n int) {
	defer trace(path, buff, ofst, fh)(&n)
	n, e := syscall.Pread(int(fh), buff, ofst)
	if nil != e {
		return errno(e)
	}
	return n
}

func (self *Bmfs) Write(path string, buff []byte, ofst int64, fh uint64) (n int) {
	defer trace(path, buff, ofst, fh)(&n)
	n, e := syscall.Pwrite(int(fh), buff, ofst)
	if nil != e {
		return errno(e)
	}
	return n
}

func (self *Bmfs) Release(path string, fh uint64) (errc int) {
	defer trace(path, fh)(&errc)
	return errno(syscall.Close(int(fh)))
}

func (self *Bmfs) Fsync(path string, datasync bool, fh uint64) (errc int) {
	defer trace(path, datasync, fh)(&errc)
	return errno(syscall.Fsync(int(fh)))
}

func (self *Bmfs) Opendir(path string) (errc int, fh uint64) {
	defer trace(path)(&errc, &fh)
	path = self.resolvePath(path)
	f, e := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if nil != e {
		return errno(e), ^uint64(0)
	}
	return 0, uint64(f)
}

func (self *Bmfs) Readdir(path string,
	fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64,
	fh uint64) (errc int) {
	defer trace(path, fill, ofst, fh)(&errc)
	path = self.resolvePath(path)
	file, e := os.Open(path)
	if nil != e {
		return errno(e)
	}
	defer file.Close()
	nams, e := file.Readdirnames(0)
	if nil != e {
		return errno(e)
	}
	nams = append([]string{".", ".."}, nams...)
	for _, name := range nams {
		if !fill(name, nil, 0) {
			break
		}
	}
	return 0
}

func (self *Bmfs) Releasedir(path string, fh uint64) (errc int) {
	defer trace(path, fh)(&errc)
	return errno(syscall.Close(int(fh)))
}

type BindMapConfig struct {
	Root string `json:"root"`
	Mounts map[string]string `json:"mounts"`
}

func main() {
	syscall.Umask(0)
	bmfs := Bmfs{}
	var configFileSet bool
	var configFilePath string
	var bindMapConfig BindMapConfig
	args, err := fuse.OptParse(os.Args, "bind_map_config= bind_map_config", &configFileSet, &configFilePath)
	if err != nil {
		log.Fatalf("bindmapfuse: error parsing command-line options: %v", err)
	}
	if configFileSet {
		configFileBytes, err := ioutil.ReadFile(configFilePath)
		if err != nil {
			log.Fatalf("bindmapfuse: error reading bind map config file %s: %v", configFilePath, err)
		}
		err = yaml.Unmarshal(configFileBytes, &bindMapConfig)
		if err != nil {
			log.Fatalf("bindmapfuse: error parsing bind map config file contents as YAML/JSON: %v", err)
		}
		log.Printf("bindmapfuse: have bindMapConfig=%+v", bindMapConfig)
	}
	bmfs.mounts = make(map[string]*Node)
	for m, p := range bindMapConfig.Mounts {
		m = filepath.Clean(m)
		bmfs.mounts[m] = &Node{realPath: p}
		for _, c := range strings.Split(filepath.ToSlash(m), "/") {
			_, ok := bmfs.mounts[c]
			if !ok {
				bmfs.mounts[c] = &Node{}
			}
		}
	}
	if bindMapConfig.Root != "" {
		bmfs.root, err = filepath.Abs(bindMapConfig.Root)
		if err != nil {
			log.Fatalf("bindmapfuse: error setting root path to %s: %v", bindMapConfig.Root, err)
		}
	}
	_host = fuse.NewFileSystemHost(&bmfs)
	_host.Mount("", args[1:])
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
