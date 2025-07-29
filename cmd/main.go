package main

import (
	"fmt"

	"github.com/duccv/go-clean-template/config"
)

func main() {
	env := config.GetEnv()
	fmt.Println("Application started with the following configuration:", env.LoggerConfig)
}
