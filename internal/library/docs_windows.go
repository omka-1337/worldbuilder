package library

import "golang.org/x/sys/windows"

// documentsDir asks Windows for the Documents known folder, which may be
// redirected (to OneDrive, another drive, …).
func documentsDir() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_Documents, 0)
}
