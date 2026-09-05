package main

import (
	"github.com/Kameleon21/tokenlens/internal/app"
	"os"
)

func main() { os.Exit(app.Run(os.Args[1:])) }
