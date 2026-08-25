//go:build windows

package pty

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsJob struct {
	mu     sync.Mutex
	handle windows.Handle
}

var ntResumeProcess = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess")

func newWindowsJob() (*windowsJob, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("terminal pty: create process job: %w", err)
	}
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		handle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		return nil, errors.Join(
			fmt.Errorf("terminal pty: enable process-tree teardown: %w", err),
			closeWindowsJobHandle(handle),
		)
	}
	return &windowsJob{handle: handle}, nil
}

func (j *windowsJob) assign(process windows.Handle) error {
	if j == nil || j.handle == 0 {
		return errors.New("terminal pty: process job is closed")
	}
	if err := windows.AssignProcessToJobObject(j.handle, process); err != nil {
		return err
	}
	return nil
}

func resumeWindowsProcess(process windows.Handle) error {
	status, _, lastErr := ntResumeProcess.Call(uintptr(process))
	result := windows.NTStatus(status)
	if result != windows.STATUS_SUCCESS {
		return errors.Join(result, lastErr)
	}
	return nil
}

func (j *windowsJob) terminate(exitCode uint32) error {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.handle == 0 {
		return nil
	}
	return windows.TerminateJobObject(j.handle, exitCode)
}

func (j *windowsJob) close() error {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.handle == 0 {
		return nil
	}
	err := closeWindowsJobHandle(j.handle)
	if err == nil {
		j.handle = 0
	}
	return err
}

func closeWindowsJobHandle(handle windows.Handle) error {
	if handle == 0 {
		return nil
	}
	if err := windows.CloseHandle(handle); err != nil {
		return fmt.Errorf("terminal pty: close process job: %w", err)
	}
	return nil
}
