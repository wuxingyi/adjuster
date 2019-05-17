# adjuster for updating bcache minimum writeback rate
# how to build
## 1、modify directory name to adjuster-1.0
```
cd ..
mv adjuster adjuster-1.0
```
## 2、get a tar.gz tarball
```
tar -czf adjuster-1.0.tar.gz adjuster-1.0/
```
## 3、generate origin source files
```
debmake
```
## 4、build debian package
```
tar -czf adjuster-1.0.tar.gz adjuster-1.0/
```
result is something like:
```
total 3372
drwxr-xr-x  3 root root    4096 May 17 08:08 .
drwxrwxrwt 19 root root   12288 May 17 08:11 ..
drwxr-xr-x  4 root root    4096 May 17 08:13 adjuster-1.0
-rw-r--r--  1 root root  678192 May 17 08:08 adjuster_1.0-1_all.deb
-rw-r--r--  1 root root    5325 May 17 08:08 adjuster_1.0-1_amd64.buildinfo
-rw-r--r--  1 root root    1720 May 17 08:08 adjuster_1.0-1_amd64.changes
-rw-r--r--  1 root root    1312 May 17 08:08 adjuster_1.0-1.debian.tar.xz
-rw-r--r--  1 root root     813 May 17 08:08 adjuster_1.0-1.dsc
lrwxrwxrwx  1 root root      19 May 17 08:08 adjuster_1.0.orig.tar.gz -> adjuster-1.0.tar.gz
-rw-r--r--  1 root root 2731687 May 17 08:08 adjuster-1.0.tar.gz
```




