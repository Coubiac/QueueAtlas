//go:build !linux

package auth

import "os"

func openAccountFile(root *os.Root) (*os.File, error) {
	return root.Open(LocalAccountFilename)
}

// Linux is the deployment target with directory synchronization. Other platforms
// retain atomic no-replacement publication and file Sync, not that durability
// guarantee. Windows permissions must be protected separately by operator ACLs.
func syncAccountDirectory(*os.Root) error { return nil }
