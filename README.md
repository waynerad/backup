# backup
Concurrent backup program written in Golang

backup.go is a simple program that compares two directories, including recursing into all subdirectories, and copies files in the soure directory tree to the destination conspiracy tree until the two are identical. It uses the file times and sizes to determine if a copy needs to be made (it does not look at the actual contents).

Because scanning the source and destination trees are typically on separate physical devices (such as an internal HD to an external HD), it makes sense to scan both of them concurrently. This program does that and as such makes an ideal demo of concurrency written in Go -- it is a simple, straightforward demonstration of how to do concurrency in Go, but at the same time a real program that does something useful.

This program has the parameters to backup as parameters to backup() called in main(). In practice I just modify main() to do whatever backup chores I want, rather than using command line parameters. The program could be easily modified to use command line parameters, using Go's flag package, if that suits you.

If you want to use the program on Windows, you'll need to change the separator from forward slash to backslash in the getDirectoryTree() function. If you want to use the same code on Window as Mac/Linux, you could modify it to check the OS version (I use only Mac/Linux so didn't bother).

