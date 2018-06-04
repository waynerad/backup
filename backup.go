package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

type wfileInfo struct {
	filePath string
	fileSize int64
	fileTime time.Time
}

type wfileSortSlice struct {
	theSlice []wfileInfo
}

func isSkippableErrorMessage(message string) bool {
	if message[len(message)-17:] == "permission denied" {
		fmt.Println(message[len(message)-17:])
		return true
	}
	if message[len(message)-33:] == "operation not supported on socket" {
		fmt.Println(message[len(message)-33:])
		return true
	}
	if message[len(message)-25:] == "no such file or directory" {
		fmt.Println(message[len(message)-25:])
		return true
	}
	return false
}

func getDirectoryTree(path string, result []wfileInfo, skipIfPermissionDenied bool) []wfileInfo {
	// separator := "\\" // WINDOWS-specific
	separator := "/"
	dir, err := os.Open(path)
	if err != nil {
		if skipIfPermissionDenied {
			if isSkippableErrorMessage(err.Error()) {
				return result
			}
		}
		fmt.Println("Error opening directory for scanning")
		fmt.Println(err)
		os.Exit(1)
	}
	defer dir.Close()
	filesInDir, err := dir.Readdir(0)
	if err != nil {
		if skipIfPermissionDenied {
			if isSkippableErrorMessage(err.Error()) {
				return result
			}
		}
		fmt.Println("Error reading directory")
		fmt.Println(err)
		os.Exit(1)
	}
	for _, filestuff := range filesInDir {
		if filestuff.Name() != "spool" {
			completePath := path + separator + filestuff.Name()
			if filestuff.IsDir() {
				result = getDirectoryTree(completePath, result, skipIfPermissionDenied)
			} else {
				result = append(result, wfileInfo{completePath, filestuff.Size(), filestuff.ModTime()})
			}
		}
	}
	return result
}

func (ptr *wfileSortSlice) Len() int {
	return len(ptr.theSlice)
}

func (ptr *wfileSortSlice) Less(i, j int) bool {
	return ptr.theSlice[i].filePath < ptr.theSlice[j].filePath
}

func (ptr *wfileSortSlice) Swap(i, j int) {
	filePath := ptr.theSlice[i].filePath
	ptr.theSlice[i].filePath = ptr.theSlice[j].filePath
	ptr.theSlice[j].filePath = filePath
	fileSize := ptr.theSlice[i].fileSize
	ptr.theSlice[i].fileSize = ptr.theSlice[j].fileSize
	ptr.theSlice[j].fileSize = fileSize
	fileTime := ptr.theSlice[i].fileTime
	ptr.theSlice[i].fileTime = ptr.theSlice[j].fileTime
	ptr.theSlice[j].fileTime = fileTime
}

func concurrentGetTree(path string, ch chan []wfileInfo, skipIfPermissionDenied bool) {
	var sortSlice wfileSortSlice

	tree := make([]wfileInfo, 0)
	tree = getDirectoryTree(path, tree, skipIfPermissionDenied)
	sortSlice.theSlice = tree
	sort.Sort(&sortSlice)
	ch <- tree
}

func lastslash(str string) int {
	var i int
	var rv int
	var lx int

	i = 0
	rv = -1
	lx = len(str)
	for i < lx {
		if (str[i] == "/"[0]) || (str[i] == "\\"[0]) {
			rv = i
		}
		i++
	}
	return rv
}

// This function scans two directories, sourcePath and destPath, then sorts the
// results (using the full paths to files in subdirectories within these
// starting source and destination directories), then walks through the source
// and destination lists in alphabetical order where it's easy to see if a file
// is present in one list but missing in the other. If a file is present in the
// source but missing in the destination, it gets copied. If it's present in
// the destination but missing in the source, the destination file gets
// deleted. If it's present in both, the file size and time are compared to see
// if they are the same. If they are different, the file is copies from source
// to destination and the old destination file is overwritten.
//
// The system optionally uses concurrency to scan the source and destination
// trees at the same time. It's a good concurrency demo in Golang
func backup(sourcePath string, destPath string, concurrent bool, doDeletes bool, skipIfPermissionDenied bool) {
	var sourceTree []wfileInfo
	var destTree []wfileInfo
	var sortSlice wfileSortSlice

	fmt.Println("")
	fmt.Println("Backing up " + sourcePath + " to " + destPath)
	startTime := time.Now()
	if concurrent {
		fmt.Println("Scanning source and destination trees concurrently.")
		receivedSource := false
		receivedDest := false
		chSource := make(chan []wfileInfo)
		chDest := make(chan []wfileInfo)
		go concurrentGetTree(sourcePath, chSource, skipIfPermissionDenied)
		go concurrentGetTree(destPath, chDest, skipIfPermissionDenied)
		for (!receivedSource) || (!receivedDest) {
			select {
			case sourceTree = <-chSource:
				fmt.Println("Source tree scanned.")
				receivedSource = true
			case destTree = <-chDest:
				fmt.Println("Destination tree scanned.")
				receivedDest = true
			}
		}
	} else {
		fmt.Println("Scanning sequentially.")
		fmt.Println("Scanning source tree.")
		sourceTree = make([]wfileInfo, 0)
		sourceTree = getDirectoryTree(sourcePath, sourceTree, skipIfPermissionDenied)
		sortSlice.theSlice = sourceTree
		sort.Sort(&sortSlice)

		fmt.Println("Scanning destination tree.")
		destTree = make([]wfileInfo, 0)
		destTree = getDirectoryTree(destPath, destTree, skipIfPermissionDenied)
		sortSlice.theSlice = destTree
		sort.Sort(&sortSlice)
	}
	stopTime := time.Now()
	duration := stopTime.Sub(startTime)
	fmt.Println("duration", duration)

	// Do actual backup
	fmt.Println("Doing backup.")

	sourceIdx := 0
	destIdx := 0
	sourceOffset := len(sourcePath)
	destOffset := len(destPath)
	timeDuration, err := time.ParseDuration("1s") // 1 second -- can be modified to take into account time zone differences if applicable
	if err != nil {
		fmt.Println("Error parsing time duration")
		fmt.Println(err)
		os.Exit(1)
	}
	for (sourceIdx < len(sourceTree)) || (destIdx < len(destTree)) {
		toCopy := -1   // -1 is used as a magic value to indicate not to copy
		toDelete := -1 // -1 is used as a magic value to indicate not to delete
		if sourceIdx == len(sourceTree) {
			// off end of source tree
			toDelete = destIdx
			destIdx++
		} else {
			sourceCompare := sourceTree[sourceIdx].filePath[sourceOffset:]
			if destIdx == len(destTree) {
				// off end of destination tree
				toCopy = sourceIdx
				sourceIdx++
			} else {
				destCompare := destTree[destIdx].filePath[destOffset:]
				if sourceCompare == destCompare {
					// both files exist -- same time & date?
					if sourceTree[sourceIdx].fileSize != destTree[destIdx].fileSize {
						toCopy = sourceIdx
					} else {
						if destTree[destIdx].fileTime.Add(timeDuration).Before(sourceTree[sourceIdx].fileTime) {
							toCopy = sourceIdx
						}
					}
					sourceIdx++
					destIdx++
				} else {
					if sourceCompare < destCompare {
						toCopy = sourceIdx
						sourceIdx++
					} else {
						toDelete = destIdx
						destIdx++
					}
				}
			}
		}
		if toCopy >= 0 {
			destination := destPath + sourceTree[toCopy].filePath[len(sourcePath):]
			fmt.Println("Copy:", sourceTree[toCopy].filePath, "=>", destination)
			filehandleSource, err := os.Open(sourceTree[toCopy].filePath)
			permissiondenied := false
			if err != nil {
				if skipIfPermissionDenied {
					if isSkippableErrorMessage(err.Error()) {
						permissiondenied = true
					}
				}
				if !permissiondenied {
					fmt.Println("Error opening source file")
					fmt.Println(err)
					os.Exit(1)
				}
			}
			if !permissiondenied {
				defer filehandleSource.Close()
				filehandleDest, err := os.Create(destination)
				if err != nil {
					// could not open file -- maybe directory doesn't exist? Create and try again, otherwise give up for good
					directory := destination[0:lastslash(destination)]
					fmt.Println("Create directory:", directory)
					err = os.MkdirAll(directory, 0777)
					if err != nil {
						fmt.Println("Error creating subdirectory")
						fmt.Println(err)
						os.Exit(1)
					}
					filehandleDest, err = os.Create(destination)
					if err != nil {
						fmt.Println("Error opening destination file for writing")
						fmt.Println(err)
						os.Exit(1)
					}
				}
				defer filehandleDest.Close()
				io.Copy(filehandleDest, filehandleSource)
				filehandleDest.Close()
				filehandleSource.Close()
				fileinfo, err := os.Stat(sourceTree[toCopy].filePath)
				if err != nil {
					fmt.Println("Error stating source file to obtain file times")
					fmt.Println(err)
					os.Exit(1)
				}
				atime := fileinfo.ModTime()
				mtime := fileinfo.ModTime()
				os.Chtimes(destination, atime, mtime)
			}
		}
		// BUGBUG deletes files but does not remove directories
		if toDelete >= 0 {
			if doDeletes {
				fmt.Println("Delete:", destTree[toDelete].filePath)
				err = os.Remove(destTree[toDelete].filePath)
				if err != nil {
					fmt.Println("Error deleting destination file flagged for deletion")
					fmt.Println(err)
					os.Exit(1)
				}
			} else {
				// fmt.Println("Would delete:", destTree[toDelete].filePath)
			}
		}
	}
}

func main() {
	backup("/source/directory", "/destination/directory/", true, true, false)
	fmt.Println("Done.")
}
