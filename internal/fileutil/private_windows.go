//go:build windows

package fileutil

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func checkPrivatePermissions(file *os.File, _ os.FileInfo) error {
	return checkPrivateWindowsACL(file)
}

func checkPrivateDirectory(file *os.File) error {
	return checkPrivateWindowsACL(file)
}

func checkPrivateWindowsACL(file *os.File) error {
	security, err := windows.GetSecurityInfo(windows.Handle(file.Fd()), windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("fileutil: inspect private file ACL: %w", err)
	}
	operator, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("fileutil: inspect private file operator: %w", err)
	}
	owner, _, err := security.Owner()
	if err != nil {
		return fmt.Errorf("fileutil: inspect private file owner: %w", err)
	}
	if !trustedPrivateFileSID(owner, operator.User.Sid) {
		return errors.New("fileutil: private file owner must be the current user, SYSTEM, or Administrators")
	}
	dacl, _, err := security.DACL()
	if err != nil {
		return fmt.Errorf("fileutil: inspect private file DACL: %w", err)
	}
	if dacl == nil {
		return errors.New("fileutil: private file requires a non-null DACL")
	}
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, index, &ace); err != nil {
			return fmt.Errorf("fileutil: inspect private file ACE: %w", err)
		}
		if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 || ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return errors.New("fileutil: private file ACL contains an unsupported ACE")
		}
		// GetAce returns a variable-length ACE whose SID begins at SidStart.
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if ace.Mask != 0 && !trustedPrivateFileSID(sid, operator.User.Sid) {
			return errors.New(
				"fileutil: private file ACL must restrict access to the current user, SYSTEM, and Administrators",
			)
		}
	}
	return nil
}

func trustedPrivateFileSID(sid, operator *windows.SID) bool {
	return sid != nil && (sid.Equals(operator) || sid.IsWellKnown(windows.WinLocalSystemSid) ||
		sid.IsWellKnown(windows.WinBuiltinAdministratorsSid))
}
