package changes

import (
	"fmt"
	"os"
	"syscall"
)

func statIdentity(_ *os.File, info os.FileInfo) (string, error) {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return fmt.Sprintf("%d:%d:%d", st.Ino, st.Ctim.Sec, st.Ctim.Nsec), nil
	}
	return "", nil
}
