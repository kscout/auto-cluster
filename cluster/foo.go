package main

import (
	"fmt"
)

type Animal interface {
	Speak()
}

type SpaceShipper interface {
	BecomeASpaceShip()
}

// Dog is an animal
type Dog struct {
	Name string
}

func (d Dog) Speak() {
	fmt.Printf("woof woof, my name is %s\n", d.Name)
}

func main() {
	dog := Dog{
		Name: "Bart",
	}
	animals := []Animal{
		dog,
	}

	for _, animal := range animals {
		animal.Speak()
	}
}
