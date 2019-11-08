package main

import "fmt"

func main() {
	i := 10
	for i > 0 {
		fmt.Println(i)
		if i % 2 == 0 {
			fmt.Println("hello")
		}
	}
	fmt.Println("done")
}
