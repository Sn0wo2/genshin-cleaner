package genshin

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func Stub(path string) error {
	_ = Unstub(path)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		return err
	}

	sd, err := windows.SecurityDescriptorFromString("D:(D;;0x00010116;;;WD)")
	if err != nil {
		return err
	}

	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}

	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.UNPROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil); err != nil {
		return err
	}

	if attrs, err := windows.GetFileAttributes(windows.StringToUTF16Ptr(path)); err == nil {
		_ = windows.SetFileAttributes(windows.StringToUTF16Ptr(path), attrs|windows.FILE_ATTRIBUTE_READONLY)
	}

	return nil
}

func Unstub(path string) error {
	parent, err := windows.GetNamedSecurityInfo(filepath.Dir(path), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}

	dacl, _, err := parent.DACL()
	if err != nil {
		return err
	}

	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.UNPROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil); err != nil {
		return err
	}

	attrs, err := windows.GetFileAttributes(windows.StringToUTF16Ptr(path))
	if err != nil {
		return err
	}

	return windows.SetFileAttributes(windows.StringToUTF16Ptr(path), attrs&^windows.FILE_ATTRIBUTE_READONLY)
}
