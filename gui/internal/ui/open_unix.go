//go:build !windows && !darwin

package ui

import "os/exec"

func openURL(target string) error { return exec.Command("xdg-open", target).Start() }
