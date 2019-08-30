// +build !windows

package util

import "os/exec"

func NewCmd(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	return cmd
}
