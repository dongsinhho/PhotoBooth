//go:build !windows

package main

import "fmt"

func main() {
	fmt.Println("DS-RX1 virtual printer smoke is supported only on Windows.")
}
