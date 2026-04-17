//go:build windows

package cli

import (
	"os/exec"
	"syscall"
)

func setDetachAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008, // CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS
	}
}
