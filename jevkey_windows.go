//go:build windows

package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

type dpapiKeyStore struct{ path string }

func newKeyStore(dir string) keyStore { return dpapiKeyStore{filepath.Join(dir, "jev.key")} }

func (dpapiKeyStore) Supported() bool { return true }

func (k dpapiKeyStore) Save(key string) error {
	b := []byte(key)
	if len(b) == 0 {
		return errors.New("APIキーを入力してください")
	}
	in := windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
	var out windows.DataBlob
	if e := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); e != nil {
		return e
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return os.WriteFile(k.path, unsafe.Slice(out.Data, out.Size), 0o600)
}

func (k dpapiKeyStore) Load() (string, error) {
	b, e := os.ReadFile(k.path)
	if errors.Is(e, os.ErrNotExist) || (e == nil && len(b) == 0) {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	in := windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
	var out windows.DataBlob
	if e = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); e != nil {
		log.Printf("jev key: %v", e)
		return "", nil
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return string(unsafe.Slice(out.Data, out.Size)), nil
}

func (k dpapiKeyStore) Clear() error {
	if e := os.Remove(k.path); e != nil && !errors.Is(e, os.ErrNotExist) {
		return e
	}
	return nil
}
