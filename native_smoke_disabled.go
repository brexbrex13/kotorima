//go:build !nativesmoke || !windows || server

package main

func runNativeProbe() bool         { return false }
func startNativeVerification(*App) {}
