package testhelper

import "math/rand"

const letterBytes = "abcdefghijklmnopqrstuvwxyz"

// RandString generates a random string of length n
func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
