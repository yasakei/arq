//go:build !linux

package runtime

func killMatching(string) error { return nil }
