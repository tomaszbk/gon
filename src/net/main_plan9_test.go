package net

import "os/exec"

func isUnreachableError(error) bool { return false }

func installTestHooks() {}

func uninstallTestHooks() {}

// forceCloseSockets must be called only from TestMain.
func forceCloseSockets() {}

func enableSocketConnect() {}

func disableSocketConnect(network string) {}

func addCmdInheritedHandle(cmd *exec.Cmd, fd uintptr) {}
