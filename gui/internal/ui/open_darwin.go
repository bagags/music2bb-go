//go:build darwin

package ui

import "os/exec"

func openURL(target string) error { return exec.Command("open", target).Start() }
