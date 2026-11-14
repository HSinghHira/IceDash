//go:build !linux

package term

import (
	"errors"
	"os"
	"os/exec"
)

// The terminal needs Linux; this lets the package build and test on a PC.
func startPTY(*exec.Cmd) (*os.File, error) { return nil, errors.New("terminals need Linux") }

func resize(*os.File, uint16, uint16) error { return nil }
