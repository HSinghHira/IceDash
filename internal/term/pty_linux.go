//go:build linux

package term

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

// startPTY runs cmd on a new pseudo-terminal and returns its controlling
// side. The usual /dev/ptmx steps (grantpt is a no-op with devpts): unlock
// the secondary side, find its number, open it, and make it the command's
// controlling terminal in a new session.
func startPTY(cmd *exec.Cmd) (*os.File, error) {
	ptm, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	var unlock int32
	if err := ioctl(ptm.Fd(), syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&unlock))); err != nil {
		ptm.Close()
		return nil, fmt.Errorf("unlock the terminal: %w", err)
	}
	var n uint32
	if err := ioctl(ptm.Fd(), syscall.TIOCGPTN, uintptr(unsafe.Pointer(&n))); err != nil {
		ptm.Close()
		return nil, fmt.Errorf("find the terminal: %w", err)
	}
	pts, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		ptm.Close()
		return nil, err
	}
	defer pts.Close()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = pts, pts, pts
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		ptm.Close()
		return nil, err
	}
	return ptm, nil
}

// resize tells the terminal its new size, in characters.
func resize(ptm *os.File, cols, rows uint16) error {
	ws := struct{ rows, cols, x, y uint16 }{rows, cols, 0, 0}
	return ioctl(ptm.Fd(), syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(&ws)))
}

func ioctl(fd, req, arg uintptr) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg); e != 0 {
		return e
	}
	return nil
}
