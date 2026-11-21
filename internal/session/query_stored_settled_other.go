//go:build !(aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package session

import "os"

// platformFileID is empty where FileInfo carries no stable file id at stat time;
// size and modification time alone identify the metadata file there.
type platformFileID struct{}

func platformFileIDOf(os.FileInfo) platformFileID {
	return platformFileID{}
}
