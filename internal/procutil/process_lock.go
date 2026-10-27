package procutil

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

func ownsFileLock(locks io.Reader, pid int, major, minor, inode uint64) (bool, error) {
	scanner := bufio.NewScanner(locks)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 8 || fields[1] != "FLOCK" || fields[2] != "ADVISORY" ||
			fields[3] != "WRITE" || fields[4] != strconv.Itoa(pid) || fields[6] != "0" || fields[7] != "EOF" {
			continue
		}
		identity := strings.Split(fields[5], ":")
		if len(identity) != 3 {
			continue
		}
		lockMajor, majorErr := strconv.ParseUint(identity[0], 16, 64)
		lockMinor, minorErr := strconv.ParseUint(identity[1], 16, 64)
		lockInode, inodeErr := strconv.ParseUint(identity[2], 10, 64)
		if majorErr == nil && minorErr == nil && inodeErr == nil &&
			lockMajor == major && lockMinor == minor && lockInode == inode {
			return true, nil
		}
	}
	return false, scanner.Err()
}
