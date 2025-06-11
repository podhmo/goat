package main

import (
	"fmt"
	"log"

	"github.com/podhmo/goat"
)

type MyOptions struct {
	Name string
	Age  int
}

func NewMyOptions() *MyOptions {
	return &MyOptions{
		Name: goat.Default("Default Name"),
		Age:  goat.Default(30),
	}
}

func CustomInit() *MyOptions {
	return &MyOptions{
		Name: goat.Default("Custom Init Name"),
		Age:  goat.Default(40),
	}
}

type AnotherOptions struct {
	Value string
}

// No initializer for AnotherOptions

func RunApp(options MyOptions) error {
	fmt.Printf("Name: %s, Age: %d\n", options.Name, options.Age)
	return nil
}

func RunAnother(options AnotherOptions) error {
	fmt.Printf("Value: %s\n", options.Value)
	return nil
}

func main() {
	log.Println("This will be replaced by goat.")
}
