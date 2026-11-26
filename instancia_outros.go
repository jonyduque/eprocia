//go:build !windows

package main

func travarInstancia(string) (func(), bool, error) { return func() {}, true, nil }

func avisarInstanciaAberta() {}
