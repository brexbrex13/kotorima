//go:build !windows

package main

import "errors"

type unsupportedKeyStore struct{}

func newKeyStore(string) keyStore { return unsupportedKeyStore{} }

func (unsupportedKeyStore) Supported() bool       { return false }
func (unsupportedKeyStore) Load() (string, error) { return "", nil }
func (unsupportedKeyStore) Save(string) error     { return errors.New("この環境では使えません") }
func (unsupportedKeyStore) Clear() error          { return nil }
