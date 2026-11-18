package changes

import (
	"encoding/binary"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func statIdentity(file *os.File, _ os.FileInfo) (string, error) {
	var basic [40]byte
	if err := windows.GetFileInformationByHandleEx(
		windows.Handle(file.Fd()),
		windows.FileBasicInfo,
		&basic[0],
		uint32(len(basic)),
	); err != nil {
		return "", err
	}
	var id [24]byte
	if err := windows.GetFileInformationByHandleEx(
		windows.Handle(file.Fd()),
		windows.FileIdInfo,
		&id[0],
		uint32(len(id)),
	); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x:%d", id, binary.LittleEndian.Uint64(basic[24:32])), nil
}
