//go:build windows

package selfupdate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func replaceExecutable(stagedPath, targetPath string) (bool, error) {
	powerShell, err := exec.LookPath("powershell.exe")
	if err != nil {
		powerShell, err = exec.LookPath("pwsh.exe")
		if err != nil {
			return false, fmt.Errorf("PowerShell is required to finish a Windows self-update")
		}
	}
	stagingDir := filepath.Dir(stagedPath)
	scriptPath := filepath.Join(stagingDir, "apply-update.ps1")
	backupPath := targetPath + ".old"
	script := fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$parentId = %s
$target = '%s'
$staged = '%s'
$backup = '%s'
$stagingDir = '%s'
try {
  Wait-Process -Id $parentId -ErrorAction SilentlyContinue
  if (Test-Path -LiteralPath $backup) { Remove-Item -LiteralPath $backup -Force }
  Move-Item -LiteralPath $target -Destination $backup -Force
  try {
    Move-Item -LiteralPath $staged -Destination $target -Force
  } catch {
    Move-Item -LiteralPath $backup -Destination $target -Force
    throw
  }
  Remove-Item -LiteralPath $backup -Force -ErrorAction SilentlyContinue
} finally {
  Remove-Item -LiteralPath $stagingDir -Recurse -Force -ErrorAction SilentlyContinue
}
`, strconv.Itoa(os.Getpid()), quotePowerShell(targetPath), quotePowerShell(stagedPath), quotePowerShell(backupPath), quotePowerShell(stagingDir))
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		return false, err
	}
	command := exec.Command(powerShell, "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-File", scriptPath)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return false, err
	}
	_ = command.Process.Release()
	return true, nil
}

func quotePowerShell(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
