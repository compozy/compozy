package procutil

import (
	"errors"
	"os"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

func startParentOwnedCommand(cmd *exec.Cmd) error {
	ConfigureCommandProcessGroup(cmd)
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := RegisterCommandProcessGroup(cmd); err != nil {
		return discardSuspendedCommand(cmd, err)
	}
	if err := resumeInitialThread(uint32(cmd.Process.Pid)); err != nil {
		return discardSuspendedCommand(cmd, err)
	}
	return nil
}

func discardSuspendedCommand(cmd *exec.Cmd, failure error) error {
	killErr := cmd.Process.Kill()
	if errors.Is(killErr, os.ErrProcessDone) {
		killErr = nil
	}
	releaseErr := releaseParentOwnedCommand(cmd)
	return errors.Join(failure, killErr, releaseErr, waitParentOwnedExit(cmd))
}

func releaseParentOwnedCommand(cmd *exec.Cmd) error {
	if job, ok := windowsJobForCommand(cmd); ok {
		return job.close()
	}
	return nil
}

func resumeInitialThread(pid uint32) (err error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, windows.CloseHandle(snapshot)) }()
	var entry windows.ThreadEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	for next := windows.Thread32First(snapshot, &entry); next == nil; next = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return err
		}
		count, resumeErr := windows.ResumeThread(thread)
		if resumeErr == nil && count != 1 {
			resumeErr = errors.New("connector's initial thread has an unexpected suspend count")
		}
		return errors.Join(resumeErr, windows.CloseHandle(thread))
	}
	return errors.New("connector's initial thread could not be found")
}
