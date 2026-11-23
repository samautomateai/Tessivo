//go:build windows

package proctree

import (
	"fmt"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

// prepare has nothing to do on Windows: the Job Object is made in attach.
func prepare(*exec.Cmd) {}

// jobTree is a Job Object that kills every process in it when it is
// terminated or its last handle closes, so the tree also dies when Tessivo
// itself exits or crashes. Children inherit the job.
//
// The child runs for a moment before it is assigned; a descendant it starts in
// that moment is outside the job. MCP servers start children only after their
// first message, which is sent after attach.
type jobTree struct{ job windows.Handle }

func attach(cmd *exec.Cmd) (osTree, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), //nolint:gosec // G103: the documented way to pass the limits struct
		uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("set job limits: %w", err)
	}
	proc, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false,
		uint32(cmd.Process.Pid)) //nolint:gosec // G115: a pid fits in uint32 on Windows
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("open process: %w", err)
	}
	defer func() { _ = windows.CloseHandle(proc) }()
	if err := windows.AssignProcessToJobObject(job, proc); err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("assign to job: %w", err)
	}
	return jobTree{job: job}, nil
}

func (j jobTree) kill() error {
	if err := windows.TerminateJobObject(j.job, 1); err != nil {
		return fmt.Errorf("terminate job: %w", err)
	}
	return nil
}

func (j jobTree) release() error {
	if err := windows.CloseHandle(j.job); err != nil {
		return fmt.Errorf("close job: %w", err)
	}
	return nil
}
