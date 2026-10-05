package main

// keyStore keeps the Jev API key outside settings, snapshots and backups.
type keyStore interface {
	Supported() bool
	Load() (string, error)
	Save(key string) error
	Clear() error
}
